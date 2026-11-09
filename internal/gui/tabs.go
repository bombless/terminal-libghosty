//go:build windows

package gui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const (
	tabConfig = iota
	tabSSH
	tabPowerShell
)

const (
	tabStripHeight   = 40
	tabItemMaxWidth  = 170
	tabItemMinWidth  = 84
	tabItemGap       = 6
	tabItemTop       = 4
	tabItemBottom    = 36
	tabItemPadX      = 10
	tabCloseWidth    = 24
	dtLeft           = 0x0000
	dtEndEllipsis    = 0x8000
	wmAppSessionExit = 0x8001
)

type sessionTab struct {
	id        uint32
	kind      int
	title     string
	container uintptr
	serverCmd *exec.Cmd
}

// tabItemWidth shrinks every tab evenly so the strip fits the current tab count.
func (mw *MainWindow) tabItemWidth() int32 {
	var client rect
	procGetClientRect.Call(mw.hTabStrip, uintptr(unsafe.Pointer(&client)))
	width := int32(tabItemMaxWidth)
	if n := int32(len(mw.tabs)); n > 0 {
		fit := (client.Right - 16 - (n-1)*tabItemGap) / n
		if fit < width {
			width = fit
		}
	}
	if width < tabItemMinWidth {
		width = tabItemMinWidth
	}
	return width
}

func (mw *MainWindow) tabItemRect(i int) rect {
	width := mw.tabItemWidth()
	left := int32(8) + int32(i)*(width+tabItemGap)
	return rect{Left: left, Top: tabItemTop, Right: left + width, Bottom: tabItemBottom}
}

func (mw *MainWindow) tabCloseRect(r rect) rect {
	return rect{Left: r.Right - tabCloseWidth + 4, Top: r.Top, Right: r.Right - 4, Bottom: r.Bottom}
}

func pointInRect(r rect, x, y int32) bool {
	return x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

func (mw *MainWindow) paintTabStrip(hdc uintptr) {
	var client rect
	procGetClientRect.Call(mw.hTabStrip, uintptr(unsafe.Pointer(&client)))
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&client)), mw.bgBrush)
	procSetBkMode.Call(hdc, 1) // TRANSPARENT
	if mw.subFont != 0 {
		procSelectObject.Call(hdc, mw.subFont)
	}
	for i, t := range mw.tabs {
		r := mw.tabItemRect(i)
		if i == mw.activeTab {
			procFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), mw.panelBrush)
			accent := rect{Left: r.Left, Top: r.Bottom - 3, Right: r.Right, Bottom: r.Bottom}
			procFillRect.Call(hdc, uintptr(unsafe.Pointer(&accent)), mw.btnBrush)
			procSetTextColor.Call(hdc, 0x00FFFFFF)
		} else {
			procSetTextColor.Call(hdc, 0x00C0C0C0)
		}
		textRect := r
		textRect.Left += tabItemPadX
		if i != tabConfig {
			textRect.Right -= tabCloseWidth
		}
		title, _ := syscall.UTF16FromString(t.title)
		procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)-1),
			uintptr(unsafe.Pointer(&textRect)), dtLeft|dtVcenter|dtSingleline|dtEndEllipsis)
		if i != tabConfig {
			closeRect := mw.tabCloseRect(r)
			closeText, _ := syscall.UTF16FromString("×")
			procDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&closeText[0])), uintptr(len(closeText)-1),
				uintptr(unsafe.Pointer(&closeRect)), dtCenter|dtVcenter|dtSingleline)
		}
	}
}

func (mw *MainWindow) handleTabStripClick(l uintptr) {
	x := int32(int16(l & 0xFFFF))
	y := int32(int16((l >> 16) & 0xFFFF))
	for i := range mw.tabs {
		r := mw.tabItemRect(i)
		if !pointInRect(r, x, y) {
			continue
		}
		if i != tabConfig && pointInRect(mw.tabCloseRect(r), x, y) {
			mw.closeTab(i)
			return
		}
		mw.activateTab(i)
		return
	}
}

func tabStripWndProc(hwnd uintptr, m uint32, w, l uintptr) uintptr {
	ptr, _, _ := procGetWindowLongPtrW.Call(hwnd, uintptr(gwlpUserData))
	mw := (*MainWindow)(unsafe.Pointer(ptr))

	switch m {
	case wmEraseBkgnd:
		if mw != nil {
			var r rect
			procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
			procFillRect.Call(w, uintptr(unsafe.Pointer(&r)), mw.bgBrush)
			return 1
		}
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		if mw != nil {
			mw.paintTabStrip(hdc)
		}
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmLButtonDown:
		if mw != nil {
			mw.handleTabStripClick(l)
		}
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(m), w, l)
	return r
}

