//go:build windows

// terminal-host provides Direct2D SSH terminal child embedding or standalone window.
package main

import (
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/bombless/terminal-libghosty/internal/gui"
	"github.com/bombless/terminal-libghosty/internal/ssh"
	"github.com/bombless/terminal-libghosty/internal/terminal"
)

var (
	u32                      = syscall.NewLazyDLL("user32.dll")
	k32                      = syscall.NewLazyDLL("kernel32.dll")
	procGetMessageW          = u32.NewProc("GetMessageW")
	procTranslateMessage     = u32.NewProc("TranslateMessage")
	procDispatchMessageW     = u32.NewProc("DispatchMessageW")
	procPostQuitMessage      = u32.NewProc("PostQuitMessage")
	procGetClientRect        = u32.NewProc("GetClientRect")
	procMoveWindow           = u32.NewProc("MoveWindow")
	procIsWindow             = u32.NewProc("IsWindow")
	procGetWindowThreadProcessId = u32.NewProc("GetWindowThreadProcessId")
	procOpenProcess          = k32.NewProc("OpenProcess")
	procWaitForSingleObject  = k32.NewProc("WaitForSingleObject")
	procCloseHandle          = k32.NewProc("CloseHandle")
)

type rect struct {
	Left, Top, Right, Bottom int32
}

type point struct {
	X, Y int32
}

type msg struct {
	Hwnd           uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             point
}

type syncFileWriter struct {
	mu sync.Mutex
	f  *os.File
}

func (s *syncFileWriter) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err = s.f.Write(p)
	if err == nil {
		_ = s.f.Sync()
	}
	return
}

func main() {
	var target, key, parentText, logDir string
	var port int
	flag.StringVar(&target, "ssh", "", "SSH target user@host[:port]")
	flag.StringVar(&key, "identity", "", "private key path")
	flag.IntVar(&port, "port", 0, "SSH port override")
	flag.StringVar(&parentText, "parent", "", "parent Win32 HWND")
	flag.StringVar(&logDir, "log-dir", "", "directory for host logs")
	flag.Parse()

	logOutputs := []io.Writer{os.Stderr}
	if logDir != "" {
		_ = os.MkdirAll(logDir, 0755)
		logPath := filepath.Join(logDir, "host.log")
		if lf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
			defer lf.Close()
			logOutputs = append(logOutputs, &syncFileWriter{f: lf})
		}
	}
	log.SetOutput(io.MultiWriter(logOutputs...))
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	log.Print("terminal-host starting (Direct2D)")

	// If no parent HWND is given, launch standalone GUI
	if parentText == "" {
		if err := gui.RunWithOptions(target, key, port); err != nil {
			log.Fatalf("GUI error: %v", err)
		}
		return
	}

	base := 10
	if strings.HasPrefix(strings.ToLower(parentText), "0x") {
		base = 16
		parentText = parentText[2:]
	}
	p, err := strconv.ParseUint(parentText, base, 64)
	if err != nil {
		log.Fatalf("invalid parent HWND: %v", err)
	}
	parent := uintptr(p)

	// Monitor parent process so terminal-host immediately terminates if parent process dies.
	var parentPid uint32
	procGetWindowThreadProcessId.Call(parent, uintptr(unsafe.Pointer(&parentPid)))
	if parentPid != 0 {
		// SYNCHRONIZE = 0x00100000
		hProc, _, _ := procOpenProcess.Call(0x00100000, 0, uintptr(parentPid))
		if hProc != 0 {
			go func() {
				defer procCloseHandle.Call(hProc)
				procWaitForSingleObject.Call(hProc, 0xFFFFFFFF)
				log.Print("parent process exited, terminating terminal-host")
				time.Sleep(50 * time.Millisecond)
				os.Exit(0)
			}()
		}
	}

	termView, err := gui.NewTerminalView(parent, func(code uint32) {
		log.Printf("session exited with code %d", code)
	})
	if err != nil {
		log.Fatalf("NewTerminalView failed: %v", err)
	}
	defer termView.Destroy()

	var pr rect
	procGetClientRect.Call(parent, uintptr(unsafe.Pointer(&pr)))
	w := int(pr.Right - pr.Left)
	h := int(pr.Bottom - pr.Top)
	if w <= 0 {
		w = 100
	}
	if h <= 0 {
		h = 100
	}
	termView.SetBounds(0, 0, w, h)
	termView.Show(true)
	termView.Focus()

	if target != "" {
		cols := uint16(w / 9)
		rows := uint16(h / 18)
		if cols < 1 {
			cols = 80
		}
		if rows < 1 {
			rows = 24
		}
		sess, err := terminal.StartSSH(ssh.Options{Target: target, IdentityFile: key, Port: port}, cols, rows)
		if err != nil {
			log.Fatalf("StartSSH failed: %v", err)
		}
		defer sess.Close()
		termView.AttachSession(sess)
	}

	// Message loop and resize polling
	go func() {
		for {
			time.Sleep(30 * time.Millisecond)
			if r, _, _ := procIsWindow.Call(parent); r == 0 {
				procPostQuitMessage.Call(0)
				return
			}
			var curPr rect
			procGetClientRect.Call(parent, uintptr(unsafe.Pointer(&curPr)))
			cw := int(curPr.Right - curPr.Left)
			ch := int(curPr.Bottom - curPr.Top)
			if cw != w || ch != h {
				w, h = cw, ch
				termView.SetBounds(0, 0, w, h)
			}
		}
	}()

	var m msg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	log.Print("terminal-host exited")
}
