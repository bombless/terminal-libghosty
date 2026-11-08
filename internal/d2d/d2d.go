//go:build windows

package d2d

import (
	"fmt"
	"syscall"
	"unsafe"
)

type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	d2d1   = syscall.NewLazyDLL("d2d1.dll")
	dwrite = syscall.NewLazyDLL("dwrite.dll")

	procD2D1CreateFactory   = d2d1.NewProc("D2D1CreateFactory")
	procDWriteCreateFactory = dwrite.NewProc("DWriteCreateFactory")

	iidID2D1Factory = GUID{
		Data1: 0x06152247,
		Data2: 0x6f50,
		Data3: 0x465a,
		Data4: [8]byte{0x92, 0x45, 0x11, 0x8b, 0xfd, 0x3b, 0x60, 0x07},
	}
	iidIDWriteFactory = GUID{
		Data1: 0xb859ee5a,
		Data2: 0xd838,
		Data3: 0x4b5b,
		Data4: [8]byte{0xa2, 0xe8, 0x1a, 0xdc, 0x7d, 0x93, 0xdb, 0x48},
	}
)

type PixelFormat struct {
	Format    uint32
	AlphaMode uint32
}

type RenderTargetProperties struct {
	Type        uint32
	PixelFormat PixelFormat
	DpiX        float32
	DpiY        float32
	Usage       uint32
	MinLevel    uint32
}

type SizeU struct {
	Width  uint32
	Height uint32
}

type HwndRenderTargetProperties struct {
	Hwnd           uintptr
	PixelSize      SizeU
	PresentOptions uint32
}

type Color struct {
	R, G, B, A float32
}

type RectF struct {
	Left, Top, Right, Bottom float32
}

func CallVtbl(obj uintptr, methodIndex int, params ...uintptr) uintptr {
	vtbl := *(*uintptr)(unsafe.Pointer(obj))
	proc := *(*uintptr)(unsafe.Pointer(vtbl + uintptr(methodIndex)*unsafe.Sizeof(uintptr(0))))
	args := append([]uintptr{obj}, params...)
	r1, _, _ := syscall.SyscallN(proc, args...)
	return r1
}

func Release(obj uintptr) {
	if obj != 0 {
		CallVtbl(obj, 2)
	}
}

type Context struct {
	D2DFactory    uintptr
	DWriteFactory uintptr
}

func NewContext() (*Context, error) {
	var d2dFactory uintptr
	hr, _, err := procD2D1CreateFactory.Call(
		0, // D2D1_FACTORY_TYPE_SINGLE_THREADED
		uintptr(unsafe.Pointer(&iidID2D1Factory)),
		0,
		uintptr(unsafe.Pointer(&d2dFactory)),
	)
	if hr != 0 {
		return nil, fmt.Errorf("D2D1CreateFactory failed: 0x%08X (err: %v)", uint32(hr), err)
	}

	var dwFactory uintptr
	hr, _, err = procDWriteCreateFactory.Call(
		0, // DWRITE_FACTORY_TYPE_SHARED
		uintptr(unsafe.Pointer(&iidIDWriteFactory)),
		uintptr(unsafe.Pointer(&dwFactory)),
	)
	if hr != 0 {
		Release(d2dFactory)
		return nil, fmt.Errorf("DWriteCreateFactory failed: 0x%08X (err: %v)", uint32(hr), err)
	}

	return &Context{
		D2DFactory:    d2dFactory,
		DWriteFactory: dwFactory,
	}, nil
}

func (c *Context) Close() {
	if c.D2DFactory != 0 {
		Release(c.D2DFactory)
		c.D2DFactory = 0
	}
	if c.DWriteFactory != 0 {
		Release(c.DWriteFactory)
		c.DWriteFactory = 0
	}
}

