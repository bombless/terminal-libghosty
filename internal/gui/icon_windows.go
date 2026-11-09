//go:build windows

package gui

import "syscall"

var (
	guiKernel32          = syscall.NewLazyDLL("kernel32.dll")
	procLoadImageW       = u32.NewProc("LoadImageW")
	procGetSystemMetrics = u32.NewProc("GetSystemMetrics")
	procGetModuleHandleW = guiKernel32.NewProc("GetModuleHandleW")
)

const (
	wmSetIcon  = 0x0080
	iconSmall  = 0
	iconBig    = 1
	imageIcon  = 1
	lrShared   = 0x8000
	smCXIcon   = 11
	smCYIcon   = 12
	smCXSmIcon = 49
	smCYSmIcon = 50

	appIconResourceID uintptr = 1
)

func loadAppIcon(cx, cy int) uintptr {
	instance, _, _ := procGetModuleHandleW.Call(0)
	if instance == 0 {
		return 0
	}
	icon, _, _ := procLoadImageW.Call(
		instance, appIconResourceID, imageIcon,
		uintptr(cx), uintptr(cy), lrShared,
	)
	return icon
}

func iconMetric(index int) int {
	value, _, _ := procGetSystemMetrics.Call(uintptr(index))
	return int(value)
}

func applyAppIcon(hwnd uintptr) {
	if big := loadAppIcon(iconMetric(smCXIcon), iconMetric(smCYIcon)); big != 0 {
		procSendMessageW.Call(hwnd, wmSetIcon, iconBig, big)
	}
	if small := loadAppIcon(iconMetric(smCXSmIcon), iconMetric(smCYSmIcon)); small != 0 {
		procSendMessageW.Call(hwnd, wmSetIcon, iconSmall, small)
	}
}
