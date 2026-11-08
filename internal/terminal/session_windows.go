//go:build windows

package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/bombless/terminal-libghosty/internal/ssh"
)

var (
	sessionDLL          = newSessionDLL()
	procCreate          = sessionDLL.NewProc("tlg_ssh_terminal_create_argv")
	procWrite           = sessionDLL.NewProc("tlg_ssh_terminal_write")
	procResize          = sessionDLL.NewProc("tlg_ssh_terminal_resize")
	procPoll            = sessionDLL.NewProc("tlg_ssh_terminal_poll_exit")
	procReadText        = sessionDLL.NewProc("tlg_ssh_terminal_read_text")
	procSnapshot        = sessionDLL.NewProc("tlg_ssh_terminal_snapshot")
	procClose           = sessionDLL.NewProc("tlg_ssh_terminal_close")
	procSetDllDirectory = syscall.NewLazyDLL("kernel32.dll").NewProc("SetDllDirectoryW")
	dllLoadOnce         sync.Once
	dllLoadErr          error
)

func newSessionDLL() *syscall.LazyDLL {
	if dir := os.Getenv("TERMINAL_LIBGHOSTY_NATIVE_DIR"); dir != "" {
		return syscall.NewLazyDLL(filepath.Join(dir, "terminal_libghosty_session.dll"))
	}
	return syscall.NewLazyDLL("terminal_libghosty_session.dll")
}

// Session methods must not race Close. The native layer serializes VT snapshots/resizes and pipe writes.
type Session struct{ handle uintptr }

// Cell and Cursor mirror the stable project-owned C ABI in native/ghostty_host.h.
type RGB struct{ R, G, B uint8 }
type Cell struct {
	TextOffset, TextLength, StyleFlags uint32
	Foreground, Background             RGB
	Selected                           uint8
	Reserved                           [3]byte
}
type Cursor struct {
	X, Y                           uint16
	Visible, Blinking, VisualStyle uint8
	Reserved                       [3]byte
}

func StartSSH(options ssh.Options, cols, rows uint16) (*Session, error) {
	args, err := ssh.BuildArgs(options)
	if err != nil {
		return nil, err
	}
	return Start("ssh.exe", args, cols, rows)
}

func findNativeDir() string {
	if dir := os.Getenv("TERMINAL_LIBGHOSTY_NATIVE_DIR"); dir != "" {
		return dir
	}
	candidates := []string{
		"build/native-local",
		"../build/native-local",
		"../../build/native-local",
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			exeDir,
			filepath.Join(exeDir, "..", "build", "native-local"),
			filepath.Join(exeDir, "native-local"),
		)
	}
	for _, c := range candidates {
		abs, err := filepath.Abs(c)
		if err == nil {
			if _, err := os.Stat(filepath.Join(abs, "terminal_libghosty_session.dll")); err == nil {
				return abs
			}
		}
	}
	return ""
}

func loadNativeDLL() error {
	dllLoadOnce.Do(func() {
		dir := findNativeDir()
		if dir != "" {
			p, err := syscall.UTF16PtrFromString(dir)
			if err != nil {
				dllLoadErr = err
				return
			}
			r1, _, callErr := procSetDllDirectory.Call(uintptr(unsafe.Pointer(p)))
			runtime.KeepAlive(p)
			if r1 == 0 {
				dllLoadErr = fmt.Errorf("SetDllDirectoryW: %w", callErr)
				return
			}
		}
		dllLoadErr = sessionDLL.Load()
		if dir != "" {
			// The session DLL and its Ghostty dependency are now loaded; restore
			// the default process search path instead of leaving a global override.
			procSetDllDirectory.Call(0)
		}
	})
	return dllLoadErr
}

func Start(executable string, args []string, cols, rows uint16) (*Session, error) {
	if err := loadNativeDLL(); err != nil {
		return nil, fmt.Errorf("load native terminal bridge: %w", err)
	}
	if !filepath.IsAbs(executable) && !strings.ContainsAny(executable, `/\\`) {
		resolved, err := exec.LookPath(executable)
		if err != nil {
			return nil, fmt.Errorf("resolve executable %q: %w", executable, err)
		}
		executable = resolved
	}
	exe, err := syscall.UTF16PtrFromString(executable)
	if err != nil {
		return nil, err
	}
	wide := make([]*uint16, len(args))
	for i, arg := range args {
		wide[i], err = syscall.UTF16PtrFromString(arg)
		if err != nil {
			return nil, fmt.Errorf("argument %d: %w", i, err)
		}
	}
	var argv uintptr
	if len(wide) > 0 {
		argv = uintptr(unsafe.Pointer(&wide[0]))
	}
	var handle uintptr
	var winErr uint32
	r1, _, _ := procCreate.Call(
		uintptr(unsafe.Pointer(&handle)), uintptr(unsafe.Pointer(exe)), argv,
		uintptr(len(wide)), uintptr(cols), uintptr(rows), 9, 18,
		uintptr(unsafe.Pointer(&winErr)))
	runtime.KeepAlive(exe)
	runtime.KeepAlive(wide)
	runtime.KeepAlive(args)
	if int32(r1) != 0 {
		return nil, fmt.Errorf("create SSH ConPTY session failed (status=%d, win32=%d)", int32(r1), winErr)
	}
	return &Session{handle: handle}, nil
}

