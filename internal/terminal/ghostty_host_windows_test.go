//go:build windows

package terminal

import (
	"bytes"
	"testing"
)

func TestGhosttyHostMatchesNativeBridgeBehavior(t *testing.T) {
	var reply []byte
	host, err := newGhosttyHost(func(data []byte) {
		reply = append(reply, data...)
	})
	if err != nil {
		t.Fatalf("create Ghostty host: %v", err)
	}
	defer host.destroy()

	input := []byte("bridge UTF-8: \xe4\xb8\xad\xe6\x96\x87 \x1b[1;32mgreen\x1b[0m\r\n\x1b[5n")
	if err := host.feedOutput(input); err != nil {
		t.Fatalf("feed Ghostty output: %v", err)
	}
	if !bytes.Equal(reply, []byte("\x1b[0n")) {
		t.Fatalf("Ghostty PTY callback = %q, want %q", reply, "\x1b[0n")
	}

	cols, rows, cells, text, cursor, err := host.snapshot()
	if err != nil {
		t.Fatalf("Ghostty snapshot: %v", err)
	}
	if cols != 80 || rows != 24 || len(cells) != int(cols)*int(rows) || len(text) == 0 {
		t.Fatalf("unexpected initial snapshot: %dx%d cells=%d text=%d", cols, rows, len(cells), len(text))
	}
	if cursor.Blinking == 0 {
		t.Fatal("default Ghostty cursor should blink, matching native host")
	}

	foundChinese, foundBoldGreen, foundGreenColor := false, false, false
	for _, cell := range cells {
		end := uint64(cell.TextOffset) + uint64(cell.TextLength)
		if end > uint64(len(text)) {
			t.Fatalf("cell text range [%d,%d) exceeds %d bytes", cell.TextOffset, end, len(text))
		}
		grapheme := text[cell.TextOffset:uint32(end)]
		if bytes.Equal(grapheme, []byte("中")) {
			foundChinese = true
		}
		if bytes.Equal(grapheme, []byte("g")) {
			if cell.StyleFlags&1 != 0 {
				foundBoldGreen = true
			}
			if cell.Foreground.G > cell.Foreground.R && cell.Foreground.G > cell.Foreground.B {
				foundGreenColor = true
			}
		}
	}
	if !foundChinese {
		t.Fatal("Ghostty snapshot lost UTF-8 grapheme content")
	}
	if !foundBoldGreen {
		t.Fatal("Ghostty snapshot lost ANSI bold style")
	}
	if !foundGreenColor {
		t.Fatal("Ghostty snapshot lost resolved ANSI foreground color")
	}

	if err := host.resize(120, 40, 9, 18); err != nil {
		t.Fatalf("resize Ghostty host: %v", err)
	}
	cols, rows, cells, _, _, err = host.snapshot()
	if err != nil {
		t.Fatalf("snapshot after resize: %v", err)
	}
	if cols != 120 || rows != 40 || len(cells) != 4800 {
		t.Fatalf("snapshot did not follow resize: %dx%d cells=%d", cols, rows, len(cells))
	}
	if err := host.resize(0, 40, 9, 18); err == nil {
		t.Fatal("zero-column resize should be rejected")
	}
	if err := host.feedOutput(nil); err != nil {
		t.Fatalf("empty Ghostty feed should be accepted: %v", err)
	}
}

func TestGhosttyHostSelectionMarksCells(t *testing.T) {
	host, err := newGhosttyHost(func([]byte) {})
	if err != nil {
		t.Fatalf("create Ghostty host: %v", err)
	}
	defer host.destroy()

	if err := host.feedOutput([]byte("hello world")); err != nil {
		t.Fatalf("feed Ghostty output: %v", err)
	}
	if err := host.setSelection(0, 0, 4, 0); err != nil {
		t.Fatalf("install selection: %v", err)
	}

	assertSelected := func(want ...bool) {
		t.Helper()
		_, _, cells, _, _, err := host.snapshot()
		if err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		for i, selected := range want {
			if got := cells[i].Selected != 0; got != selected {
				t.Fatalf("cell %d selected = %v, want %v", i, got, selected)
			}
		}
	}

	assertSelected(true, true, true, true, true, false)

	if err := host.clearSelection(); err != nil {
		t.Fatalf("clear selection: %v", err)
	}
	assertSelected(false, false, false, false, false, false)

	if err := host.setSelection(0, 0, 0, 99); err == nil {
		t.Fatal("out-of-bounds selection should be rejected")
	}
}

