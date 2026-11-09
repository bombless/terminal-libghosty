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
// into the parent window and owns the SSH or local shell session independently of
// the GUI.
func RunHost(parentText, target, identity string, port int, shell string) error {
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
	if r, ok := containerScreenRect(parent); ok {
		view.SetScreenBounds(int(r.Left), int(r.Top), int(r.Right-r.Left), int(r.Bottom-r.Top))
	} else {
		view.SetScreenBounds(0, 0, width, height)
	}
	view.Show(true)
	view.Focus()
	owner, _, _ := procGetAncestor.Call(parent, gaRoot)
	go ensureForeground(view.HWND(), owner)

	cols, rows := uint16(width/9), uint16(height/18)
	if cols < 1 {
		cols = 80
	}
	if rows < 1 {
		rows = 24
	}
	var session *terminal.Session
	var startErr error
	switch {
	case shell == "" && target != "":
		session, startErr = terminal.StartSSH(ssh.Options{
			Target: target, IdentityFile: identity, Port: port,
		}, cols, rows)
		if startErr != nil {
			return fmt.Errorf("start SSH session: %w", startErr)
		}
	case strings.EqualFold(shell, "pwsh") || strings.EqualFold(shell, "pwsh.exe"):
		session, startErr = terminal.StartPowerShell(cols, rows)
		if startErr != nil {
			return fmt.Errorf("start PowerShell session: %w", startErr)
		}
	case shell != "":
		return fmt.Errorf("unsupported shell %q", shell)
	}
	if session != nil {
		defer session.Close()
		view.AttachSession(session)
	}

	// Track the container as the GUI lays out tabs, collapses the list or
	// moves the main window. The view is a top-level window (IME needs that),
	// so it must follow the container rectangle explicitly.
	go func() {
		visible := false
		var last rect
		for {
			time.Sleep(30 * time.Millisecond)
			if alive, _, _ := hostIsWindow.Call(parent); alive == 0 {
				os.Exit(0)
			}
			containerVisible, _, _ := procIsWindowVisible.Call(parent)
			ownerIconic, _, _ := procIsIconic.Call(owner)
			if containerVisible == 0 || ownerIconic != 0 {
				if visible {
					view.Show(false)
					visible = false
				}
				continue
			}
			r, ok := containerScreenRect(parent)
			if !ok {
				continue
			}
			if !visible || r != last {
				view.SetScreenBounds(int(r.Left), int(r.Top), int(r.Right-r.Left), int(r.Bottom-r.Top))
				last = r
				if !visible {
					view.Show(true)
					visible = true
				}
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

func containerScreenRect(container uintptr) (rect, bool) {
	var client rect
	procGetClientRect.Call(container, uintptr(unsafe.Pointer(&client)))
	w, h := client.Right-client.Left, client.Bottom-client.Top
	if w <= 0 || h <= 0 {
		return rect{}, false
	}
	var origin point
	procClientToScreen.Call(container, uintptr(unsafe.Pointer(&origin)))
	return rect{Left: origin.X, Top: origin.Y, Right: origin.X + w, Bottom: origin.Y + h}, true
}

// The GUI grants foreground rights with AllowSetForegroundWindow; retry briefly
// while the owner stays in front so a freshly opened tab is ready to type in.
func ensureForeground(hwnd, owner uintptr) {
	for i := 0; i < 10; i++ {
		fg, _, _ := procGetForegroundWindow.Call()
		if fg == hwnd {
			return
		}
		if fg != owner {
			return
		}
		if r, _, _ := procSetForegroundWindow.Call(hwnd); r != 0 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
