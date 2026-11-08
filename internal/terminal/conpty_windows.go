//go:build windows

package terminal

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

const (
	procThreadAttributePseudoConsole  = 0x00020016
	extendedStartupInfoPresent        = 0x00080000
	createUnicodeEnvironment          = 0x00000400
	startfUseStdHandles               = 0x00000100
	handleFlagInherit                 = 0x00000001
	jobObjectExtendedLimitInformation = 9
	jobObjectLimitKillOnJobClose      = 0x00002000
	waitObject0                       = 0
	waitTimeout                       = 258
)

var (
	kernel32ConPTY                        = syscall.NewLazyDLL("kernel32.dll")
	procCreatePipe                        = kernel32ConPTY.NewProc("CreatePipe")
	procSetHandleInformation              = kernel32ConPTY.NewProc("SetHandleInformation")
	procCreatePseudoConsole               = kernel32ConPTY.NewProc("CreatePseudoConsole")
	procClosePseudoConsole                = kernel32ConPTY.NewProc("ClosePseudoConsole")
	procResizePseudoConsole               = kernel32ConPTY.NewProc("ResizePseudoConsole")
	procInitializeProcThreadAttributeList = kernel32ConPTY.NewProc("InitializeProcThreadAttributeList")
	procUpdateProcThreadAttribute         = kernel32ConPTY.NewProc("UpdateProcThreadAttribute")
	procDeleteProcThreadAttributeList     = kernel32ConPTY.NewProc("DeleteProcThreadAttributeList")
	procCreateProcessW                    = kernel32ConPTY.NewProc("CreateProcessW")
	procCreateJobObjectW                  = kernel32ConPTY.NewProc("CreateJobObjectW")
	procSetInformationJobObject           = kernel32ConPTY.NewProc("SetInformationJobObject")
	procAssignProcessToJobObject          = kernel32ConPTY.NewProc("AssignProcessToJobObject")
	procTerminateJobObject                = kernel32ConPTY.NewProc("TerminateJobObject")
	procTerminateProcess                  = kernel32ConPTY.NewProc("TerminateProcess")
	procReadFile                          = kernel32ConPTY.NewProc("ReadFile")
	procWriteFile                         = kernel32ConPTY.NewProc("WriteFile")
	procWaitForSingleObject               = kernel32ConPTY.NewProc("WaitForSingleObject")
	procGetExitCodeProcess                = kernel32ConPTY.NewProc("GetExitCodeProcess")
	procCloseHandle                       = kernel32ConPTY.NewProc("CloseHandle")
)

type securityAttributes struct {
	Length             uint32
	SecurityDescriptor uintptr
	InheritHandle      int32
}

type startupInfoEx struct {
	StartupInfo   syscall.StartupInfo
	AttributeList uintptr
}

type jobBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	_                       uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	_                       uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobExtendedLimitInformation struct {
	BasicLimitInformation jobBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

type conPTY struct {
	mu         sync.Mutex
	writeMu    sync.Mutex
	closed     bool
	input      syscall.Handle
	output     syscall.Handle
	process    syscall.Handle
	job        syscall.Handle
	pseudo     syscall.Handle
	exitKnown  bool
	exitCode   uint32
	onOutput   func([]byte)
	onExit     func(uint32)
	readerDone chan struct{}
	closeDone  chan struct{}
}

func conptyWin32Error(op string, code syscall.Errno) error {
	return fmt.Errorf("%s: Win32 error %d: %w", op, uint32(code), code)
}

func lastWin32Error(op string) error {
	last := syscall.GetLastError()
	if code, ok := last.(syscall.Errno); ok {
		return conptyWin32Error(op, code)
	}
	return fmt.Errorf("%s: %w", op, last)
}

func closeConPTYHandle(h *syscall.Handle) {
	if *h != 0 && *h != syscall.InvalidHandle {
		procCloseHandle.Call(uintptr(*h))
		*h = 0
	}
}

func quoteWindowsArg(arg string) string {
	needsQuotes := arg == ""
	for _, r := range arg {
		if r == ' ' || r == '\t' || r == '"' {
			needsQuotes = true
			break
		}
	}
	if !needsQuotes {
		return arg
	}
	var b strings.Builder
	b.Grow(len(arg) + 2)
	b.WriteByte('"')
	slashes := 0
	for _, r := range arg {
		if r == '\\' {
			slashes++
			continue
		}
		if r == '"' {
			b.WriteString(strings.Repeat("\\", slashes*2+1))
			b.WriteRune(r)
			slashes = 0
			continue
		}
		b.WriteString(strings.Repeat("\\", slashes))
		slashes = 0
		b.WriteRune(r)
	}
	b.WriteString(strings.Repeat("\\", slashes*2))
	b.WriteByte('"')
	return b.String()
}

func buildWindowsCommandLine(executable string, args []string) (string, error) {
	if executable == "" {
		return "", fmt.Errorf("empty executable")
	}
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, quoteWindowsArg(executable))
	for i, arg := range args {
		if strings.IndexByte(arg, 0) >= 0 {
			return "", fmt.Errorf("argument %d contains NUL", i)
		}
		parts = append(parts, quoteWindowsArg(arg))
	}
	return strings.Join(parts, " "), nil
}

