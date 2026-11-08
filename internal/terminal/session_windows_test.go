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
