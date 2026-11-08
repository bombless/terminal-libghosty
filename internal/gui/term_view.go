//go:build windows

package gui

import (
	"bytes"
	"fmt"
	"log"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/bombless/terminal-libghosty/internal/d2d"
	"github.com/bombless/terminal-libghosty/internal/terminal"
)

var (
	u32                      = syscall.NewLazyDLL("user32.dll")
	g32                      = syscall.NewLazyDLL("gdi32.dll")
	procCreateWindowExW      = u32.NewProc("CreateWindowExW")
	procDestroyWindow        = u32.NewProc("DestroyWindow")
	procRegisterClassW       = u32.NewProc("RegisterClassW")
	procDefWindowProcW       = u32.NewProc("DefWindowProcW")
	procShowWindow           = u32.NewProc("ShowWindow")
	procUpdateWindow         = u32.NewProc("UpdateWindow")
	procSetFocus             = u32.NewProc("SetFocus")
	procBeginPaint           = u32.NewProc("BeginPaint")
	procEndPaint             = u32.NewProc("EndPaint")
	procGetClientRect        = u32.NewProc("GetClientRect")
	procMoveWindow           = u32.NewProc("MoveWindow")
	procSetTimer             = u32.NewProc("SetTimer")
	procKillTimer            = u32.NewProc("KillTimer")
	procInvalidateRect       = u32.NewProc("InvalidateRect")
	procGetWindowLongPtrW    = u32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW    = u32.NewProc("SetWindowLongPtrW")
	procGetKeyState          = u32.NewProc("GetKeyState")
	procSendMessageW         = u32.NewProc("SendMessageW")
	procSetWindowTextW       = u32.NewProc("SetWindowTextW")
	procGetWindowTextW       = u32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW = u32.NewProc("GetWindowTextLengthW")
	procPostQuitMessage      = u32.NewProc("PostQuitMessage")
	procGetMessageW          = u32.NewProc("GetMessageW")
	procTranslateMessage     = u32.NewProc("TranslateMessage")
	procDispatchMessageW     = u32.NewProc("DispatchMessageW")
	procEnableWindow         = u32.NewProc("EnableWindow")
	procDrawTextW            = u32.NewProc("DrawTextW")
	procFillRect             = u32.NewProc("FillRect")
	procCreateSolidBrush     = g32.NewProc("CreateSolidBrush")
	procDeleteObject         = g32.NewProc("DeleteObject")
	procSetTextColor         = g32.NewProc("SetTextColor")
	procSetBkColor           = g32.NewProc("SetBkColor")
	procSetBkMode            = g32.NewProc("SetBkMode")
	procCreateFontW          = g32.NewProc("CreateFontW")
	procSelectObject         = g32.NewProc("SelectObject")
	procGetCaretBlinkTime    = u32.NewProc("GetCaretBlinkTime")
)

const (
	gwlpUserData    = ^uintptr(20) // -21 in two's complement
	wmDestroy       = 2
	wmSize          = 5
	wmSetFocus      = 7
	wmKillFocus     = 8
	wmPaint         = 15
	wmClose         = 0x0010
	wmEraseBkgnd    = 0x14
	wmGetDlgCode    = 0x0087
	wmKeyDown       = 0x100
	wmChar          = 0x102
	wmTimer         = 0x113
	wmLButtonDown   = 0x201
	wmCommand       = 0x0111
	wmCtlColorStatic = 0x0138
	wmCtlColorEdit  = 0x0133
	wmCtlColorBtn   = 0x0135
	wmCtlColorListbox = 0x0134
	wmDrawItem      = 0x002B

	dlgcWantAllKeys = 0x0004
	dlgcWantArrows  = 0x0001
	dlgcWantTab     = 0x0002
	dlgcWantChars   = 0x0080

	wsChild         = 0x40000000
	wsVisible       = 0x10000000
	wsTabStop       = 0x10000
	wsClipChildren  = 0x02000000
	wsClipSiblings  = 0x04000000
	wsOverlapped    = 0x00000000
	wsCaption       = 0x00C00000
	wsSysMenu       = 0x00080000
	wsMinimizeBox   = 0x00020000
	wsMaximizeBox   = 0x00010000
	wsThickFrame    = 0x00040000
	wsOverlappedWindow = wsOverlapped | wsCaption | wsSysMenu | wsThickFrame | wsMinimizeBox | wsMaximizeBox
)