func newConPTY(executable string, args []string, cols, rows uint16, onOutput func([]byte), onExit func(uint32)) (*conPTY, error) {
	if executable == "" || cols == 0 || rows == 0 {
		return nil, fmt.Errorf("invalid ConPTY executable or dimensions")
	}
	if !strings.ContainsAny(executable, "/\\") {
		resolved, err := exec.LookPath(executable)
		if err != nil {
			return nil, fmt.Errorf("resolve executable %q: %w", executable, err)
		}
		executable = resolved
	}
	commandLine, err := buildWindowsCommandLine(executable, args)
	if err != nil {
		return nil, err
	}
	cmd16, err := syscall.UTF16FromString(commandLine)
	if err != nil {
		return nil, err
	}

	s := &conPTY{onOutput: onOutput, onExit: onExit, readerDone: make(chan struct{}), closeDone: make(chan struct{})}
	var childInput, childOutput syscall.Handle
	var attrs uintptr
	var attrListInitialized bool
	var attrWords []uintptr
	var pi syscall.ProcessInformation
	fail := func(cause error) (*conPTY, error) {
		if attrListInitialized {
			procDeleteProcThreadAttributeList.Call(attrs)
		}
		closeConPTYHandle(&childInput)
		closeConPTYHandle(&childOutput)
		if pi.Process != 0 {
			procTerminateProcess.Call(uintptr(pi.Process), 1)
			procWaitForSingleObject.Call(uintptr(pi.Process), 5000)
		}
		if s.pseudo != 0 {
			procClosePseudoConsole.Call(uintptr(s.pseudo))
		}
		closeConPTYHandle(&pi.Thread)
		closeConPTYHandle(&pi.Process)
		closeConPTYHandle(&s.input)
		closeConPTYHandle(&s.output)
		closeConPTYHandle(&s.job)
		return nil, cause
	}

	security := securityAttributes{Length: uint32(unsafe.Sizeof(securityAttributes{})), InheritHandle: 1}
	ok, _, _ := procCreatePipe.Call(uintptr(unsafe.Pointer(&childInput)), uintptr(unsafe.Pointer(&s.input)), uintptr(unsafe.Pointer(&security)), 0)
	if ok == 0 {
		return fail(lastWin32Error("CreatePipe(input)"))
	}
	ok, _, _ = procCreatePipe.Call(uintptr(unsafe.Pointer(&s.output)), uintptr(unsafe.Pointer(&childOutput)), uintptr(unsafe.Pointer(&security)), 0)
	if ok == 0 {
		return fail(lastWin32Error("CreatePipe(output)"))
	}
	ok, _, _ = procSetHandleInformation.Call(uintptr(s.input), handleFlagInherit, 0)
	if ok == 0 {
		return fail(lastWin32Error("SetHandleInformation(input)"))
	}
	ok, _, _ = procSetHandleInformation.Call(uintptr(s.output), handleFlagInherit, 0)
	if ok == 0 {
		return fail(lastWin32Error("SetHandleInformation(output)"))
	}

	coord := uintptr(uint32(uint16(cols)) | uint32(uint16(rows))<<16)
	hr, _, _ := procCreatePseudoConsole.Call(coord, uintptr(childInput), uintptr(childOutput), 0, uintptr(unsafe.Pointer(&s.pseudo)))
	closeConPTYHandle(&childInput)
	closeConPTYHandle(&childOutput)
	if int32(hr) < 0 {
		return fail(fmt.Errorf("CreatePseudoConsole: HRESULT 0x%08x", uint32(hr)))
	}

	var attrSize uintptr
	procInitializeProcThreadAttributeList.Call(0, 1, 0, uintptr(unsafe.Pointer(&attrSize)))
	if attrSize == 0 {
		return fail(lastWin32Error("InitializeProcThreadAttributeList(size)"))
	}
	attrWords = make([]uintptr, (attrSize+unsafe.Sizeof(uintptr(0))-1)/unsafe.Sizeof(uintptr(0)))
	attrs = uintptr(unsafe.Pointer(&attrWords[0]))
	ok, _, _ = procInitializeProcThreadAttributeList.Call(attrs, 1, 0, uintptr(unsafe.Pointer(&attrSize)))
	if ok == 0 {
		return fail(lastWin32Error("InitializeProcThreadAttributeList"))
	}
	attrListInitialized = true
	ok, _, _ = procUpdateProcThreadAttribute.Call(attrs, 0, procThreadAttributePseudoConsole, uintptr(s.pseudo), unsafe.Sizeof(s.pseudo), 0, 0)
	if ok == 0 {
		return fail(lastWin32Error("UpdateProcThreadAttribute(PSEUDOCONSOLE)"))
	}

	si := startupInfoEx{AttributeList: attrs}
	si.StartupInfo.Cb = uint32(unsafe.Sizeof(si))
	si.StartupInfo.Flags = startfUseStdHandles
	var app *uint16
	if filepathIsAbsWindows(executable) {
		app, err = syscall.UTF16PtrFromString(executable)
		if err != nil {
			return fail(err)
		}
	}
	ok, _, _ = procCreateProcessW.Call(
		uintptr(unsafe.Pointer(app)), uintptr(unsafe.Pointer(&cmd16[0])),
		0, 0, 0, extendedStartupInfoPresent|createUnicodeEnvironment,
		0, 0, uintptr(unsafe.Pointer(&si)), uintptr(unsafe.Pointer(&pi)),
	)
	runtime.KeepAlive(cmd16)
	runtime.KeepAlive(app)
	runtime.KeepAlive(attrWords)
	if ok == 0 {
		return fail(lastWin32Error("CreateProcessW"))
	}
	closeConPTYHandle(&pi.Thread)
	s.process = pi.Process
	procDeleteProcThreadAttributeList.Call(attrs)
	attrListInitialized = false
	attrs = 0

	// Match the original C module: Job Objects improve cleanup when available,
	// but unsupported/nested-job environments must not prevent terminal startup.
	job, _, _ := procCreateJobObjectW.Call(0, 0)
	if job != 0 {
		candidate := syscall.Handle(job)
		limits := jobExtendedLimitInformation{}
		limits.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
		setOK, _, _ := procSetInformationJobObject.Call(uintptr(candidate), jobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), unsafe.Sizeof(limits))
		if setOK != 0 {
			assignOK, _, _ := procAssignProcessToJobObject.Call(uintptr(candidate), uintptr(s.process))
			if assignOK != 0 {
				s.job = candidate
			} else {
				closeConPTYHandle(&candidate)
			}
		} else {
			closeConPTYHandle(&candidate)
		}
	}

	go s.readOutput()
	return s, nil
}