func TestGhosttyHostScrollingAndModes(t *testing.T) {
	host, err := newGhosttyHost(func([]byte) {})
	if err != nil {
		t.Fatalf("create Ghostty host: %v", err)
	}
	defer host.destroy()

	if err := host.resize(80, 24, 9, 18); err != nil {
		t.Fatalf("resize host: %v", err)
	}

	sb, err := host.scrollbar()
	if err != nil {
		t.Fatalf("initial scrollbar: %v", err)
	}
	if sb.Len != 24 || sb.Total != 24 || sb.Offset != 0 {
		t.Fatalf("unexpected initial scrollbar: %+v", sb)
	}

	// Feed 60 lines of text so scrollback is populated.
	var buf bytes.Buffer
	for i := 1; i <= 60; i++ {
		buf.WriteString("line ")
		buf.WriteString(string(rune('0' + (i % 10))))
		buf.WriteString(" content\r\n")
	}
	if err := host.feedOutput(buf.Bytes()); err != nil {
		t.Fatalf("feed output: %v", err)
	}

	sb, err = host.scrollbar()
	if err != nil {
		t.Fatalf("scrollbar after lines: %v", err)
	}
	if sb.Total <= sb.Len {
		t.Fatalf("expected scrollback total > len, got Total=%d, Len=%d", sb.Total, sb.Len)
	}
	bottomOffset := sb.Total - sb.Len
	if sb.Offset != bottomOffset {
		t.Fatalf("expected initial viewport at bottom offset=%d, got %d", bottomOffset, sb.Offset)
	}

	// Scroll up 5 lines
	if err := host.scrollViewportDelta(-5); err != nil {
		t.Fatalf("scroll delta -5: %v", err)
	}
	sb, err = host.scrollbar()
	if err != nil {
		t.Fatalf("scrollbar after scroll: %v", err)
	}
	if sb.Offset != bottomOffset-5 {
		t.Fatalf("expected offset %d, got %d", bottomOffset-5, sb.Offset)
	}

	// Scroll to top
	if err := host.scrollViewportTop(); err != nil {
		t.Fatalf("scroll top: %v", err)
	}
	sb, err = host.scrollbar()
	if err != nil {
		t.Fatalf("scrollbar after scroll top: %v", err)
	}
	if sb.Offset != 0 {
		t.Fatalf("expected offset 0 at top, got %d", sb.Offset)
	}

	// Scroll to bottom
	if err := host.scrollViewportBottom(); err != nil {
		t.Fatalf("scroll bottom: %v", err)
	}
	sb, err = host.scrollbar()
	if err != nil {
		t.Fatalf("scrollbar after scroll bottom: %v", err)
	}
	if sb.Offset != bottomOffset {
		t.Fatalf("expected offset %d at bottom, got %d", bottomOffset, sb.Offset)
	}

	// Test alternate screen detection
	isAlt, err := host.isAlternateScreen()
	if err != nil || isAlt {
		t.Fatalf("expected normal screen initially, got isAlt=%v err=%v", isAlt, err)
	}
	if err := host.feedOutput([]byte("\x1b[?1049h")); err != nil {
		t.Fatalf("enter alternate screen: %v", err)
	}
	isAlt, err = host.isAlternateScreen()
	if err != nil || !isAlt {
		t.Fatalf("expected alternate screen after sequence, got isAlt=%v err=%v", isAlt, err)
	}
	if err := host.feedOutput([]byte("\x1b[?1049l")); err != nil {
		t.Fatalf("leave alternate screen: %v", err)
	}
	isAlt, err = host.isAlternateScreen()
	if err != nil || isAlt {
		t.Fatalf("expected normal screen after leaving, got isAlt=%v err=%v", isAlt, err)
	}

	// Test mouse tracking detection
	isMouse, err := host.isMouseTracking()
	if err != nil || isMouse {
		t.Fatalf("expected mouse tracking false initially, got isMouse=%v err=%v", isMouse, err)
	}
	if err := host.feedOutput([]byte("\x1b[?1000h")); err != nil {
		t.Fatalf("enable mouse tracking: %v", err)
	}
	isMouse, err = host.isMouseTracking()
	if err != nil || !isMouse {
		t.Fatalf("expected mouse tracking true after sequence, got isMouse=%v err=%v", isMouse, err)
	}
	if err := host.feedOutput([]byte("\x1b[?1000l")); err != nil {
		t.Fatalf("disable mouse tracking: %v", err)
	}
	isMouse, err = host.isMouseTracking()
	if err != nil || isMouse {
		t.Fatalf("expected mouse tracking false after leaving, got isMouse=%v err=%v", isMouse, err)
	}
}
