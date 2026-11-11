//go:build windows

package terminal

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWindowsCommandLineQuoting(t *testing.T) {
	tests := []struct {
		arg, want string
	}{
		{"plain", "plain"},
		{"", "\"\""},
		{"with space", "\"with space\""},
		{"a\"b", "\"a\\\"b\""},
		{"tail\\", "tail\\"},
		{"tail with space\\", "\"tail with space\\\\\""},
	}
	for _, tt := range tests {
		if got := quoteWindowsArg(tt.arg); got != tt.want {
			t.Errorf("quoteWindowsArg(%q) = %q, want %q", tt.arg, got, tt.want)
		}
	}
	if _, err := buildWindowsCommandLine("cmd.exe", []string{"bad\x00arg"}); err == nil {
		t.Fatal("expected NUL argument to be rejected")
	}
}

func TestConPTYProcessIOResizeAndClose(t *testing.T) {
	var mu sync.Mutex
	var output strings.Builder
	exitCh := make(chan uint32, 1)
	s, err := newConPTY("cmd.exe", []string{"/d", "/q"}, 80, 24, func(p []byte) {
		mu.Lock()
		output.Write(p)
		mu.Unlock()
	}, func(code uint32) {
		exitCh <- code
	})
	if err != nil {
		t.Fatalf("create ConPTY: %v", err)
	}

	if err := s.Resize(100, 30); err != nil {
		s.Close()
		t.Fatalf("resize ConPTY: %v", err)
	}
	if err := s.Resize(0, 30); err == nil {
		s.Close()
		t.Fatal("zero-sized resize should fail")
	}
	if err := s.Write([]byte("echo conpty-go-roundtrip-ok\r\nexit\r\n")); err != nil {
		s.Close()
		t.Fatalf("write to ConPTY: %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	var code uint32
	for time.Now().Before(deadline) {
		exited, exitCode, pollErr := s.PollExit()
		if pollErr != nil {
			s.Close()
			t.Fatalf("poll ConPTY child: %v", pollErr)
		}
		if exited {
			code = exitCode
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if code != 0 {
		mu.Lock()
		partial := output.String()
		mu.Unlock()
		s.Close()
		t.Fatalf("ConPTY child exit code = %d (output=%q)", code, partial)
	}
	var got string
	drainDeadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(drainDeadline) {
		mu.Lock()
		got = output.String()
		mu.Unlock()
		if strings.Contains(got, "conpty-go-roundtrip-ok") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(got, "conpty-go-roundtrip-ok") {
		s.Close()
		t.Fatalf("ConPTY output missing marker: %q", got)
	}
	s.Close()
	select {
	case callbackCode := <-exitCh:
		if callbackCode != 0 {
			t.Fatalf("exit callback code = %d, want 0", callbackCode)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("C-compatible exit callback was not delivered after output EOF")
	}
	s.Close()
	if err := s.Write([]byte("after-close")); err == nil {
		t.Fatal("write after Close should fail")
	}
}

func TestConPTYLongOutput(t *testing.T) {
	var mu sync.Mutex
	var output strings.Builder
	command := "for /L %i in (1,1,5000) do @echo line-%i"
	s, err := newConPTY("cmd.exe", []string{"/d", "/q", "/c", command}, 100, 30, func(p []byte) {
		mu.Lock()
		output.Write(p)
		mu.Unlock()
	}, nil)
	if err != nil {
		t.Fatalf("create ConPTY: %v", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	processExited := false
	for time.Now().Before(deadline) {
		exited, code, pollErr := s.PollExit()
		if pollErr != nil {
			s.Close()
			t.Fatalf("poll long-output child: %v", pollErr)
		}
		if exited {
			if code != 0 {
				s.Close()
				t.Fatalf("long-output child exit code = %d", code)
			}
			processExited = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !processExited {
		s.Close()
		t.Fatal("long-output child timed out")
	}
	s.Close() // closes HPCON, drains final bytes, and joins the output reader
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		got := output.String()
		mu.Unlock()
		if strings.Contains(got, "line-5000") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	got := output.String()
	mu.Unlock()
	t.Fatalf("long output was truncated before final marker (bytes=%d, tail=%q)", len(got), got[max(0, len(got)-160):])
}

func TestConPTYConcurrentClose(t *testing.T) {
	s, err := newConPTY("cmd.exe", []string{"/d", "/q"}, 80, 24, nil, nil)
	if err != nil {
		t.Fatalf("create ConPTY: %v", err)
	}
	done := make(chan struct{}, 2)
	go func() { s.Close(); done <- struct{}{} }()
	go func() { s.Close(); done <- struct{}{} }()
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent Close did not complete")
		}
	}
}

func TestConPTYCreateFailureCleansUp(t *testing.T) {
	if _, err := newConPTY("C:\\definitely-not-present\\missing.exe", nil, 80, 24, nil, nil); err == nil {
		t.Fatal("expected missing executable to fail")
	}
}

func TestConPTYKittyPassthrough(t *testing.T) {
	var mu sync.Mutex
	var output strings.Builder

	pwshCmd := "[Console]::Out.Write([char]27 + '_Ga=T,f=100;PWSH7_TEST' + [char]27 + [char]92 + 'PWSH7_DONE'); exit 0"
	s, err := newConPTY("pwsh.exe", []string{"-NoLogo", "-NoProfile", "-Command", pwshCmd}, 80, 24, func(p []byte) {
		mu.Lock()
		output.Write(p)
		mu.Unlock()
	}, nil)
	if err != nil {
		t.Fatalf("create ConPTY: %v", err)
	}
	defer s.Close()

	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		got := output.String()
		mu.Unlock()
		if strings.Contains(got, "PWSH7_DONE") {
			if !strings.Contains(got, "\x1b_Ga=T,f=100;PWSH7_TEST\x1b\\") {
				t.Fatalf("Kitty APC sequence stripped! ConPTY output: %q", got)
			}
			t.Logf("Success! Found Kitty APC sequence in output: %q", got)
			for time.Now().Before(deadline) {
				if exited, _, _ := s.PollExit(); exited {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	mu.Lock()
	got := output.String()
	mu.Unlock()
	t.Fatalf("Timeout waiting for child output. Current output: %q", got)
}
