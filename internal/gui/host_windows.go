//go:build windows

package gui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/bombless/terminal-libghosty/internal/ssh"
	"github.com/bombless/terminal-libghosty/internal/terminal"
)

var (
	hostKernel32                 = syscall.NewLazyDLL("kernel32.dll")
	hostGetWindowThreadProcessID = u32.NewProc("GetWindowThreadProcessId")
	hostIsWindow                 = u32.NewProc("IsWindow")
	hostOpenProcess              = hostKernel32.NewProc("OpenProcess")
	hostWaitForSingleObject      = hostKernel32.NewProc("WaitForSingleObject")
	hostCloseHandle              = hostKernel32.NewProc("CloseHandle")
)

// RunHost is the child-process server entry point. It embeds the terminal surface
// into the parent window and owns the SSH/ConPTY session independently of the GUI.
func RunHost(parentText, target, identity string, port int) error {
	base := 10
	if strings.HasPrefix(strings.ToLower(parentText), "0x") {
		base = 16
		parentText = parentText[2:]
	}
	value, err := strconv.ParseUint(parentText, base, 64)
	if err != nil || value == 0 {
		return fmt.Errorf("invalid parent window handle %q", parentText)
	}
	parent := uintptr(value)
	if alive, _, _ := hostIsWindow.Call(parent); alive == 0 {
		return fmt.Errorf("parent window no longer exists")
	}

	// Tie the child process lifetime to the parent process, not merely its HWND.
	var parentPID uint32
	hostGetWindowThreadProcessID.Call(parent, uintptr(unsafe.Pointer(&parentPID)))
	if parentPID != 0 {
		handle, _, _ := hostOpenProcess.Call(0x00100000, 0, uintptr(parentPID)) // SYNCHRONIZE
		if handle != 0 {
			go func() {
				defer hostCloseHandle.Call(handle)
				hostWaitForSingleObject.Call(handle, 0xFFFFFFFF)
				os.Exit(0)
			}()
		}
	}

	view, err := NewTerminalView(parent, func(code uint32) {})
	if err != nil {
		return fmt.Errorf("create terminal view: %w", err)
	}
	defer view.Destroy()

	var bounds rect
	procGetClientRect.Call(parent, uintptr(unsafe.Pointer(&bounds)))
	width := int(bounds.Right - bounds.Left)
	height := int(bounds.Bottom - bounds.Top)
	if width < 1 {
		width = 80 * 9
	}
	if height < 1 {
		height = 24 * 18
	}
	view.SetBounds(0, 0, width, height)
	view.Show(true)
	view.Focus()

	if target != "" {
		cols, rows := uint16(width/9), uint16(height/18)
		if cols < 1 {
			cols = 80
		}
		if rows < 1 {
			rows = 24
		}
		session, startErr := terminal.StartSSH(ssh.Options{
			Target: target, IdentityFile: identity, Port: port,
		}, cols, rows)
		if startErr != nil {
			return fmt.Errorf("start SSH session: %w", startErr)
		}
		defer session.Close()
		view.AttachSession(session)
	}

	// Resize the child surface as the parent resizes its client area.
	go func() {
		lastW, lastH := width, height
		for {
			time.Sleep(30 * time.Millisecond)
			if alive, _, _ := hostIsWindow.Call(parent); alive == 0 {
				os.Exit(0)
			}
			var current rect
			procGetClientRect.Call(parent, uintptr(unsafe.Pointer(&current)))
			w, h := int(current.Right-current.Left), int(current.Bottom-current.Top)
			if w > 0 && h > 0 && (w != lastW || h != lastH) {
				lastW, lastH = w, h
				view.SetBounds(0, 0, w, h)
			}
		}
	}()

	var message msg
	for {
		result, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
	return nil
}