type wndClass struct {
	Style                              uint32
	WndProc                            uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
}

type rect struct {
	Left, Top, Right, Bottom int32
}

type paintStruct struct {
	Hdc                uintptr
	Erase              int32
	Paint              rect
	Restore, IncUpdate int32
	Reserved           [32]byte
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

type drawItemStruct struct {
	CtlType    uint32
	CtlID      uint32
	ItemID     uint32
	ItemAction uint32
	ItemState  uint32
	HwndItem   uintptr
	Hdc        uintptr
	RcItem     rect
	ItemData   uintptr
}

type TerminalView struct {
	hwnd       uintptr
	sess       *terminal.Session
	d2dCtx     *d2d.Context
	rt         *d2d.RenderTarget
	brush      uintptr
	textFormat uintptr
	cellWidth  float32
	cellHeight float32
	cols       uint16
	rows       uint16

	frameReady  bool
	frameCols   uint16
	frameRows   uint16
	frameCells  []terminal.Cell
	frameText   []byte
	frameCursor terminal.Cursor

	focused            bool
	cursorBlinkVisible bool
	lastBlinkToggle    time.Time
	blinkInterval      time.Duration

	processExited bool
	onExit        func(code uint32)
	mu            sync.Mutex
}

var (
	registerD2DViewOnce sync.Once
	d2dViewClassName    = "TerminalLibghostyD2DView"
)

func utf16Ptr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func termViewWndProc(hwnd uintptr, m uint32, w, l uintptr) uintptr {
	ptr, _, _ := procGetWindowLongPtrW.Call(hwnd, uintptr(gwlpUserData))
	tv := (*TerminalView)(unsafe.Pointer(ptr))

	switch m {
	case wmGetDlgCode:
		return dlgcWantAllKeys | dlgcWantArrows | dlgcWantTab | dlgcWantChars

	case wmSetFocus:
		if tv != nil {
			tv.mu.Lock()
			tv.focused = true
			tv.cursorBlinkVisible = true
			tv.lastBlinkToggle = time.Now()
			tv.mu.Unlock()
			procInvalidateRect.Call(hwnd, 0, 0)
		}
		return 0

	case wmKillFocus:
		if tv != nil {
			tv.mu.Lock()
			tv.focused = false
			tv.cursorBlinkVisible = true
			tv.mu.Unlock()
			procInvalidateRect.Call(hwnd, 0, 0)
		}
		return 0

	case wmLButtonDown:
		procSetFocus.Call(hwnd)
		return 0

	case wmEraseBkgnd:
		return 1

	case wmSize:
		if tv != nil {
			var r rect
			procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
			width := uint32(r.Right - r.Left)
			height := uint32(r.Bottom - r.Top)
			if width > 0 && height > 0 {
				if tv.rt != nil {
					_ = tv.rt.Resize(width, height)
				}
				c := uint16(width / uint32(tv.cellWidth))
				rr := uint16(height / uint32(tv.cellHeight))
				if c < 1 {
					c = 1
				}
				if rr < 1 {
					rr = 1
				}
				if (c != tv.cols || rr != tv.rows) && tv.sess != nil {
					tv.cols, tv.rows = c, rr
					_ = tv.sess.Resize(c, rr)
				}
			}
		}
		return 0

	case wmChar:
		if tv != nil && tv.sess != nil && w <= 0x10FFFF {
			tv.resetCursorBlink()
			buf := []byte(string(rune(w)))
			_ = tv.sess.Write(buf)
		}
		return 0

	case wmKeyDown:
		if tv != nil && tv.sess != nil {
			seq := ""
			switch w {
			case 0x0D:
				seq = "\r"
			case 0x08:
				seq = "\x7f"
			case 0x09:
				seq = "\t"
			case 0x1B:
				seq = "\x1b"
			case 0x25: // Left
				seq = "\x1b[D"
			case 0x26: // Up
				seq = "\x1b[A"
			case 0x27: // Right
				seq = "\x1b[C"
			case 0x28: // Down
				seq = "\x1b[B"
			case 0x24: // Home
				seq = "\x1b[H"
			case 0x23: // End
				seq = "\x1b[F"
			case 0x2D: // Insert
				seq = "\x1b[2~"
			case 0x2E: // Delete
				seq = "\x1b[3~"
			case 0x21: // PageUp
				seq = "\x1b[5~"
			case 0x22: // PageDown
				seq = "\x1b[6~"
			}
			if seq != "" {
				tv.resetCursorBlink()
				_ = tv.sess.Write([]byte(seq))
				return 0
			}
		}

	case wmTimer:
		if tv != nil {
			tv.pollState()
		}
		return 0

	case wmPaint:
		var ps paintStruct
		procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		if tv != nil {
			tv.render()
		}
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0

	case wmDestroy:
		if tv != nil {
			procKillTimer.Call(hwnd, 1)
			tv.cleanupD2D()
		}
		return 0
	}

	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(m), w, l)
	return r
}

