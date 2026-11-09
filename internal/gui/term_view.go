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
	u32                          = syscall.NewLazyDLL("user32.dll")
	g32                          = syscall.NewLazyDLL("gdi32.dll")
	imm32                        = syscall.NewLazyDLL("imm32.dll")
	procCreateWindowExW          = u32.NewProc("CreateWindowExW")
	procDestroyWindow            = u32.NewProc("DestroyWindow")
	procRegisterClassW           = u32.NewProc("RegisterClassW")
	procDefWindowProcW           = u32.NewProc("DefWindowProcW")
	procShowWindow               = u32.NewProc("ShowWindow")
	procUpdateWindow             = u32.NewProc("UpdateWindow")
	procSetFocus                 = u32.NewProc("SetFocus")
	procBeginPaint               = u32.NewProc("BeginPaint")
	procEndPaint                 = u32.NewProc("EndPaint")
	procGetClientRect            = u32.NewProc("GetClientRect")
	procMoveWindow               = u32.NewProc("MoveWindow")
	procSetTimer                 = u32.NewProc("SetTimer")
	procKillTimer                = u32.NewProc("KillTimer")
	procInvalidateRect           = u32.NewProc("InvalidateRect")
	procGetWindowLongPtrW        = u32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW        = u32.NewProc("SetWindowLongPtrW")
	procGetKeyState              = u32.NewProc("GetKeyState")
	procSendMessageW             = u32.NewProc("SendMessageW")
	procSetWindowTextW           = u32.NewProc("SetWindowTextW")
	procGetWindowTextW           = u32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW     = u32.NewProc("GetWindowTextLengthW")
	procPostQuitMessage          = u32.NewProc("PostQuitMessage")
	procPostMessageW             = u32.NewProc("PostMessageW")
	procGetMessageW              = u32.NewProc("GetMessageW")
	procTranslateMessage         = u32.NewProc("TranslateMessage")
	procDispatchMessageW         = u32.NewProc("DispatchMessageW")
	procEnableWindow             = u32.NewProc("EnableWindow")
	procDrawTextW                = u32.NewProc("DrawTextW")
	procFillRect                 = u32.NewProc("FillRect")
	procCreateSolidBrush         = g32.NewProc("CreateSolidBrush")
	procDeleteObject             = g32.NewProc("DeleteObject")
	procSetTextColor             = g32.NewProc("SetTextColor")
	procSetBkColor               = g32.NewProc("SetBkColor")
	procSetBkMode                = g32.NewProc("SetBkMode")
	procCreateFontW              = g32.NewProc("CreateFontW")
	procSelectObject             = g32.NewProc("SelectObject")
	procGetCaretBlinkTime        = u32.NewProc("GetCaretBlinkTime")
	procClientToScreen           = u32.NewProc("ClientToScreen")
	procGetAncestor              = u32.NewProc("GetAncestor")
	procSetCapture               = u32.NewProc("SetCapture")
	procReleaseCapture           = u32.NewProc("ReleaseCapture")
	procIsWindowVisible          = u32.NewProc("IsWindowVisible")
	procIsIconic                 = u32.NewProc("IsIconic")
	procSetForegroundWindow      = u32.NewProc("SetForegroundWindow")
	procGetForegroundWindow      = u32.NewProc("GetForegroundWindow")
	procAllowSetForegroundWindow = u32.NewProc("AllowSetForegroundWindow")

	procImmGetContext            = imm32.NewProc("ImmGetContext")
	procImmReleaseContext        = imm32.NewProc("ImmReleaseContext")
	procImmGetCompositionStringW = imm32.NewProc("ImmGetCompositionStringW")
	procImmSetCompositionWindow  = imm32.NewProc("ImmSetCompositionWindow")
	procImmSetCompositionFontW   = imm32.NewProc("ImmSetCompositionFontW")
)

