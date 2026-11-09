//go:build windows

package terminal

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

const (
	ghosttySuccess             = 0
	ghosttyOutOfSpace          = -3
	ghosttyNoValue             = -4
	ghosttyDataCols            = 1
	ghosttyDataRows            = 2
	ghosttyDataRowIter         = 4
	ghosttyDataCursor          = 18
	ghosttyDataColors          = 19
	ghosttyRowDataCells        = 3
	ghosttyCellDataStyle       = 2
	ghosttyCellDataBG          = 5
	ghosttyCellDataFG          = 6
	ghosttyCellDataSel         = 7
	ghosttyCellDataUTF8        = 9
	ghosttyTerminalUser        = 1
	ghosttyTerminalWrite       = 2
	ghosttyTerminalCursorBlink = 24
)

type ghosttyRGB struct{ R, G, B uint8 }
type ghosttyBuffer struct{ Ptr, Cap, Len uintptr }
type ghosttyStyleColor struct {
	Tag   uint8
	Value [3]byte
}
type ghosttyStyle struct {
	FG, BG, Underline ghosttyStyleColor
	Flags, Reserved   uint16
}
type ghosttyCursor struct {
	Size                                       uintptr
	ViewportHas                                uint8
	Pad                                        uint8
	ViewportX, ViewportY                       uint16
	WideTail, Visible, Blinking, PasswordInput uint8
	VisualStyle                                int32
}
type ghosttyColors struct {
	Size                           uintptr
	Background, Foreground, Cursor ghosttyRGB
	CursorHas                      uint8
	Palette                        [256]ghosttyRGB
}

type ghosttyProc struct {
	call func(args ...uintptr) (uintptr, uintptr, error)
}

func (p *ghosttyProc) Call(args ...uintptr) (uintptr, uintptr, error) {
	return p.call(args...)
}

type ghosttyAPI struct {
	terminalNew, terminalFree, terminalSet, terminalWrite, terminalResize *ghosttyProc
	renderNew, renderFree, renderUpdate, renderGet, renderClean           *ghosttyProc
	rowIteratorNew, rowIteratorFree, rowIteratorNext, rowGet              *ghosttyProc
	rowCellsNew, rowCellsFree, rowCellsNext, rowCellsGet                  *ghosttyProc
}

var (
	ghosttyLoadOnce      sync.Once
	ghosttyLoadErr       error
	ghostty              *ghosttyAPI
	ghosttyCallbackID    atomic.Uint64
	ghosttyCallbacks     sync.Map
	ghosttyWriteCallback = syscall.NewCallback(ghosttyWritePTYCallback)
)

func ghosttyWritePTYCallback(_ uintptr, userdata uintptr, data uintptr, length uintptr) (ret uintptr) {
	defer func() { _ = recover() }()
	v, ok := ghosttyCallbacks.Load(userdata)
	if !ok || length == 0 || data == 0 || length > uintptr(int(^uint(0)>>1)) {
		return 0
	}
	fn, ok := v.(func([]byte))
	if !ok {
		return 0
	}
	fn(append([]byte(nil), unsafe.Slice((*byte)(unsafe.Pointer(data)), int(length))...))
	return 0
}

func procBool(p *ghosttyProc, args ...uintptr) bool {
	r, _, _ := p.Call(args...)
	return r != 0
}

type ghosttyHost struct {
	api                                 *ghosttyAPI
	terminal, render, rowIter, rowCells uintptr
	callbackID                          uintptr
}

