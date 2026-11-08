//go:build windows

package terminal

import (
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestGhosttyQueryChildProcess(t *testing.T) {
	if os.Getenv("TERMINAL_LIBGHOSTY_VT_QUERY_CHILD") != "1" {
		return
	}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getStdHandle := kernel32.NewProc("GetStdHandle")
	getConsoleMode := kernel32.NewProc("GetConsoleMode")
	setConsoleMode := kernel32.NewProc("SetConsoleMode")
	input, _, _ := getStdHandle.Call(^uintptr(9)) // STD_INPUT_HANDLE (-10)
	var mode uint32
	ok, _, _ := getConsoleMode.Call(input, uintptr(unsafe.Pointer(&mode)))
	if ok == 0 {
		t.Fatalf("GetConsoleMode(stdin): %v", syscall.GetLastError())
	}
	mode = (mode &^ (0x0001 | 0x0002 | 0x0004)) | 0x0200 // raw VT input
	ok, _, _ = setConsoleMode.Call(input, uintptr(mode))
	if ok == 0 {
		t.Fatalf("SetConsoleMode(stdin): %v", syscall.GetLastError())
	}
	if _, err := os.Stdout.Write([]byte("\x1b[5n")); err != nil {
		t.Fatalf("write DSR query: %v", err)
	}
	response := make([]byte, 64)
	var received strings.Builder
	for received.Len() < 63 {
		n, err := os.Stdin.Read(response)
		if err != nil {
			t.Fatalf("read DSR response: %v", err)
		}
		received.Write(response[:n])
		if strings.Contains(received.String(), "n") {
			break
		}
	}
	if !strings.Contains(received.String(), "\x1b[0n") {
		t.Fatalf("unexpected DSR response: %q", received.String())
	}
	if _, err := os.Stdout.Write([]byte("ghostty-reply-ok\r\n")); err != nil {
		t.Fatalf("write success marker: %v", err)
	}
}

func TestSessionGhosttyDSRReplyRoundTrip(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TERMINAL_LIBGHOSTY_VT_QUERY_CHILD", "1")
	s, err := Start(executable, []string{"-test.run=^TestGhosttyQueryChildProcess$"}, 100, 30)
	if err != nil {
		t.Fatalf("start DSR child through Go session: %v", err)
	}
	defer s.Close()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		exited, code, pollErr := s.PollExit()
		if pollErr != nil {
			t.Fatal(pollErr)
		}
		if exited {
			if code != 0 {
				screen, _ := s.SnapshotText()
				t.Fatalf("DSR child exit code=%d screen=%q", code, screen)
			}
			drainDeadline := time.Now().Add(500 * time.Millisecond)
			for time.Now().Before(drainDeadline) {
				screen, snapshotErr := s.SnapshotText()
				if snapshotErr == nil && strings.Contains(screen, "ghostty-reply-ok") {
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
			screen, _ := s.SnapshotText()
			t.Fatalf("Ghostty DSR reply did not complete child round-trip: %q", screen)
		}
		time.Sleep(20 * time.Millisecond)
	}
	screen, _ := s.SnapshotText()
	t.Fatalf("DSR child timed out waiting for Ghostty reply: %q", screen)
}
