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