func newGhosttyHost(writeInput func([]byte)) (*ghosttyHost, error) {
	a, err := loadGhosttyAPI()
	if err != nil {
		return nil, err
	}
	h := &ghosttyHost{api: a}
	fail := func(err error) (*ghosttyHost, error) { h.destroy(); return nil, err }
	r, _, _ := a.renderNew.Call(0, uintptr(unsafe.Pointer(&h.render)))
	if int32(r) != ghosttySuccess || h.render == 0 {
		return fail(fmt.Errorf("ghostty_render_state_new failed: %d", int32(r)))
	}
	r, _, _ = a.rowIteratorNew.Call(0, uintptr(unsafe.Pointer(&h.rowIter)))
	if int32(r) != ghosttySuccess || h.rowIter == 0 {
		return fail(fmt.Errorf("ghostty_render_state_row_iterator_new failed: %d", int32(r)))
	}
	r, _, _ = a.rowCellsNew.Call(0, uintptr(unsafe.Pointer(&h.rowCells)))
	if int32(r) != ghosttySuccess || h.rowCells == 0 {
		return fail(fmt.Errorf("ghostty_render_state_row_cells_new failed: %d", int32(r)))
	}
	r, _, _ = a.terminalNew.Call(0, uintptr(unsafe.Pointer(&h.terminal)), 80, 24)
	if int32(r) != ghosttySuccess || h.terminal == 0 {
		return fail(fmt.Errorf("ghostty_terminal_new failed: %d", int32(r)))
	}
	h.callbackID = uintptr(ghosttyCallbackID.Add(1))
	ghosttyCallbacks.Store(h.callbackID, writeInput)
	r, _, _ = a.terminalSet.Call(h.terminal, ghosttyTerminalUser, h.callbackID)
	if int32(r) != ghosttySuccess {
		return fail(fmt.Errorf("set Ghostty userdata failed: %d", int32(r)))
	}
	r, _, _ = a.terminalSet.Call(h.terminal, ghosttyTerminalWrite, ghosttyWriteCallback)
	if int32(r) != ghosttySuccess {
		return fail(fmt.Errorf("install Ghostty write callback failed: %d", int32(r)))
	}
	blink := uint8(1)
	r, _, _ = a.terminalSet.Call(h.terminal, ghosttyTerminalCursorBlink, uintptr(unsafe.Pointer(&blink)))
	runtime.KeepAlive(&blink)
	if int32(r) != ghosttySuccess {
		return fail(fmt.Errorf("set Ghostty default cursor blink failed: %d", int32(r)))
	}
	return h, nil
}

