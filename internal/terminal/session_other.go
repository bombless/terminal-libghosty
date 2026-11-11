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

type RenderImage struct {
	ImageID     uint32
	PlacementID uint32
	Layer       uint32
	Z           int32
	Col         int32
	Row         int32
	GridCols    uint32
	GridRows    uint32
	OffsetX     uint32
	OffsetY     uint32
	Width       uint32
	Height      uint32
	SrcX        uint32
	SrcY        uint32
	SrcWidth    uint32
	SrcHeight   uint32
	ImgWidth    uint32
	ImgHeight   uint32
	Pixels      []byte
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
func (*Session) ResizeWithCellSize(uint16, uint16, uint32, uint32) error { return fmt.Errorf("unsupported platform") }
func (*Session) Snapshot() (uint16, uint16, []Cell, []byte, Cursor, error) { return 0, 0, nil, nil, Cursor{}, fmt.Errorf("unsupported platform") }
func (*Session) SnapshotWithImages() (uint16, uint16, []Cell, []byte, Cursor, []RenderImage, error) { return 0, 0, nil, nil, Cursor{}, nil, fmt.Errorf("unsupported platform") }
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