func (c *Context) CreateTextFormat(fontFamily string, fontSize float32) (uintptr, error) {
	wFont, err := syscall.UTF16PtrFromString(fontFamily)
	if err != nil {
		return 0, err
	}
	wLocale, _ := syscall.UTF16PtrFromString("en-us")
	var textFormat uintptr
	fontSizeBits := uintptr(*(*uint32)(unsafe.Pointer(&fontSize)))
	hr := CallVtbl(
		c.DWriteFactory,
		15, // CreateTextFormat
		uintptr(unsafe.Pointer(wFont)),
		0,   // fontCollection = NULL
		400, // DWRITE_FONT_WEIGHT_NORMAL
		0,   // DWRITE_FONT_STYLE_NORMAL
		5,   // DWRITE_FONT_STRETCH_NORMAL
		fontSizeBits,
		uintptr(unsafe.Pointer(wLocale)),
		uintptr(unsafe.Pointer(&textFormat)),
	)
	if hr != 0 {
		return 0, fmt.Errorf("CreateTextFormat failed: 0x%08X", uint32(hr))
	}
	return textFormat, nil
}

type RenderTarget struct {
	handle uintptr
}

func (c *Context) CreateHwndRenderTarget(hwnd uintptr, width, height uint32) (*RenderTarget, error) {
	rtProps := RenderTargetProperties{}
	hwndProps := HwndRenderTargetProperties{
		Hwnd:           hwnd,
		PixelSize:      SizeU{Width: width, Height: height},
		PresentOptions: 0,
	}
	var rt uintptr
	hr := CallVtbl(
		c.D2DFactory,
		14, // CreateHwndRenderTarget
		uintptr(unsafe.Pointer(&rtProps)),
		uintptr(unsafe.Pointer(&hwndProps)),
		uintptr(unsafe.Pointer(&rt)),
	)
	if hr != 0 {
		return nil, fmt.Errorf("CreateHwndRenderTarget failed: 0x%08X", uint32(hr))
	}
	return &RenderTarget{handle: rt}, nil
}

func (rt *RenderTarget) Handle() uintptr {
	return rt.handle
}

func (rt *RenderTarget) Release() {
	if rt.handle != 0 {
		Release(rt.handle)
		rt.handle = 0
	}
}

func (rt *RenderTarget) BeginDraw() {
	CallVtbl(rt.handle, 48)
}

func (rt *RenderTarget) EndDraw() error {
	hr := CallVtbl(rt.handle, 49, 0, 0)
	if hr != 0 {
		return fmt.Errorf("EndDraw failed: 0x%08X", uint32(hr))
	}
	return nil
}

func (rt *RenderTarget) Clear(color Color) {
	CallVtbl(rt.handle, 47, uintptr(unsafe.Pointer(&color)))
}

func (rt *RenderTarget) FillRectangle(rect RectF, brush uintptr) {
	CallVtbl(rt.handle, 17, uintptr(unsafe.Pointer(&rect)), brush)
}

func (rt *RenderTarget) DrawText(text string, textFormat uintptr, rect RectF, brush uintptr) {
	if len(text) == 0 {
		return
	}
	utf16Chars, err := syscall.UTF16FromString(text)
	if err != nil || len(utf16Chars) <= 1 {
		return
	}
	// stringLength excluding terminating null
	strLen := uintptr(len(utf16Chars) - 1)
	CallVtbl(
		rt.handle,
		27, // DrawText
		uintptr(unsafe.Pointer(&utf16Chars[0])),
		strLen,
		textFormat,
		uintptr(unsafe.Pointer(&rect)),
		brush,
		0, // D2D1_DRAW_TEXT_OPTIONS_NONE
		0, // DWRITE_MEASURING_MODE_NATURAL
	)
}

func (rt *RenderTarget) Resize(width, height uint32) error {
	size := SizeU{Width: width, Height: height}
	hr := CallVtbl(rt.handle, 58, uintptr(unsafe.Pointer(&size)))
	if hr != 0 {
		return fmt.Errorf("Resize failed: 0x%08X", uint32(hr))
	}
	return nil
}

func (rt *RenderTarget) CreateSolidColorBrush(initialColor Color) (uintptr, error) {
	var brush uintptr
	hr := CallVtbl(
		rt.handle,
		8, // CreateSolidColorBrush
		uintptr(unsafe.Pointer(&initialColor)),
		0,
		uintptr(unsafe.Pointer(&brush)),
	)
	if hr != 0 {
		return 0, fmt.Errorf("CreateSolidColorBrush failed: 0x%08X", uint32(hr))
	}
	return brush, nil
}

func SetBrushColor(brush uintptr, color Color) {
	CallVtbl(brush, 8, uintptr(unsafe.Pointer(&color)))
}