func registerD2DViewClass() {
	registerD2DViewOnce.Do(func() {
		cb := syscall.NewCallback(termViewWndProc)
		name := utf16Ptr(d2dViewClassName)
		cursor, _, _ := procLoadCursorW.Call(0, 32512)
		wc := wndClass{
			WndProc:   cb,
			ClassName: name,
			Cursor:    cursor,
		}
		procRegisterClassW.Call(uintptr(unsafe.Pointer(&wc)))
	})
}

func NewTerminalView(parentHwnd uintptr, onExit func(code uint32)) (*TerminalView, error) {
	registerD2DViewClass()

	ctx, err := d2d.NewContext()
	if err != nil {
		return nil, fmt.Errorf("init Direct2D: %w", err)
	}

	textFormat, err := ctx.CreateTextFormat("Consolas", 15.0)
	if err != nil {
		ctx.Close()
		return nil, fmt.Errorf("create DirectWrite text format: %w", err)
	}

	blinkInterval := 500 * time.Millisecond
	if r, _, _ := procGetCaretBlinkTime.Call(); r > 0 && r < 0xFFFFFFFF {
		blinkInterval = time.Duration(r) * time.Millisecond
	}

	tv := &TerminalView{
		d2dCtx:             ctx,
		textFormat:         textFormat,
		cellWidth:          9.0,
		cellHeight:         18.0,
		cols:               80,
		rows:               24,
		onExit:             onExit,
		focused:            true,
		cursorBlinkVisible: true,
		lastBlinkToggle:    time.Now(),
		blinkInterval:      blinkInterval,
	}

	name := utf16Ptr(d2dViewClassName)
	title := utf16Ptr("Terminal View")
	hwnd, _, callErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(name)),
		uintptr(unsafe.Pointer(title)),
		wsChild|wsTabStop|wsClipSiblings,
		0, 0, 100, 100,
		parentHwnd,
		0, 0, 0,
	)
	if hwnd == 0 {
		ctx.Close()
		return nil, fmt.Errorf("CreateWindowExW failed: %v", callErr)
	}
	tv.hwnd = hwnd

	procSetWindowLongPtrW.Call(hwnd, uintptr(gwlpUserData), uintptr(unsafe.Pointer(tv)))

	var r rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	w := uint32(r.Right - r.Left)
	h := uint32(r.Bottom - r.Top)
	if w == 0 {
		w = 100
	}
	if h == 0 {
		h = 100
	}

	rt, err := ctx.CreateHwndRenderTarget(hwnd, w, h)
	if err != nil {
		procDestroyWindow.Call(hwnd)
		ctx.Close()
		return nil, fmt.Errorf("CreateHwndRenderTarget: %w", err)
	}
	tv.rt = rt

	brush, err := rt.CreateSolidColorBrush(d2d.Color{R: 1, G: 1, B: 1, A: 1})
	if err != nil {
		rt.Release()
		procDestroyWindow.Call(hwnd)
		ctx.Close()
		return nil, fmt.Errorf("CreateSolidColorBrush: %w", err)
	}
	tv.brush = brush

	procSetTimer.Call(hwnd, 1, 25, 0)
	return tv, nil
}

func (tv *TerminalView) HWND() uintptr {
	return tv.hwnd
}