const (
	gwlpUserData          = ^uintptr(20) // -21 in two's complement
	wmDestroy             = 2
	wmSize                = 5
	wmSetFocus            = 7
	wmKillFocus           = 8
	wmPaint               = 15
	wmClose               = 0x0010
	wmEraseBkgnd          = 0x14
	wmGetDlgCode          = 0x0087
	wmKeyDown             = 0x100
	wmChar                = 0x102
	wmImeStartComposition = 0x010D
	wmImeEndComposition   = 0x010E
	wmImeComposition      = 0x010F
	wmImeSetContext       = 0x0281
	wmImeChar             = 0x0286
	wmTimer               = 0x113
	wmLButtonDown         = 0x201
	wmLButtonUp           = 0x202
	wmMouseMove           = 0x200
	wmCaptureChanged      = 0x215
	wmCommand             = 0x0111
	wmCtlColorStatic      = 0x0138
	wmCtlColorEdit        = 0x0133
	wmCtlColorBtn         = 0x0135
	wmCtlColorListbox     = 0x0134
	wmDrawItem            = 0x002B

	dlgcWantAllKeys = 0x0004
	dlgcWantArrows  = 0x0001
	dlgcWantTab     = 0x0002
	dlgcWantChars   = 0x0080

	wsChild            = 0x40000000
	wsPopup            = 0x80000000
	wsVisible          = 0x10000000
	wsTabStop          = 0x10000
	wsClipChildren     = 0x02000000
	wsClipSiblings     = 0x04000000
	wsOverlapped       = 0x00000000
	wsCaption          = 0x00C00000
	wsSysMenu          = 0x00080000
	wsMinimizeBox      = 0x00020000
	wsMaximizeBox      = 0x00010000
	wsThickFrame       = 0x00040000
	wsOverlappedWindow = wsOverlapped | wsCaption | wsSysMenu | wsThickFrame | wsMinimizeBox | wsMaximizeBox

	swShowNoActivate = 4
	gaRoot           = 2

	gcsCompStr                 = 0x0008
	gcsResultStr               = 0x0800
	cfsPoint                   = 0x0002
	iscShowUICompositionWindow = 0x80000000
)

// GhosttyStyleFlags bit positions, matching ghostty/src/terminal/c/style.zig.
const (
	styleItalic    uint32 = 1 << 1
	styleFaint     uint32 = 1 << 2
	styleInverse   uint32 = 1 << 4
	styleInvisible uint32 = 1 << 5
)

var defaultBackground = d2d.Color{R: 15.0 / 255.0, G: 19.0 / 255.0, B: 26.0 / 255.0, A: 1.0}

type cellPaint struct {
	fg, bg    d2d.Color
	fillBg    bool
	textAlpha float32
	italic    bool
	hideText  bool
}

func rgbColor(c terminal.RGB) d2d.Color {
	return d2d.Color{R: float32(c.R) / 255.0, G: float32(c.G) / 255.0, B: float32(c.B) / 255.0, A: 1.0}
}

// paintForCell resolves Ghostty's raw cell colors and style flags the way
// Ghostty's own renderer does: faint halves glyph opacity, inverse swaps
// foreground and background, and invisible suppresses the glyph.
func paintForCell(cell terminal.Cell) cellPaint {
	p := cellPaint{
		fg:        rgbColor(cell.Foreground),
		bg:        rgbColor(cell.Background),
		fillBg:    cell.Background != (terminal.RGB{}),
		textAlpha: 1.0,
		italic:    cell.StyleFlags&styleItalic != 0,
		hideText:  cell.StyleFlags&styleInvisible != 0,
	}
	if cell.StyleFlags&styleFaint != 0 {
		p.textAlpha = 0.5
	}
	if cell.StyleFlags&styleInverse != 0 {
		fg, bg := p.fg, p.bg
		if !p.fillBg {
			bg = defaultBackground
		}
		p.fg, p.bg = bg, fg
		p.fillBg = true
	}
	return p
}

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

type compositionForm struct {
	Style      uint32
	CurrentPos point
	Area       rect
}

