//go:build windows

package terminal

import (
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/bombless/terminal-libghosty/internal/ssh"
)

func TestGhosttySnapshotABIShape(t *testing.T) {
	if got := unsafe.Sizeof(Cell{}); got != 24 {
		t.Fatalf("native tlg_terminal_cell ABI size = %d, want 24", got)
	}
	if got := unsafe.Sizeof(Cursor{}); got != 10 {
		t.Fatalf("native tlg_terminal_cursor ABI size = %d, want 10", got)
	}
}

func TestStartSSHRejectsInvalidTarget(t *testing.T) {
	if _, err := StartSSH(ssh.Options{Target: "bad target"}, 80, 24); err == nil {
		t.Fatal("expected invalid SSH target to be rejected before native process creation")
	}
}

func TestNativeSSHVersionE2E(t *testing.T) {
	s, err := Start("ssh.exe", []string{"-V"}, 100, 30)
	if err != nil {
		t.Fatalf("start ssh through ConPTY/Ghostty: %v", err)
	}
	defer s.Close()

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		exited, code, err := s.PollExit()
		if err != nil {
			t.Fatal(err)
		}
		if exited {
			if code != 0 {
				t.Fatalf("ssh -V exit code = %d", code)
			}
			time.Sleep(100 * time.Millisecond) // drain final ConPTY output
			screen, err := s.SnapshotText()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(screen, "OpenSSH") {
				t.Fatalf("Ghostty snapshot did not contain SSH version: %q", screen)
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("ssh -V did not exit before timeout")
}

func TestStartSSHBuildArgsReachConPTY(t *testing.T) {
	s, err := StartSSH(ssh.Options{Target: "probe@127.0.0.1:65534"}, 100, 30)
	if err != nil {
		t.Fatalf("start validated SSH target through native session: %v", err)
	}
	defer s.Close()

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		exited, code, err := s.PollExit()
		if err != nil {
			t.Fatal(err)
		}
		if exited {
			if code == 0 {
				t.Fatal("SSH to a closed loopback port unexpectedly succeeded")
			}
			time.Sleep(100 * time.Millisecond)
			screen, err := s.SnapshotText()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(strings.ToLower(screen), "connection refused") {
				t.Fatalf("SSH argv/output did not reach ConPTY/Ghostty: %q", screen)
			}
			cols, rows, cells, text, _, err := s.Snapshot()
			if err != nil {
				t.Fatalf("read Ghostty styled cell snapshot: %v", err)
			}
			if cols != 100 || rows != 30 || len(cells) != int(cols)*int(rows) {
				t.Fatalf("unexpected Ghostty cell grid: %dx%d (%d cells)", cols, rows, len(cells))
			}
			if len(text) == 0 {
				t.Fatal("styled Ghostty cell snapshot returned no grapheme bytes")
			}
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("SSH connection attempt did not exit before timeout")
}

func TestPowerShellSessionScrolling(t *testing.T) {
	s, err := StartPowerShell(80, 24)
	if err != nil {
		t.Skipf("StartPowerShell skipped: %v", err)
	}
	defer s.Close()

	// Wait for PowerShell prompt to appear
	deadline := time.Now().Add(10 * time.Second)
	promptFound := false
	for time.Now().Before(deadline) {
		text, err := s.SnapshotText()
		if err == nil && (strings.Contains(text, "PS ") || strings.Contains(text, ">")) {
			promptFound = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !promptFound {
		t.Fatal("PowerShell prompt not found within timeout")
	}

	// Output 50 lines in PowerShell
	cmd := "1..50 | ForEach-Object { \"PWSHTEST_LINE_$_\" }\r\n"
	if err := s.Write([]byte(cmd)); err != nil {
		t.Fatalf("write command to PowerShell: %v", err)
	}

	// Wait for scrollback to accumulate
	scrollbackFound := false
	deadline = time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		sb, err := s.Scrollbar()
		if err == nil && sb.Total > sb.Len {
			scrollbackFound = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !scrollbackFound {
		t.Fatal("PowerShell output did not generate scrollback")
	}

	sb, err := s.Scrollbar()
	if err != nil {
		t.Fatalf("get scrollbar: %v", err)
	}
	bottomOffset := sb.Total - sb.Len
	if sb.Offset != bottomOffset {
		t.Fatalf("expected viewport initially at bottom offset %d, got %d", bottomOffset, sb.Offset)
	}

	// Scroll up by 5 lines
	scrollUpLines := uint64(5)
	if err := s.ScrollViewportDelta(-int(scrollUpLines)); err != nil {
		t.Fatalf("ScrollViewportDelta(-5): %v", err)
	}
	sb, err = s.Scrollbar()
	if err != nil {
		t.Fatalf("get scrollbar after scroll up: %v", err)
	}
	expectedOffset := uint64(0)
	if bottomOffset > scrollUpLines {
		expectedOffset = bottomOffset - scrollUpLines
	}
	if sb.Offset != expectedOffset {
		t.Fatalf("expected offset %d, got %d (total=%d len=%d)", expectedOffset, sb.Offset, sb.Total, sb.Len)
	}

	// Scroll to top
	if err := s.ScrollViewportTop(); err != nil {
		t.Fatalf("ScrollViewportTop: %v", err)
	}
	sb, err = s.Scrollbar()
	if err != nil {
		t.Fatalf("get scrollbar after scroll top: %v", err)
	}
	if sb.Offset != 0 {
		t.Fatalf("expected offset 0 at top, got %d", sb.Offset)
	}

	// Verify snapshot text at top contains earlier lines
	text, err := s.SnapshotText()
	if err != nil {
		t.Fatalf("SnapshotText at top: %v", err)
	}
	if !strings.Contains(text, "PWSHTEST_LINE_") {
		t.Fatalf("scrolled snapshot does not contain test line: %q", text)
	}

	// Scroll to bottom
	if err := s.ScrollViewportBottom(); err != nil {
		t.Fatalf("ScrollViewportBottom: %v", err)
	}
	sb, err = s.Scrollbar()
	if err != nil {
		t.Fatalf("get scrollbar after scroll bottom: %v", err)
	}
	if sb.Offset != sb.Total-sb.Len {
		t.Fatalf("expected offset %d at bottom, got %d", sb.Total-sb.Len, sb.Offset)
	}
}