func (tv *TerminalView) AttachSession(sess *terminal.Session) {
	tv.mu.Lock()
	defer tv.mu.Unlock()
	tv.sess = sess
	tv.processExited = false
	tv.frameReady = false
	tv.frameCells = nil
	tv.frameText = nil
	tv.focused = true
	tv.cursorBlinkVisible = true
	tv.lastBlinkToggle = time.Now()

	if sess != nil {
		var r rect
		procGetClientRect.Call(tv.hwnd, uintptr(unsafe.Pointer(&r)))
		w := uint32(r.Right - r.Left)
		h := uint32(r.Bottom - r.Top)
		c := uint16(w / uint32(tv.cellWidth))
		rr := uint16(h / uint32(tv.cellHeight))
		if c < 1 {
			c = 1
		}
		if rr < 1 {
			rr = 1
		}
		tv.cols, tv.rows = c, rr
		_ = sess.Resize(c, rr)
	}
}

func (tv *TerminalView) CloseSession() {
	tv.mu.Lock()
	defer tv.mu.Unlock()
	if tv.sess != nil {
		tv.sess.Close()
		tv.sess = nil
	}
}

func (tv *TerminalView) SetBounds(x, y, w, h int) {
	procMoveWindow.Call(tv.hwnd, uintptr(x), uintptr(y), uintptr(w), uintptr(h), 1)
}

func (tv *TerminalView) Show(visible bool) {
	cmd := 0
	if visible {
		cmd = 5 // SW_SHOW
	}
	procShowWindow.Call(tv.hwnd, uintptr(cmd))
}

func (tv *TerminalView) Focus() {
	tv.mu.Lock()
	tv.focused = true
	tv.cursorBlinkVisible = true
	tv.lastBlinkToggle = time.Now()
	tv.mu.Unlock()
	procSetFocus.Call(tv.hwnd)
}

func (tv *TerminalView) resetCursorBlink() {
	tv.mu.Lock()
	tv.cursorBlinkVisible = true
	tv.lastBlinkToggle = time.Now()
	tv.mu.Unlock()
	procInvalidateRect.Call(tv.hwnd, 0, 0)
}

func (tv *TerminalView) Destroy() {
	if tv.hwnd != 0 {
		procDestroyWindow.Call(tv.hwnd)
		tv.hwnd = 0
	}
}

func (tv *TerminalView) cleanupD2D() {
	if tv.brush != 0 {
		d2d.Release(tv.brush)
		tv.brush = 0
	}
	if tv.rt != nil {
		tv.rt.Release()
		tv.rt = nil
	}
	if tv.textFormat != 0 {
		d2d.Release(tv.textFormat)
		tv.textFormat = 0
	}
	if tv.d2dCtx != nil {
		tv.d2dCtx.Close()
		tv.d2dCtx = nil
	}
}

func (tv *TerminalView) pollState() {
	tv.mu.Lock()
	sess := tv.sess
	exited := tv.processExited
	tv.mu.Unlock()

	if sess == nil {
		return
	}

	if !exited {
		done, code, err := sess.PollExit()
		if err == nil && done {
			tv.mu.Lock()
			tv.processExited = true
			tv.mu.Unlock()
			log.Printf("Terminal SSH session exited with code %d", code)
			if tv.onExit != nil {
				tv.onExit(code)
			}
		}
	}

	cc, rr, cells, text, cursor, err := sess.Snapshot()
	if err != nil {
		return
	}

	changed := !tv.frameReady || cc != tv.frameCols || rr != tv.frameRows || cursor != tv.frameCursor || !bytes.Equal(text, tv.frameText)
	if !changed && len(cells) == len(tv.frameCells) {
		for i := range cells {
			if cells[i] != tv.frameCells[i] {
				changed = true
				break
			}
		}
	} else if len(cells) != len(tv.frameCells) {
		changed = true
	}

	now := time.Now()
	blinkChanged := false

	tv.mu.Lock()
	shouldBlink := cursor.Visible != 0 && cursor.Blinking != 0 && tv.focused
	if shouldBlink {
		interval := tv.blinkInterval
		if interval <= 0 {
			interval = 500 * time.Millisecond
		}
		if now.Sub(tv.lastBlinkToggle) >= interval {
			tv.cursorBlinkVisible = !tv.cursorBlinkVisible
			tv.lastBlinkToggle = now
			blinkChanged = true
		}
	} else {
		if !tv.cursorBlinkVisible {
			tv.cursorBlinkVisible = true
			blinkChanged = true
		}
	}

	if changed {
		tv.frameCols, tv.frameRows = cc, rr
		tv.frameCells = cells
		tv.frameText = text
		tv.frameCursor = cursor
		tv.frameReady = true
		tv.cursorBlinkVisible = true
		tv.lastBlinkToggle = now
	}
	tv.mu.Unlock()

	if changed || blinkChanged {
		procInvalidateRect.Call(tv.hwnd, 0, 0)
	}
}

