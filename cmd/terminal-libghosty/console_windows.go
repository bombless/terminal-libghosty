//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	consoleKernel32 = syscall.NewLazyDLL("kernel32.dll")
	consoleUser32   = syscall.NewLazyDLL("user32.dll")

	procGetConsoleWindow      = consoleKernel32.NewProc("GetConsoleWindow")
	procGetConsoleProcessList = consoleKernel32.NewProc("GetConsoleProcessList")
	procShowWindow            = consoleUser32.NewProc("ShowWindow")
	procMessageBoxW           = consoleUser32.NewProc("MessageBoxW")

	consoleVisible bool
)

// prepareConsole hides the console only when it was created for this process,
// which is what happens when a console-subsystem build is double-clicked. A
// console shared with the launching terminal belongs to the user, so it is left
// alone and stderr stays usable for development builds.
func prepareConsole() {
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd == 0 {
		return
	}
	var pids [2]uint32
	count, _, _ := procGetConsoleProcessList.Call(
		uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)),
	)
	if count > 1 {
		consoleVisible = true
		return
	}
	procShowWindow.Call(hwnd, 0) // SW_HIDE
}

func reportFatal(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stderr, message)
	if consoleVisible {
		return
	}
	title, _ := syscall.UTF16FromString("Terminal Libghosty")
	text, _ := syscall.UTF16FromString(message)
	procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(&text[0])),
		uintptr(unsafe.Pointer(&title[0])),
		0x10, // MB_ICONERROR
	)
}
