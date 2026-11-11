//go:build windows

package gui

import (
	"testing"

	"github.com/bombless/terminal-libghosty/internal/terminal"
)

func TestPaintForCellPredictionStyle(t *testing.T) {
	// PSReadLine's default InlinePredictionColor is SGR 97;2;3: bright white,
	// faint and italic. The faint flag must reduce the glyph opacity so the
	// suggestion is visually distinct from typed input.
	cell := terminal.Cell{
		Foreground: terminal.RGB{R: 0xEA, G: 0xEA, B: 0xEA},
		StyleFlags: styleFaint | styleItalic,
	}
	paint := paintForCell(cell)
	if paint.textAlpha != 0.5 {
		t.Fatalf("faint text alpha = %v, want 0.5", paint.textAlpha)
	}
	if !paint.italic {
		t.Fatal("italic style not detected")
	}
	if paint.fillBg {
		t.Fatal("default cell background must not be filled")
	}
}

func TestPaintForCellInverse(t *testing.T) {
	cell := terminal.Cell{
		Foreground: terminal.RGB{R: 0xEA, G: 0xEA, B: 0xEA},
		StyleFlags: styleInverse,
	}
	paint := paintForCell(cell)
	if !paint.fillBg || paint.bg != (rgbColor(cell.Foreground)) {
		t.Fatalf("inverse background = %+v fill=%v, want cell foreground", paint.bg, paint.fillBg)
	}
	if paint.fg != defaultBackground {
		t.Fatalf("inverse foreground = %+v, want default background", paint.fg)
	}
}

func TestPaintForCellInvisible(t *testing.T) {
	cell := terminal.Cell{
		Foreground: terminal.RGB{R: 0xFF, G: 0xFF, B: 0xFF},
		StyleFlags: styleInvisible,
	}
	if !paintForCell(cell).hideText {
		t.Fatal("invisible style does not hide text")
	}
}

func TestTerminalViewCellFromPixels(t *testing.T) {
	tv := &TerminalView{
		cellWidth:  10.0,
		cellHeight: 20.0,
		cols:       80,
		rows:       24,
	}

	x, y := tv.cellFromPixels(0, 0)
	if x != 0 || y != 0 {
		t.Fatalf("cell at 0,0: got %d,%d, want 0,0", x, y)
	}

	x, y = tv.cellFromPixels(25, 45)
	if x != 2 || y != 2 {
		t.Fatalf("cell at 25,45: got %d,%d, want 2,2", x, y)
	}

	// Clamping out-of-bounds
	x, y = tv.cellFromPixels(-10, -5)
	if x != 0 || y != 0 {
		t.Fatalf("cell at -10,-5: got %d,%d, want 0,0", x, y)
	}
	x, y = tv.cellFromPixels(1000, 1000)
	if x != 79 || y != 23 {
		t.Fatalf("cell at 1000,1000: got %d,%d, want 79,23", x, y)
	}
}

func TestTerminalViewHitScrollbar(t *testing.T) {
	tv := &TerminalView{
		cellWidth:      10.0,
		cellHeight:     20.0,
		cols:           80,
		rows:           24,
		frameCols:      80,
		frameRows:      24,
		frameScrollbar: terminal.Scrollbar{Total: 100, Len: 24, Offset: 76},
	}

	// Total width is 80 * 10 = 800. Scrollbar is rightmost 10px (790 - 800).
	lParamInside := uintptr(795) | (uintptr(100) << 16)
	if !tv.hitScrollbar(lParamInside) {
		t.Fatal("expected hitScrollbar to be true for x=795")
	}

	lParamOutside := uintptr(780) | (uintptr(100) << 16)
	if tv.hitScrollbar(lParamOutside) {
		t.Fatal("expected hitScrollbar to be false for x=780")
	}

	// When no scrollback (Total <= Len)
	tv.frameScrollbar = terminal.Scrollbar{Total: 24, Len: 24, Offset: 0}
	if tv.hitScrollbar(lParamInside) {
		t.Fatal("expected hitScrollbar to be false when no scrollback exists")
	}
}