func (tv *TerminalView) render() {
	if tv.rt == nil {
		return
	}

	tv.mu.Lock()
	frameReady := tv.frameReady
	cc, rr := tv.frameCols, tv.frameRows
	cells := tv.frameCells
	buf := tv.frameText
	cur := tv.frameCursor
	cursorVisible := tv.cursorBlinkVisible
	focused := tv.focused
	tv.mu.Unlock()

	tv.rt.BeginDraw()

	// Clear background: #0F131A (15, 19, 26)
	tv.rt.Clear(d2d.Color{
		R: 15.0 / 255.0,
		G: 19.0 / 255.0,
		B: 26.0 / 255.0,
		A: 1.0,
	})

	if frameReady && len(cells) > 0 {
		cw := tv.cellWidth
		ch := tv.cellHeight

		for y := uint16(0); y < rr; y++ {
			for x := uint16(0); x < cc; x++ {
				idx := int(y)*int(cc) + int(x)
				if idx >= len(cells) {
					continue
				}
				cell := cells[idx]

				cellRect := d2d.RectF{
					Left:   float32(x) * cw,
					Top:    float32(y) * ch,
					Right:  float32(x+1) * cw,
					Bottom: float32(y+1) * ch,
				}

				// Cell background
				if cell.Selected != 0 {
					d2d.SetBrushColor(tv.brush, d2d.Color{
						R: float32(0x28) / 255.0,
						G: float32(0x50) / 255.0,
						B: float32(0xA0) / 255.0,
						A: 1.0,
					})
					tv.rt.FillRectangle(cellRect, tv.brush)
				} else if cell.Background.R != 0 || cell.Background.G != 0 || cell.Background.B != 0 {
					d2d.SetBrushColor(tv.brush, d2d.Color{
						R: float32(cell.Background.R) / 255.0,
						G: float32(cell.Background.G) / 255.0,
						B: float32(cell.Background.B) / 255.0,
						A: 1.0,
					})
					tv.rt.FillRectangle(cellRect, tv.brush)
				}

				// Cell text
				end := uint64(cell.TextOffset) + uint64(cell.TextLength)
				if cell.TextLength > 0 && end <= uint64(len(buf)) {
					textStr := string(buf[cell.TextOffset:uint32(end)])
					d2d.SetBrushColor(tv.brush, d2d.Color{
						R: float32(cell.Foreground.R) / 255.0,
						G: float32(cell.Foreground.G) / 255.0,
						B: float32(cell.Foreground.B) / 255.0,
						A: 1.0,
					})
					// DrawText with slight top padding for baseline centering
					textRect := d2d.RectF{
						Left:   float32(x) * cw,
						Top:    float32(y)*ch + 1.0,
						Right:  float32(x+1) * cw,
						Bottom: float32(y+1) * ch,
					}
					tv.rt.DrawText(textStr, tv.textFormat, textRect, tv.brush)
				}
			}
		}

		// Cursor
		if cur.Visible != 0 && cur.X < cc && cur.Y < rr {
			drawCursor := true
			if cur.Blinking != 0 && focused {
				drawCursor = cursorVisible
			}
			if drawCursor {
				d2d.SetBrushColor(tv.brush, d2d.Color{
					R: float32(0xE8) / 255.0,
					G: float32(0xE8) / 255.0,
					B: float32(0xE8) / 255.0,
					A: 0.85,
				})
				cursorRect := d2d.RectF{
					Left:   float32(cur.X) * cw,
					Top:    float32(cur.Y) * ch,
					Right:  float32(cur.X)*cw + 2.0,
					Bottom: float32(cur.Y+1) * ch,
				}
				tv.rt.FillRectangle(cursorRect, tv.brush)
			}
		}
	}

	_ = tv.rt.EndDraw()
}
