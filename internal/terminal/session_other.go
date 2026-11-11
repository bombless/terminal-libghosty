//go:build !windows

package terminal

import (
	"fmt"
	"github.com/bombless/terminal-libghosty/internal/ssh"
)

type Scrollbar struct {
	Total  uint64
	Offset uint64
	Len    uint64
}

type Session struct{}

func StartSSH(ssh.Options, uint16, uint16) (*Session, error) {
	return nil, fmt.Errorf("the native ConPTY/Ghostty session is only available on Windows")
}
func StartPowerShell(uint16, uint16) (*Session, error) {
	return nil, fmt.Errorf("the native ConPTY/Ghostty session is only available on Windows")
}
func Start(string, []string, uint16, uint16) (*Session, error) {
	return nil, fmt.Errorf("the native ConPTY/Ghostty session is only available on Windows")
}
func (*Session) Write([]byte) error                       { return fmt.Errorf("unsupported platform") }
func (*Session) Resize(uint16, uint16) error              { return fmt.Errorf("unsupported platform") }
func (*Session) PollExit() (bool, uint32, error)          { return false, 0, fmt.Errorf("unsupported platform") }
func (*Session) Select(uint16, uint16, uint16, uint16) error { return fmt.Errorf("unsupported platform") }
func (*Session) ClearSelection() error                       { return fmt.Errorf("unsupported platform") }
func (*Session) SelectedText() (string, error)               { return "", fmt.Errorf("unsupported platform") }
func (*Session) SnapshotText() (string, error)            { return "", fmt.Errorf("unsupported platform") }
func (*Session) ScrollViewportDelta(int) error            { return fmt.Errorf("unsupported platform") }
func (*Session) ScrollViewportTop() error                 { return fmt.Errorf("unsupported platform") }
func (*Session) ScrollViewportBottom() error              { return fmt.Errorf("unsupported platform") }
func (*Session) ScrollViewportRow(uint64) error           { return fmt.Errorf("unsupported platform") }
func (*Session) Scrollbar() (Scrollbar, error)            { return Scrollbar{}, fmt.Errorf("unsupported platform") }
func (*Session) IsAlternateScreen() (bool, error)         { return false, fmt.Errorf("unsupported platform") }
func (*Session) IsMouseTracking() (bool, error)           { return false, fmt.Errorf("unsupported platform") }
func (*Session) Close()                                   {}
