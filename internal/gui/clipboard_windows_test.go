//go:build windows

package gui

import (
	"testing"
)

func TestClipboardSetAndGet(t *testing.T) {
	testStr := "TestClipboard_你好Ghostty_12345"
	if err := SetClipboardText(0, testStr); err != nil {
		t.Fatalf("SetClipboardText: %v", err)
	}

	got, err := GetClipboardText(0)
	if err != nil {
		t.Fatalf("GetClipboardText: %v", err)
	}
	if got != testStr {
		t.Fatalf("clipboard got %q, want %q", got, testStr)
	}
}
