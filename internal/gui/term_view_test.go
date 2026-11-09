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