type logFontW struct {
	Height         int32
	Width          int32
	Escapement     int32
	Orientation    int32
	Weight         int32
	Italic         byte
	Underline      byte
	StrikeOut      byte
	CharSet        byte
	OutPrecision   byte
	ClipPrecision  byte
	Quality        byte
	PitchAndFamily byte
	FaceName       [32]uint16
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
	hwnd           uintptr
	sess           *terminal.Session
	d2dCtx         *d2d.Context
	rt             *d2d.RenderTarget
	brush          uintptr
	textFormat     uintptr
	textFormatItal uintptr
	cellWidth      float32
	cellHeight     float32
	cols           uint16
	rows           uint16

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

	preedit string
	imeFont logFontW

	selecting              bool
	selAnchorX, selAnchorY uint16
	selLastX, selLastY     uint16

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

	case wmImeSetContext:
		if tv != nil {
			tv.positionComposition()
		}
		// The composition string is rendered inline; keep the IME from painting
		// its own composition window on top of the terminal.
		l &^= iscShowUICompositionWindow
		r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(m), w, l)
		return r

	case wmImeStartComposition:
		if tv != nil {
			tv.positionComposition()
		}
		return 0

	case wmImeComposition:
		if tv != nil {
			tv.handleComposition(l)
		}
		return 0

	case wmImeChar:
		if tv != nil && tv.sess != nil && w <= 0x10FFFF {
			tv.resetCursorBlink()
			_ = tv.sess.Write([]byte(string(rune(w))))
		}
		return 0

	case wmImeEndComposition:
		if tv != nil {
			tv.setPreedit("")
		}
		return 0

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
			tv.setPreedit("")
			procInvalidateRect.Call(hwnd, 0, 0)
		}
		return 0

	case wmLButtonDown:
		procSetFocus.Call(hwnd)
		procSetCapture.Call(hwnd)
		if tv != nil {
			tv.beginSelection(l)
		}
		return 0

	case wmMouseMove:
		if tv != nil {
			tv.updateSelection(l)
		}
		return 0

	case wmLButtonUp:
		if tv != nil {
			tv.endSelection()
		}
		procReleaseCapture.Call()
		return 0

	case wmCaptureChanged:
		if tv != nil {
			tv.endSelection()
		}
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
			// Enter, Backspace, Tab and Esc already arrive as wmChar after
			// TranslateMessage; mapping them here too would double every
			// keystroke (e.g. Tab completing two path levels).
			switch w {
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
		procPostQuitMessage.Call(0)
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

	textFormat, err := ctx.CreateTextFormat("Consolas", 15.0, d2d.FontStyleNormal)
	if err != nil {
		ctx.Close()
		return nil, fmt.Errorf("create DirectWrite text format: %w", err)
	}

	textFormatItal, err := ctx.CreateTextFormat("Consolas", 15.0, d2d.FontStyleItalic)
	if err != nil {
		d2d.Release(textFormat)
		ctx.Close()
		return nil, fmt.Errorf("create DirectWrite italic text format: %w", err)
	}

	blinkInterval := 500 * time.Millisecond
	if r, _, _ := procGetCaretBlinkTime.Call(); r > 0 && r < 0xFFFFFFFF {
		blinkInterval = time.Duration(r) * time.Millisecond
	}

	tv := &TerminalView{
		d2dCtx:             ctx,
		textFormat:         textFormat,
		textFormatItal:     textFormatItal,
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
	tv.imeFont = logFontW{Height: -16, Weight: 400, CharSet: 1, PitchAndFamily: 0x31}
	copy(tv.imeFont.FaceName[:], syscall.StringToUTF16("Consolas"))

	name := utf16Ptr(d2dViewClassName)
	title := utf16Ptr("Terminal View")
	owner, _, _ := procGetAncestor.Call(parentHwnd, gaRoot)
	hwnd, _, callErr := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(name)),
		uintptr(unsafe.Pointer(title)),
		wsPopup|wsClipSiblings,
		0, 0, 100, 100,
		owner, // owned top-level window: IME only engages for our own root
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

func (tv *TerminalView) SetScreenBounds(x, y, w, h int) {
	procMoveWindow.Call(tv.hwnd, uintptr(x), uintptr(y), uintptr(w), uintptr(h), 1)
}

func (tv *TerminalView) Show(visible bool) {
	cmd := 0 // SW_HIDE
	if visible {
		cmd = swShowNoActivate
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

// cellFromLParam maps client-area pixels from a mouse message to a cell,
// clamping drags that leave the terminal window to the nearest edge.
func (tv *TerminalView) cellFromLParam(l uintptr) (uint16, uint16) {
	px := int(int16(uint16(l)))
	py := int(int16(uint16(l >> 16)))
	x, y := 0, 0
	if tv.cellWidth > 0 && tv.cellHeight > 0 {
		x = int(float32(px) / tv.cellWidth)
		y = int(float32(py) / tv.cellHeight)
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	maxX := int(tv.cols) - 1
	if maxX < 0 {
		maxX = 0
	}
	maxY := int(tv.rows) - 1
	if maxY < 0 {
		maxY = 0
	}
	if x > maxX {
		x = maxX
	}
	if y > maxY {
		y = maxY
	}
	return uint16(x), uint16(y)
}

func (tv *TerminalView) beginSelection(l uintptr) {
	x, y := tv.cellFromLParam(l)
	tv.selecting = true
	tv.selAnchorX, tv.selAnchorY = x, y
	tv.selLastX, tv.selLastY = x, y
	tv.mu.Lock()
	sess := tv.sess
	tv.mu.Unlock()
	if sess != nil {
		_ = sess.ClearSelection()
	}
}

func (tv *TerminalView) updateSelection(l uintptr) {
	if !tv.selecting {
		return
	}
	x, y := tv.cellFromLParam(l)
	if x == tv.selLastX && y == tv.selLastY {
		return
	}
	tv.selLastX, tv.selLastY = x, y
	tv.mu.Lock()
	sess := tv.sess
	tv.mu.Unlock()
	if sess != nil {
		_ = sess.Select(tv.selAnchorX, tv.selAnchorY, x, y)
	}
}

func (tv *TerminalView) endSelection() {
	tv.selecting = false
}

func (tv *TerminalView) handleComposition(l uintptr) {
	imc, _, _ := procImmGetContext.Call(tv.hwnd)
	if imc == 0 {
		return
	}
	defer procImmReleaseContext.Call(tv.hwnd, imc)

	if l&gcsResultStr != 0 {
		if s := imeCompositionString(imc, gcsResultStr); s != "" {
			tv.mu.Lock()
			sess := tv.sess
			tv.mu.Unlock()
			tv.setPreedit("")
			if sess != nil {
				tv.resetCursorBlink()
				_ = sess.Write([]byte(s))
			}
		}
	}
	if l&gcsCompStr != 0 {
		tv.setPreedit(imeCompositionString(imc, gcsCompStr))
		tv.positionComposition()
	}
}

func imeCompositionString(imc uintptr, flag uintptr) string {
	size, _, _ := procImmGetCompositionStringW.Call(imc, flag, 0, 0)
	if int32(size) <= 0 {
		return ""
	}
	buf := make([]uint16, size/2)
	procImmGetCompositionStringW.Call(imc, flag, uintptr(unsafe.Pointer(&buf[0])), size)
	return syscall.UTF16ToString(buf)
}

func (tv *TerminalView) setPreedit(s string) {
	tv.mu.Lock()
	changed := tv.preedit != s
	tv.preedit = s
	tv.mu.Unlock()
	if changed {
		procInvalidateRect.Call(tv.hwnd, 0, 0)
	}
}

func (tv *TerminalView) positionComposition() {
	imc, _, _ := procImmGetContext.Call(tv.hwnd)
	if imc == 0 {
		return
	}
	defer procImmReleaseContext.Call(tv.hwnd, imc)

	tv.mu.Lock()
	x := int32(tv.frameCursor.X) * int32(tv.cellWidth)
	y := int32(tv.frameCursor.Y) * int32(tv.cellHeight)
	tv.mu.Unlock()

	cf := compositionForm{Style: cfsPoint, CurrentPos: point{X: x, Y: y}}
	procImmSetCompositionWindow.Call(imc, uintptr(unsafe.Pointer(&cf)))
	procImmSetCompositionFontW.Call(imc, uintptr(unsafe.Pointer(&tv.imeFont)))
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
	if tv.textFormatItal != 0 {
		d2d.Release(tv.textFormatItal)
		tv.textFormatItal = 0
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
	preedit := tv.preedit
	tv.mu.Unlock()

	tv.rt.BeginDraw()

	tv.rt.Clear(defaultBackground)

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
				paint := paintForCell(cell)

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
				} else if paint.fillBg {
					d2d.SetBrushColor(tv.brush, paint.bg)
					tv.rt.FillRectangle(cellRect, tv.brush)
				}

				// Cell text
				end := uint64(cell.TextOffset) + uint64(cell.TextLength)
				if !paint.hideText && cell.TextLength > 0 && end <= uint64(len(buf)) {
					textStr := string(buf[cell.TextOffset:uint32(end)])
					fg := paint.fg
					fg.A = paint.textAlpha
					d2d.SetBrushColor(tv.brush, fg)
					format := tv.textFormat
					right := float32(x+1) * cw
					if paint.italic {
						if tv.textFormatItal != 0 {
							format = tv.textFormatItal
						}
						// Italic glyphs overhang their advance width.
						right += 2.0
					}
					// DrawText with slight top padding for baseline centering
					textRect := d2d.RectF{
						Left:   float32(x) * cw,
						Top:    float32(y)*ch + 1.0,
						Right:  right,
						Bottom: float32(y+1) * ch,
					}
					tv.rt.DrawText(textStr, format, textRect, tv.brush)
				}
			}
		}

		// IME composition string drawn at the cursor cell.
		if preedit != "" && cur.Y < rr {
			runes := []rune(preedit)
			maxCols := int(cc) - int(cur.X)
			if maxCols < 1 {
				maxCols = 1
			}
			if len(runes) > maxCols {
				runes = runes[:maxCols]
			}
			width := float32(len(runes)) * cw
			left := float32(cur.X) * cw
			top := float32(cur.Y) * ch
			bg := d2d.RectF{Left: left, Top: top, Right: left + width, Bottom: top + ch}
			d2d.SetBrushColor(tv.brush, d2d.Color{R: 0.13, G: 0.16, B: 0.22, A: 1})
			tv.rt.FillRectangle(bg, tv.brush)
			d2d.SetBrushColor(tv.brush, d2d.Color{R: 0.91, G: 0.91, B: 0.91, A: 1})
			textRect := d2d.RectF{Left: left, Top: top + 1, Right: left + width, Bottom: top + ch}
			tv.rt.DrawText(string(runes), tv.textFormat, textRect, tv.brush)
			underline := d2d.RectF{Left: left, Top: top + ch - 2, Right: left + width, Bottom: top + ch - 1}
			tv.rt.FillRectangle(underline, tv.brush)
		}

		// Cursor
		if preedit == "" && cur.Visible != 0 && cur.X < cc && cur.Y < rr {
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