func (mw *MainWindow) startSSHSession(target, identity string, port int, title string) {
	args := []string{"--ssh", target}
	if identity != "" {
		args = append(args, "--identity", identity)
	}
	if port > 0 {
		args = append(args, "--port", strconv.Itoa(port))
	}
	if strings.TrimSpace(title) == "" {
		title = target
	}
	mw.startSession(tabSSH, title, args, fmt.Sprintf("SSH session started for %s", target))
}

func (mw *MainWindow) startPowerShellSession() {
	if _, err := exec.LookPath("pwsh.exe"); err != nil {
		mw.setStatus("pwsh.exe not found: install PowerShell 7")
		return
	}
	mw.startSession(tabPowerShell, "PowerShell", []string{"--shell", "pwsh"}, "PowerShell session started")
}

// openProfileSession opens a new SSH tab from the saved profile, even when an
// identical session is already open.
func (mw *MainWindow) openProfileSession(idx int) {
	if idx < 0 || idx >= len(mw.profiles) {
		return
	}
	p := mw.profiles[idx]
	mw.startSSHSession(p.Target, p.Identity, p.Port, p.Name)
}

func (mw *MainWindow) startSession(kind int, title string, sessionArgs []string, status string) {
	exe, err := os.Executable()
	if err != nil {
		mw.setStatus(fmt.Sprintf("Cannot locate application executable: %v", err))
		return
	}
	container, _, _ := procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(utf16Ptr(panelWindowClassName))), 0,
		wsChild|wsClipChildren, 0, 0, 10, 10, mw.hRightPanel, 0, 0, 0,
	)
	if container == 0 {
		mw.setStatus("Failed to create terminal container.")
		return
	}
	procSetWindowLongPtrW.Call(container, uintptr(gwlpUserData), uintptr(unsafe.Pointer(mw)))

	mw.nextTabID++
	tab := &sessionTab{id: mw.nextTabID, kind: kind, title: title, container: container}
	mw.tabs = append(mw.tabs, tab)
	idx := len(mw.tabs) - 1
	mw.layout()

	args := append([]string{"--server", "--parent", fmt.Sprintf("0x%X", container)}, sessionArgs...)
	cmd := exec.Command(exe, args...)
	cmd.Dir = filepath.Dir(exe)
	if err := cmd.Start(); err != nil {
		mw.removeTabAt(idx)
		mw.setStatus(fmt.Sprintf("Failed to start terminal server: %v", err))
		return
	}
	tab.serverCmd = cmd
	mw.activateTab(idx)
	mw.setStatus(status)

	mainHwnd := mw.hwnd
	id := tab.id
	go func() {
		_ = cmd.Wait()
		procPostMessageW.Call(mainHwnd, wmAppSessionExit, uintptr(id), 0)
	}()
}

func (mw *MainWindow) activateTab(i int) {
	if i < 0 || i >= len(mw.tabs) {
		return
	}
	mw.activeTab = i
	for j, t := range mw.tabs {
		cmd := uintptr(0) // SW_HIDE
		if j == i {
			cmd = 5 // SW_SHOW
		}
		procShowWindow.Call(t.container, cmd)
	}
	if i == tabConfig {
		procSetFocus.Call(mw.hEditName)
	}
	procInvalidateRect.Call(mw.hTabStrip, 0, 0)
}

func (mw *MainWindow) closeTab(i int) {
	if i <= tabConfig || i >= len(mw.tabs) {
		return
	}
	tab := mw.tabs[i]
	if tab.serverCmd != nil && tab.serverCmd.Process != nil {
		_ = tab.serverCmd.Process.Kill()
	}
	mw.removeTabAt(i)
	mw.setStatus("Session closed.")
}

func (mw *MainWindow) closeTabByID(id uint32) {
	for i, t := range mw.tabs {
		if t.kind != tabConfig && t.id == id {
			mw.removeTabAt(i)
			mw.setStatus("Session closed.")
			return
		}
	}
}

func (mw *MainWindow) removeTabAt(i int) {
	if i <= tabConfig || i >= len(mw.tabs) {
		return
	}
	tab := mw.tabs[i]
	mw.tabs = append(mw.tabs[:i], mw.tabs[i+1:]...)
	if tab.container != 0 {
		procDestroyWindow.Call(tab.container)
	}
	if mw.activeTab == i {
		next := i - 1
		if next < 0 {
			next = 0
		}
		mw.activeTab = -1
		mw.activateTab(next)
	} else if mw.activeTab > i {
		mw.activeTab--
	}
	mw.layout()
	procInvalidateRect.Call(mw.hTabStrip, 0, 0)
}
