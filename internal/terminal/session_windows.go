//go:build windows

package terminal

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bombless/terminal-libghosty/internal/ssh"
)

// Session composes the Go ConPTY process manager with the official Ghostty VT
// library. vtMu serializes every terminal mutation and snapshot operation.
type Session struct {
	mu        sync.Mutex
	vtMu      sync.Mutex
	conpty    *conPTY
	ghostty   *ghosttyHost
	closing   bool
	closeDone chan struct{}
}

type RGB struct{ R, G, B uint8 }
type Cell struct {
	TextOffset, TextLength, StyleFlags uint32
	Foreground, Background             RGB
	Selected                           uint8
	Reserved                           [3]byte
}
type Cursor struct {
	X, Y                           uint16
	Visible, Blinking, VisualStyle uint8
	Reserved                       [3]byte
}

func StartSSH(options ssh.Options, cols, rows uint16) (*Session, error) {
	args, err := ssh.BuildArgs(options)
	if err != nil {
		return nil, err
	}
	return Start("ssh.exe", args, cols, rows)
}

// StartPowerShell starts a local PowerShell 7 session (pwsh.exe) in ConPTY.
func StartPowerShell(cols, rows uint16) (*Session, error) {
	return Start("pwsh.exe", []string{"-NoLogo"}, cols, rows)
}