func filepathIsAbsWindows(path string) bool {
	return len(path) >= 3 && ((path[1] == ':' && (path[2] == '\\' || path[2] == '/')) || strings.HasPrefix(path, "\\\\"))
}

func (s *conPTY) readOutput() {
	defer close(s.readerDone)
	buf := make([]byte, 16*1024)
	for {
		var read uint32
		ok, _, _ := procReadFile.Call(uintptr(s.output), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&read)), 0)
		if ok == 0 || read == 0 {
			break
		}
		if s.onOutput != nil {
			chunk := append([]byte(nil), buf[:read]...)
			s.onOutput(chunk)
		}
	}
	// The C callback contract fires only after the output pipe reaches EOF.
	procWaitForSingleObject.Call(uintptr(s.process), ^uintptr(0))
	var code uint32
	ok, _, _ := procGetExitCodeProcess.Call(uintptr(s.process), uintptr(unsafe.Pointer(&code)))
	if ok != 0 {
		s.mu.Lock()
		s.exitCode, s.exitKnown = code, true
		s.mu.Unlock()
		if s.onExit != nil {
			s.onExit(code)
		}
	}
}

func (s *conPTY) Write(data []byte) error {
	if s == nil {
		return fmt.Errorf("nil ConPTY")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("ConPTY is closed")
	}
	input := s.input
	s.mu.Unlock()
	offset := 0
	for offset < len(data) {
		chunk := len(data) - offset
		if chunk > 0x7fffffff {
			chunk = 0x7fffffff
		}
		var written uint32
		ok, _, _ := procWriteFile.Call(uintptr(input), uintptr(unsafe.Pointer(&data[offset])), uintptr(chunk), uintptr(unsafe.Pointer(&written)), 0)
		if ok == 0 || written == 0 {
			return lastWin32Error("WriteFile(ConPTY input)")
		}
		offset += int(written)
	}
	runtime.KeepAlive(data)
	return nil
}

