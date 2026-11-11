//go:build windows

package gui

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

var (
	clipUser32   = syscall.NewLazyDLL("user32.dll")
	clipKernel32 = syscall.NewLazyDLL("kernel32.dll")

	procOpenClipboard    = clipUser32.NewProc("OpenClipboard")
	procCloseClipboard   = clipUser32.NewProc("CloseClipboard")
	procEmptyClipboard   = clipUser32.NewProc("EmptyClipboard")
	procSetClipboardData = clipUser32.NewProc("SetClipboardData")
	procGetClipboardData = clipUser32.NewProc("GetClipboardData")

	procGlobalAlloc  = clipKernel32.NewProc("GlobalAlloc")
	procGlobalLock   = clipKernel32.NewProc("GlobalLock")
	procGlobalUnlock = clipKernel32.NewProc("GlobalUnlock")
	procGlobalFree   = clipKernel32.NewProc("GlobalFree")
)

const (
	cfUnicodeText = 13     // CF_UNICODETEXT
	gmemMoveable  = 0x0002 // GMEM_MOVEABLE
)

// SetClipboardText sets the current clipboard text as UTF-16 Unicode.
// If hwnd is 0, the clipboard is associated with the current task.
func SetClipboardText(hwnd uintptr, text string) error {
	if text == "" {
		return nil
	}
	u16 := syscall.StringToUTF16(text)
	byteLen := len(u16) * 2

	hMem, _, _ := procGlobalAlloc.Call(gmemMoveable, uintptr(byteLen))
	if hMem == 0 {
		return fmt.Errorf("GlobalAlloc failed")
	}

	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		procGlobalFree.Call(hMem)
		return fmt.Errorf("GlobalLock failed")
	}

	dest := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), byteLen)
	src := unsafe.Slice((*byte)(unsafe.Pointer(&u16[0])), byteLen)
	copy(dest, src)

	procGlobalUnlock.Call(hMem)

	// Attempt to open the clipboard with retries
	var opened bool
	for i := 0; i < 5; i++ {
		r, _, _ := procOpenClipboard.Call(hwnd)
		if r != 0 {
			opened = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !opened {
		procGlobalFree.Call(hMem)
		return fmt.Errorf("OpenClipboard failed")
	}
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()

	r, _, _ := procSetClipboardData.Call(cfUnicodeText, hMem)
	if r == 0 {
		procGlobalFree.Call(hMem)
		return fmt.Errorf("SetClipboardData failed")
	}
	// System owns hMem upon success; do not free
	return nil
}

// GetClipboardText reads the current Unicode text from the clipboard.
func GetClipboardText(hwnd uintptr) (string, error) {
	var opened bool
	for i := 0; i < 5; i++ {
		r, _, _ := procOpenClipboard.Call(hwnd)
		if r != 0 {
			opened = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !opened {
		return "", fmt.Errorf("OpenClipboard failed")
	}
	defer procCloseClipboard.Call()

	hMem, _, _ := procGetClipboardData.Call(cfUnicodeText)
	if hMem == 0 {
		return "", fmt.Errorf("no unicode text in clipboard")
	}

	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		return "", fmt.Errorf("GlobalLock failed")
	}
	defer procGlobalUnlock.Call(hMem)

	var u16 []uint16
	for p := (*uint16)(unsafe.Pointer(ptr)); *p != 0; p = (*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + 2)) {
		u16 = append(u16, *p)
	}
	return syscall.UTF16ToString(u16), nil
}