func Start(executable string, args []string, cols, rows uint16) (*Session, error) {
	if executable == "" || cols == 0 || rows == 0 {
		return nil, fmt.Errorf("invalid terminal executable or dimensions")
	}
	if !filepath.IsAbs(executable) && !strings.ContainsAny(executable, "/\\") {
		resolved, err := exec.LookPath(executable)
		if err != nil {
			return nil, fmt.Errorf("resolve executable %q: %w", executable, err)
		}
		executable = resolved
	}
	s := &Session{closeDone: make(chan struct{})}
	inputReady := make(chan struct{})
	host, err := newGhosttyHost(func(data []byte) {
		// The ConPTY reader can deliver a query immediately after process
		// creation. Wait until the returned process handle is published.
		<-inputReady
		s.mu.Lock()
		p := s.conpty
		closing := s.closing
		s.mu.Unlock()
		if p != nil && !closing {
			_ = p.Write(data)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("create Ghostty VT host: %w", err)
	}
	s.ghostty = host
	if err := host.resize(cols, rows, 9, 18); err != nil {
		host.destroy()
		return nil, fmt.Errorf("initialize Ghostty VT size: %w", err)
	}
	p, err := newConPTY(executable, args, cols, rows, func(data []byte) {
		s.mu.Lock()
		closing := s.closing
		s.mu.Unlock()
		if closing {
			return
		}
		s.vtMu.Lock()
		defer s.vtMu.Unlock()
		s.mu.Lock()
		host := s.ghostty
		closing = s.closing
		s.mu.Unlock()
		if !closing && host != nil {
			_ = host.feedOutput(data)
		}
	}, nil)
	if err != nil {
		host.destroy()
		return nil, fmt.Errorf("create ConPTY process: %w", err)
	}
	s.mu.Lock()
	s.conpty = p
	s.mu.Unlock()
	close(inputReady)
	return s, nil
}

func (s *Session) Write(data []byte) error {
	if s == nil {
		return fmt.Errorf("terminal session is closed")
	}
	s.mu.Lock()
	if s.closing || s.conpty == nil {
		s.mu.Unlock()
		return fmt.Errorf("terminal session is closed")
	}
	p := s.conpty
	s.mu.Unlock()
	return p.Write(data)
}

func (s *Session) Resize(cols, rows uint16) error {
	if s == nil || cols == 0 || rows == 0 {
		return fmt.Errorf("invalid terminal resize dimensions")
	}
	s.vtMu.Lock()
	defer s.vtMu.Unlock()
	s.mu.Lock()
	if s.closing || s.conpty == nil || s.ghostty == nil {
		s.mu.Unlock()
		return fmt.Errorf("terminal session is closed")
	}
	p, host := s.conpty, s.ghostty
	s.mu.Unlock()
	if err := p.Resize(cols, rows); err != nil {
		return fmt.Errorf("resize ConPTY: %w", err)
	}
	if err := host.resize(cols, rows, 9, 18); err != nil {
		return fmt.Errorf("resize Ghostty VT: %w", err)
	}
	return nil
}

func (s *Session) PollExit() (bool, uint32, error) {
	if s == nil {
		return false, 0, fmt.Errorf("terminal session is closed")
	}
	s.mu.Lock()
	if s.closing || s.conpty == nil {
		s.mu.Unlock()
		return false, 0, fmt.Errorf("terminal session is closed")
	}
	p := s.conpty
	s.mu.Unlock()
	return p.PollExit()
}

// Select installs a linear selection between two active-area cells.
func (s *Session) Select(startCol, startRow, endCol, endRow uint16) error {
	if s == nil {
		return fmt.Errorf("terminal session is closed")
	}
	s.vtMu.Lock()
	defer s.vtMu.Unlock()
	s.mu.Lock()
	if s.closing || s.ghostty == nil {
		s.mu.Unlock()
		return fmt.Errorf("terminal session is closed")
	}
	host := s.ghostty
	s.mu.Unlock()
	return host.setSelection(startCol, startRow, endCol, endRow)
}

func (s *Session) ClearSelection() error {
	if s == nil {
		return fmt.Errorf("terminal session is closed")
	}
	s.vtMu.Lock()
	defer s.vtMu.Unlock()
	s.mu.Lock()
	if s.closing || s.ghostty == nil {
		s.mu.Unlock()
		return fmt.Errorf("terminal session is closed")
	}
	host := s.ghostty
	s.mu.Unlock()
	return host.clearSelection()
}

func (s *Session) ScrollViewportDelta(delta int) error {
	if s == nil {
		return fmt.Errorf("terminal session is closed")
	}
	s.vtMu.Lock()
	defer s.vtMu.Unlock()
	s.mu.Lock()
	if s.closing || s.ghostty == nil {
		s.mu.Unlock()
		return fmt.Errorf("terminal session is closed")
	}
	host := s.ghostty
	s.mu.Unlock()
	return host.scrollViewportDelta(delta)
}

func (s *Session) ScrollViewportTop() error {
	if s == nil {
		return fmt.Errorf("terminal session is closed")
	}
	s.vtMu.Lock()
	defer s.vtMu.Unlock()
	s.mu.Lock()
	if s.closing || s.ghostty == nil {
		s.mu.Unlock()
		return fmt.Errorf("terminal session is closed")
	}
	host := s.ghostty
	s.mu.Unlock()
	return host.scrollViewportTop()
}

func (s *Session) ScrollViewportBottom() error {
	if s == nil {
		return fmt.Errorf("terminal session is closed")
	}
	s.vtMu.Lock()
	defer s.vtMu.Unlock()
	s.mu.Lock()
	if s.closing || s.ghostty == nil {
		s.mu.Unlock()
		return fmt.Errorf("terminal session is closed")
	}
	host := s.ghostty
	s.mu.Unlock()
	return host.scrollViewportBottom()
}

func (s *Session) ScrollViewportRow(row uint64) error {
	if s == nil {
		return fmt.Errorf("terminal session is closed")
	}
	s.vtMu.Lock()
	defer s.vtMu.Unlock()
	s.mu.Lock()
	if s.closing || s.ghostty == nil {
		s.mu.Unlock()
		return fmt.Errorf("terminal session is closed")
	}
	host := s.ghostty
	s.mu.Unlock()
	return host.scrollViewportRow(row)
}

func (s *Session) Scrollbar() (Scrollbar, error) {
	if s == nil {
		return Scrollbar{}, fmt.Errorf("terminal session is closed")
	}
	s.vtMu.Lock()
	defer s.vtMu.Unlock()
	s.mu.Lock()
	if s.closing || s.ghostty == nil {
		s.mu.Unlock()
		return Scrollbar{}, fmt.Errorf("terminal session is closed")
	}
	host := s.ghostty
	s.mu.Unlock()
	return host.scrollbar()
}

func (s *Session) IsAlternateScreen() (bool, error) {
	if s == nil {
		return false, fmt.Errorf("terminal session is closed")
	}
	s.vtMu.Lock()
	defer s.vtMu.Unlock()
	s.mu.Lock()
	if s.closing || s.ghostty == nil {
		s.mu.Unlock()
		return false, fmt.Errorf("terminal session is closed")
	}
	host := s.ghostty
	s.mu.Unlock()
	return host.isAlternateScreen()
}

func (s *Session) IsMouseTracking() (bool, error) {
	if s == nil {
		return false, fmt.Errorf("terminal session is closed")
	}
	s.vtMu.Lock()
	defer s.vtMu.Unlock()
	s.mu.Lock()
	if s.closing || s.ghostty == nil {
		s.mu.Unlock()
		return false, fmt.Errorf("terminal session is closed")
	}
	host := s.ghostty
	s.mu.Unlock()
	return host.isMouseTracking()
}

// Snapshot returns a caller-owned copy of Ghostty's rendered cell grid.
func (s *Session) Snapshot() (uint16, uint16, []Cell, []byte, Cursor, error) {
	if s == nil {
		return 0, 0, nil, nil, Cursor{}, fmt.Errorf("terminal session is closed")
	}
	s.vtMu.Lock()
	defer s.vtMu.Unlock()
	s.mu.Lock()
	if s.closing || s.ghostty == nil {
		s.mu.Unlock()
		return 0, 0, nil, nil, Cursor{}, fmt.Errorf("terminal session is closed")
	}
	host := s.ghostty
	s.mu.Unlock()
	return host.snapshot()
}

// SnapshotText returns the cell grid as UTF-8 text with CRLF row endings.
// Empty cells are represented by one ASCII space, matching the native API.
func (s *Session) SnapshotText() (string, error) {
	cols, rows, cells, text, _, err := s.Snapshot()
	if err != nil {
		return "", err
	}
	var out strings.Builder
	out.Grow(len(text) + len(cells) + int(rows)*2)
	index := 0
	for y := uint16(0); y < rows; y++ {
		for x := uint16(0); x < cols; x++ {
			c := cells[index]
			index++
			if c.TextLength == 0 {
				out.WriteByte(' ')
				continue
			}
			end := uint64(c.TextOffset) + uint64(c.TextLength)
			if end > uint64(len(text)) {
				return "", fmt.Errorf("Ghostty cell text range exceeds snapshot buffer")
			}
			out.Write(text[c.TextOffset:uint32(end)])
		}
		out.WriteString("\r\n")
	}
	return out.String(), nil
}

// SelectedText returns the text currently selected in the terminal viewport,
// or an empty string if no cells are selected.
func (s *Session) SelectedText() (string, error) {
	cols, rows, cells, text, _, err := s.Snapshot()
	if err != nil {
		return "", err
	}
	return ExtractSelectedText(cols, rows, cells, text), nil
}

func (s *Session) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.closing {
		done := s.closeDone
		s.mu.Unlock()
		<-done
		return
	}
	s.closing = true
	p := s.conpty
	s.mu.Unlock()

	// Never hold vtMu while joining the ConPTY reader: that reader may be
	// waiting to finish an output callback that needs vtMu.
	if p != nil {
		p.Close()
	}
	s.vtMu.Lock()
	s.mu.Lock()
	host := s.ghostty
	s.ghostty = nil
	s.conpty = nil
	s.mu.Unlock()
	if host != nil {
		host.destroy()
	}
	s.vtMu.Unlock()
	close(s.closeDone)
}
