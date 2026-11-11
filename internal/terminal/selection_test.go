package terminal

import (
	"testing"
)

func TestExtractSelectedTextEmpty(t *testing.T) {
	if got := ExtractSelectedText(5, 2, nil, nil); got != "" {
		t.Fatalf("expected empty string for nil cells, got %q", got)
	}

	cells := make([]Cell, 10)
	if got := ExtractSelectedText(5, 2, cells, []byte("hello")); got != "" {
		t.Fatalf("expected empty string when no cells are selected, got %q", got)
	}
}

func TestExtractSelectedTextSingleLine(t *testing.T) {
	// 5 cols, 2 rows
	// row 0: "hello"
	// row 1: "world"
	text := []byte("helloworld")
	cells := make([]Cell, 10)
	for i := 0; i < 10; i++ {
		cells[i] = Cell{
			TextOffset: uint32(i),
			TextLength: 1,
		}
	}

	// Select "ell" in row 0 (x=1..3)
	cells[1].Selected = 1
	cells[2].Selected = 1
	cells[3].Selected = 1

	got := ExtractSelectedText(5, 2, cells, text)
	want := "ell"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExtractSelectedTextMultiLineWithTrailingSpaces(t *testing.T) {
	// 10 cols, 2 rows
	// row 0: "hi        " (selected cols 0..9)
	// row 1: "there     " (selected cols 0..4)
	text := []byte("hithere")
	cells := make([]Cell, 20)

	// row 0
	cells[0] = Cell{TextOffset: 0, TextLength: 1, Selected: 1} // 'h'
	cells[1] = Cell{TextOffset: 1, TextLength: 1, Selected: 1} // 'i'
	for x := 2; x < 10; x++ {
		cells[x] = Cell{TextOffset: 0, TextLength: 0, Selected: 1} // spaces
	}

	// row 1
	for x := 0; x < 5; x++ {
		cells[10+x] = Cell{TextOffset: uint32(2 + x), TextLength: 1, Selected: 1} // "there"
	}

	got := ExtractSelectedText(10, 2, cells, text)
	want := "hi\r\nthere"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExtractSelectedTextWideChars(t *testing.T) {
	// 6 cols, 1 row
	// "中文" -> '中' (len 3, width 2), spacer, '文' (len 3, width 2), spacer, ' ', ' '
	text := []byte("中文")
	cells := make([]Cell, 6)
	cells[0] = Cell{TextOffset: 0, TextLength: 3, Selected: 1}
	cells[1] = Cell{TextOffset: 0, TextLength: 0, Selected: 1} // spacer
	cells[2] = Cell{TextOffset: 3, TextLength: 3, Selected: 1}
	cells[3] = Cell{TextOffset: 0, TextLength: 0, Selected: 1} // spacer
	cells[4] = Cell{TextOffset: 0, TextLength: 0, Selected: 0}
	cells[5] = Cell{TextOffset: 0, TextLength: 0, Selected: 0}

	got := ExtractSelectedText(6, 1, cells, text)
	want := "中文"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExtractSelectedTextPreserveSpacesOnlySelection(t *testing.T) {
	// 5 cols, 1 row: all empty spaces, user explicitly selected 3 spaces
	cells := make([]Cell, 5)
	cells[1].Selected = 1
	cells[2].Selected = 1
	cells[3].Selected = 1

	got := ExtractSelectedText(5, 1, cells, nil)
	want := "   "
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
