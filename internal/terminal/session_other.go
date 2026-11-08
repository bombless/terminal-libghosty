//go:build !windows

package terminal

import (
	"fmt"
	"github.com/bombless/terminal-libghosty/internal/ssh"
)

type Session struct{}

func StartSSH(ssh.Options, uint16, uint16) (*Session, error) {
	return nil, fmt.Errorf("the native ConPTY/Ghostty session is only available on Windows")
}
func Start(string, []string, uint16, uint16) (*Session, error) {
	return nil, fmt.Errorf("the native ConPTY/Ghostty session is only available on Windows")
}
func (*Session) Write([]byte) error              { return fmt.Errorf("unsupported platform") }
func (*Session) Resize(uint16, uint16) error     { return fmt.Errorf("unsupported platform") }
func (*Session) PollExit() (bool, uint32, error) { return false, 0, fmt.Errorf("unsupported platform") }
func (*Session) SnapshotText() (string, error)   { return "", fmt.Errorf("unsupported platform") }
func (*Session) Close()                          {}