func (h *ghosttyHost) feedOutput(data []byte) error {
	if h == nil || h.terminal == 0 {
		return fmt.Errorf("Ghostty host is closed")
	}
	if len(data) > 0 {
		h.api.terminalWrite.Call(h.terminal, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
		runtime.KeepAlive(data)
	}
	return nil
}
func (h *ghosttyHost) resize(cols, rows uint16, cellWidth, cellHeight uint32) error {
	if h == nil || h.terminal == 0 || cols == 0 || rows == 0 || cellWidth == 0 || cellHeight == 0 {
		return fmt.Errorf("invalid Ghostty resize dimensions")
	}
	r, _, _ := h.api.terminalResize.Call(h.terminal, uintptr(cols), uintptr(rows), uintptr(cellWidth), uintptr(cellHeight))
	if int32(r) != ghosttySuccess {
		return fmt.Errorf("ghostty_terminal_resize failed: %d", int32(r))
	}
	return nil
}

func (h *ghosttyHost) snapshot() (uint16, uint16, []Cell, []byte, Cursor, error) {
	if h == nil || h.terminal == 0 || h.render == 0 {
		return 0, 0, nil, nil, Cursor{}, fmt.Errorf("Ghostty host is closed")
	}
	a := h.api
	r, _, _ := a.renderUpdate.Call(h.render, h.terminal)
	if int32(r) != ghosttySuccess {
		return 0, 0, nil, nil, Cursor{}, fmt.Errorf("Ghostty render update failed: %d", int32(r))
	}
	var cols, rows uint16
	r, _, _ = a.renderGet.Call(h.render, ghosttyDataCols, uintptr(unsafe.Pointer(&cols)))
	if int32(r) != ghosttySuccess {
		return 0, 0, nil, nil, Cursor{}, fmt.Errorf("Ghostty columns query failed: %d", int32(r))
	}
	r, _, _ = a.renderGet.Call(h.render, ghosttyDataRows, uintptr(unsafe.Pointer(&rows)))
	if int32(r) != ghosttySuccess || cols == 0 || rows == 0 {
		return 0, 0, nil, nil, Cursor{}, fmt.Errorf("Ghostty rows query failed: %d", int32(r))
	}
	cur := ghosttyCursor{Size: unsafe.Sizeof(ghosttyCursor{})}
	r, _, _ = a.renderGet.Call(h.render, ghosttyDataCursor, uintptr(unsafe.Pointer(&cur)))
	if int32(r) != ghosttySuccess {
		return 0, 0, nil, nil, Cursor{}, fmt.Errorf("Ghostty cursor query failed: %d", int32(r))
	}
	colors := ghosttyColors{Size: unsafe.Sizeof(ghosttyColors{})}
	r, _, _ = a.renderGet.Call(h.render, ghosttyDataColors, uintptr(unsafe.Pointer(&colors)))
	if int32(r) != ghosttySuccess {
		return 0, 0, nil, nil, Cursor{}, fmt.Errorf("Ghostty colors query failed: %d", int32(r))
	}
	r, _, _ = a.renderGet.Call(h.render, ghosttyDataRowIter, uintptr(unsafe.Pointer(&h.rowIter)))
	if int32(r) != ghosttySuccess {
		return 0, 0, nil, nil, Cursor{}, fmt.Errorf("Ghostty row iterator query failed: %d", int32(r))
	}

	count := int(cols) * int(rows)
	lengths := make([]uint32, 0, count)
	var total uint64
	for procBool(a.rowIteratorNext, h.rowIter) {
		r, _, _ = a.rowGet.Call(h.rowIter, ghosttyRowDataCells, uintptr(unsafe.Pointer(&h.rowCells)))
		if int32(r) != ghosttySuccess {
			return 0, 0, nil, nil, Cursor{}, fmt.Errorf("Ghostty row cells query failed: %d", int32(r))
		}
		for procBool(a.rowCellsNext, h.rowCells) {
			buf := ghosttyBuffer{}
			r, _, _ = a.rowCellsGet.Call(h.rowCells, ghosttyCellDataUTF8, uintptr(unsafe.Pointer(&buf)))
			if int32(r) != ghosttyOutOfSpace && int32(r) != ghosttySuccess {
				return 0, 0, nil, nil, Cursor{}, fmt.Errorf("Ghostty grapheme sizing failed: %d", int32(r))
			}
			if buf.Len > uintptr(^uint32(0)) || total+uint64(buf.Len) > uint64(^uint32(0)) {
				return 0, 0, nil, nil, Cursor{}, fmt.Errorf("Ghostty snapshot exceeds 32-bit text offsets")
			}
			lengths = append(lengths, uint32(buf.Len))
			total += uint64(buf.Len)
		}
	}
	if len(lengths) != count {
		return 0, 0, nil, nil, Cursor{}, fmt.Errorf("Ghostty returned %d cells for %dx%d viewport", len(lengths), cols, rows)
	}
	cells := make([]Cell, count)
	text := make([]byte, int(total))
	r, _, _ = a.renderGet.Call(h.render, ghosttyDataRowIter, uintptr(unsafe.Pointer(&h.rowIter)))
	if int32(r) != ghosttySuccess {
		return 0, 0, nil, nil, Cursor{}, fmt.Errorf("reset Ghostty row iterator failed: %d", int32(r))
	}
	var offset uint32
	index := 0
	for procBool(a.rowIteratorNext, h.rowIter) {
		r, _, _ = a.rowGet.Call(h.rowIter, ghosttyRowDataCells, uintptr(unsafe.Pointer(&h.rowCells)))
		if int32(r) != ghosttySuccess {
			return 0, 0, nil, nil, Cursor{}, fmt.Errorf("Ghostty row cells query failed: %d", int32(r))
		}
		for procBool(a.rowCellsNext, h.rowCells) {
			c := &cells[index]
			c.TextOffset = offset
			c.TextLength = lengths[index]
			var style ghosttyStyle
			r, _, _ = a.rowCellsGet.Call(h.rowCells, ghosttyCellDataStyle, uintptr(unsafe.Pointer(&style)))
			if int32(r) != ghosttySuccess {
				return 0, 0, nil, nil, Cursor{}, fmt.Errorf("Ghostty cell style query failed: %d", int32(r))
			}
			c.StyleFlags = uint32(style.Flags)
			c.Foreground = RGB{colors.Foreground.R, colors.Foreground.G, colors.Foreground.B}
			c.Background = RGB{colors.Background.R, colors.Background.G, colors.Background.B}
			var color ghosttyRGB
			r, _, _ = a.rowCellsGet.Call(h.rowCells, ghosttyCellDataFG, uintptr(unsafe.Pointer(&color)))
			if int32(r) == ghosttySuccess {
				c.Foreground = RGB{color.R, color.G, color.B}
			} else if int32(r) != ghosttyNoValue {
				return 0, 0, nil, nil, Cursor{}, fmt.Errorf("Ghostty foreground query failed: %d", int32(r))
			}
			r, _, _ = a.rowCellsGet.Call(h.rowCells, ghosttyCellDataBG, uintptr(unsafe.Pointer(&color)))
			if int32(r) == ghosttySuccess {
				c.Background = RGB{color.R, color.G, color.B}
			} else if int32(r) != ghosttyNoValue {
				return 0, 0, nil, nil, Cursor{}, fmt.Errorf("Ghostty background query failed: %d", int32(r))
			}
			var selected uint8
			r, _, _ = a.rowCellsGet.Call(h.rowCells, ghosttyCellDataSel, uintptr(unsafe.Pointer(&selected)))
			if int32(r) != ghosttySuccess {
				return 0, 0, nil, nil, Cursor{}, fmt.Errorf("Ghostty selection query failed: %d", int32(r))
			}
			c.Selected = selected
			if c.TextLength > 0 {
				buf := ghosttyBuffer{Ptr: uintptr(unsafe.Pointer(&text[int(offset)])), Cap: uintptr(c.TextLength)}
				r, _, _ = a.rowCellsGet.Call(h.rowCells, ghosttyCellDataUTF8, uintptr(unsafe.Pointer(&buf)))
				if int32(r) != ghosttySuccess || buf.Len != uintptr(c.TextLength) {
					return 0, 0, nil, nil, Cursor{}, fmt.Errorf("Ghostty grapheme copy failed: result=%d length=%d", int32(r), buf.Len)
				}
			}
			offset += c.TextLength
			index++
		}
	}
	r, _, _ = a.renderClean.Call(h.render)
	if int32(r) != ghosttySuccess {
		return 0, 0, nil, nil, Cursor{}, fmt.Errorf("Ghostty render clean failed: %d", int32(r))
	}
	cursor := Cursor{X: cur.ViewportX, Y: cur.ViewportY, Visible: cur.Visible, Blinking: cur.Blinking, VisualStyle: uint8(cur.VisualStyle)}
	if cur.ViewportHas == 0 {
		cursor.X = 0
		cursor.Y = 0
		cursor.Visible = 0
	}
	runtime.KeepAlive(cells)
	runtime.KeepAlive(text)
	return cols, rows, cells, text, cursor, nil
}

func (h *ghosttyHost) destroy() {
	if h == nil {
		return
	}
	if h.callbackID != 0 {
		ghosttyCallbacks.Delete(h.callbackID)
		h.callbackID = 0
	}
	if h.api != nil {
		if h.rowCells != 0 {
			h.api.rowCellsFree.Call(h.rowCells)
			h.rowCells = 0
		}
		if h.rowIter != 0 {
			h.api.rowIteratorFree.Call(h.rowIter)
			h.rowIter = 0
		}
		if h.render != 0 {
			h.api.renderFree.Call(h.render)
			h.render = 0
		}
		if h.terminal != 0 {
			h.api.terminalFree.Call(h.terminal)
			h.terminal = 0
		}
	}
}