func (s *Session) Write(data []byte) error {
	if s == nil || s.handle == 0 {
		return fmt.Errorf("terminal session is closed")
	}
	var ptr uintptr
	if len(data) > 0 {
		ptr = uintptr(unsafe.Pointer(&data[0]))
	}
	var winErr uint32
	r1, _, _ := procWrite.Call(s.handle, ptr, uintptr(len(data)), uintptr(unsafe.Pointer(&winErr)))
	runtime.KeepAlive(data)
	if int32(r1) != 0 {
		return fmt.Errorf("terminal write failed (status=%d, win32=%d)", int32(r1), winErr)
	}
	return nil
}

func (s *Session) Resize(cols, rows uint16) error {
	if s == nil || s.handle == 0 {
		return fmt.Errorf("terminal session is closed")
	}
	var winErr uint32
	r1, _, _ := procResize.Call(s.handle, uintptr(cols), uintptr(rows), 9, 18, uintptr(unsafe.Pointer(&winErr)))
	if int32(r1) != 0 {
		return fmt.Errorf("terminal resize failed (status=%d, win32=%d)", int32(r1), winErr)
	}
	return nil
}

func (s *Session) PollExit() (bool, uint32, error) {
	if s == nil || s.handle == 0 {
		return false, 0, fmt.Errorf("terminal session is closed")
	}
	var code, winErr uint32
	r1, _, _ := procPoll.Call(s.handle, uintptr(unsafe.Pointer(&code)), uintptr(unsafe.Pointer(&winErr)))
	if int32(r1) < 0 {
		return false, 0, fmt.Errorf("terminal poll failed (status=%d, win32=%d)", int32(r1), winErr)
	}
	return r1 == 1, code, nil
}

// Snapshot returns a caller-owned copy of Ghostty's rendered cell grid.
func (s *Session) Snapshot() (uint16, uint16, []Cell, []byte, Cursor, error) {
	if s == nil || s.handle == 0 {
		return 0, 0, nil, nil, Cursor{}, fmt.Errorf("terminal session is closed")
	}
	var cols, rows uint16
	var cellCount, textBytes uintptr
	var cursor Cursor
	r1, _, _ := procSnapshot.Call(s.handle, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&cols)), uintptr(unsafe.Pointer(&rows)),
		uintptr(unsafe.Pointer(&cellCount)), uintptr(unsafe.Pointer(&textBytes)),
		uintptr(unsafe.Pointer(&cursor)))
	if int32(r1) != -4 && int32(r1) != 0 {
		return 0, 0, nil, nil, Cursor{}, fmt.Errorf("terminal snapshot sizing failed: %d", int32(r1))
	}
	cells := make([]Cell, int(cellCount))
	text := make([]byte, int(textBytes))
	var cellPtr, textPtr uintptr
	if len(cells) != 0 {
		cellPtr = uintptr(unsafe.Pointer(&cells[0]))
	}
	if len(text) != 0 {
		textPtr = uintptr(unsafe.Pointer(&text[0]))
	}
	r1, _, _ = procSnapshot.Call(s.handle, cellPtr, cellCount, textPtr, textBytes,
		uintptr(unsafe.Pointer(&cols)), uintptr(unsafe.Pointer(&rows)),
		uintptr(unsafe.Pointer(&cellCount)), uintptr(unsafe.Pointer(&textBytes)),
		uintptr(unsafe.Pointer(&cursor)))
	runtime.KeepAlive(cells)
	runtime.KeepAlive(text)
	if int32(r1) != 0 {
		return 0, 0, nil, nil, Cursor{}, fmt.Errorf("terminal snapshot failed: %d", int32(r1))
	}
	return cols, rows, cells, text, cursor, nil
}

func (s *Session) SnapshotText() (string, error) {
	if s == nil || s.handle == 0 {
		return "", fmt.Errorf("terminal session is closed")
	}
	var required uintptr
	r1, _, _ := procReadText.Call(s.handle, 0, 0, uintptr(unsafe.Pointer(&required)))
	if int32(r1) != -4 && int32(r1) != 0 {
		return "", fmt.Errorf("terminal snapshot sizing failed: %d", int32(r1))
	}
	buf := make([]byte, int(required))
	if required == 0 {
		return "", nil
	}
	r1, _, _ = procReadText.Call(s.handle, uintptr(unsafe.Pointer(&buf[0])), required, uintptr(unsafe.Pointer(&required)))
	runtime.KeepAlive(buf)
	if int32(r1) != 0 {
		return "", fmt.Errorf("terminal snapshot failed: %d", int32(r1))
	}
	return string(buf[:int(required)]), nil
}

func (s *Session) Close() {
	if s == nil || s.handle == 0 {
		return
	}
	procClose.Call(s.handle)
	s.handle = 0
}
