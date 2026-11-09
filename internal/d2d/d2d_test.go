//go:build windows

package d2d

import (
	"syscall"
	"testing"
	"unsafe"
)

type wndClass struct {
	Style                              uint32
	WndProc                            uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
}

func TestD2DContextAndRender(t *testing.T) {
	ctx, err := NewContext()
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}
	defer ctx.Close()

	textFormat, err := ctx.CreateTextFormat("Consolas", 15.0, FontStyleNormal)
	if err != nil {
		t.Fatalf("CreateTextFormat failed: %v", err)
	}
	defer Release(textFormat)

	u32 := syscall.NewLazyDLL("user32.dll")
	procCreateWindowExW := u32.NewProc("CreateWindowExW")
	procDestroyWindow := u32.NewProc("DestroyWindow")
	procDefWindowProcW := u32.NewProc("DefWindowProcW")
	procRegisterClassW := u32.NewProc("RegisterClassW")

	clsName, _ := syscall.UTF16PtrFromString("TestD2DPkgClass")
	wc := wndClass{WndProc: procDefWindowProcW.Addr(), ClassName: clsName}
	procRegisterClassW.Call(uintptr(unsafe.Pointer(&wc)))

	title, _ := syscall.UTF16PtrFromString("TestD2DPkg")
	hwnd, _, _ := procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(clsName)), uintptr(unsafe.Pointer(title)),
		0x80000000, 0, 0, 400, 300, 0, 0, 0, 0,
	)
	if hwnd == 0 {
		t.Fatal("CreateWindowExW failed")
	}
	defer procDestroyWindow.Call(hwnd)

	rt, err := ctx.CreateHwndRenderTarget(hwnd, 400, 300)
	if err != nil {
		t.Fatalf("CreateHwndRenderTarget failed: %v", err)
	}
	defer rt.Release()

	brush, err := rt.CreateSolidColorBrush(Color{R: 1, G: 0, B: 0, A: 1})
	if err != nil {
		t.Fatalf("CreateSolidColorBrush failed: %v", err)
	}
	defer Release(brush)

	rt.BeginDraw()
	rt.Clear(Color{R: 0.1, G: 0.1, B: 0.1, A: 1.0})
	SetBrushColor(brush, Color{R: 0, G: 1, B: 0, A: 1})
	rt.FillRectangle(RectF{Left: 10, Top: 10, Right: 50, Bottom: 50}, brush)
	rt.DrawText("Hello D2D", textFormat, RectF{Left: 60, Top: 10, Right: 200, Bottom: 50}, brush)
	if err := rt.EndDraw(); err != nil {
		t.Fatalf("EndDraw failed: %v", err)
	}

	if err := rt.Resize(500, 400); err != nil {
		t.Fatalf("Resize failed: %v", err)
	}
}
