//go:build windows

package terminal

import (
	"testing"
	"time"
)

func TestSessionRepeatedStartAndConcurrentClose(t *testing.T) {
	for i := 0; i < 5; i++ {
		s, err := Start("cmd.exe", []string{"/d", "/q"}, 80, 24)
		if err != nil {
			t.Fatalf("iteration %d: start Session: %v", i, err)
		}
		done := make(chan struct{}, 2)
		go func() { s.Close(); done <- struct{}{} }()
		go func() { s.Close(); done <- struct{}{} }()
		for n := 0; n < 2; n++ {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatalf("iteration %d: concurrent Session.Close did not complete", i)
			}
		}
		if err := s.Write([]byte("after-close")); err == nil {
			t.Fatalf("iteration %d: Write after Close should fail", i)
		}
	}
}