func (s *conPTY) Resize(cols, rows uint16) error {
	if s == nil || cols == 0 || rows == 0 {
		return fmt.Errorf("invalid ConPTY dimensions")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("ConPTY is closed")
	}
	coord := uintptr(uint32(uint16(cols)) | uint32(uint16(rows))<<16)
	hr, _, _ := procResizePseudoConsole.Call(uintptr(s.pseudo), coord)
	if int32(hr) < 0 {
		return fmt.Errorf("ResizePseudoConsole: HRESULT 0x%08x", uint32(hr))
	}
	return nil
}

func (s *conPTY) PollExit() (bool, uint32, error) {
	if s == nil {
		return false, 0, fmt.Errorf("nil ConPTY")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false, 0, fmt.Errorf("ConPTY is closed")
	}
	if s.exitKnown {
		return true, s.exitCode, nil
	}
	wait, _, _ := procWaitForSingleObject.Call(uintptr(s.process), 0)
	if wait == waitTimeout {
		return false, 0, nil
	}
	if wait != waitObject0 {
		return false, 0, lastWin32Error("WaitForSingleObject(ConPTY process)")
	}
	var code uint32
	ok, _, _ := procGetExitCodeProcess.Call(uintptr(s.process), uintptr(unsafe.Pointer(&code)))
	if ok == 0 {
		return false, 0, lastWin32Error("GetExitCodeProcess")
	}
	s.exitCode, s.exitKnown = code, true
	return true, code, nil
}

func (s *conPTY) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.closed {
		done := s.closeDone
		s.mu.Unlock()
		<-done
		return
	}
	s.closed = true
	process, job, pseudo := s.process, s.job, s.pseudo
	s.mu.Unlock()

	if process != 0 {
		wait, _, _ := procWaitForSingleObject.Call(uintptr(process), 0)
		if wait == waitTimeout {
			if job != 0 {
				procTerminateJobObject.Call(uintptr(job), 1)
			} else {
				procTerminateProcess.Call(uintptr(process), 1)
			}
		}
	}
	if pseudo != 0 {
		procClosePseudoConsole.Call(uintptr(pseudo))
	}
	<-s.readerDone

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.mu.Lock()
	closeConPTYHandle(&s.input)
	closeConPTYHandle(&s.output)
	closeConPTYHandle(&s.process)
	closeConPTYHandle(&s.job)
	s.pseudo = 0
	s.mu.Unlock()
	close(s.closeDone)
}
