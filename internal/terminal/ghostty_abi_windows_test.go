//go:build windows

package terminal

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

// TestGhosttyOfficialDLLABI is the first-stage, pure-Go ABI spike. It calls
// only the official upstream C ABI; it deliberately does not use cgo.
func TestGhosttyOfficialDLLABI(t *testing.T) {
	dllPath := os.Getenv("GHOSTTY_VT_DLL")
	if dllPath == "" {
		_, file, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("cannot locate package source directory")
		}
		dllPath = filepath.Join(filepath.Dir(file), "..", "..", "build", "native-local", "ghostty-vt.dll")
	}
	dllPath, err := filepath.Abs(dllPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dllPath); err != nil {
		t.Skipf("official Ghostty DLL unavailable at %q: %v", dllPath, err)
	}

	dll, err := syscall.LoadDLL(dllPath)
	if err != nil {
		t.Fatalf("load official Ghostty DLL %q: %v", dllPath, err)
	}
	defer dll.Release()

	proc := func(name string) *syscall.Proc {
		p, err := dll.FindProc(name)
		if err != nil {
			t.Fatalf("find Ghostty export %s: %v", name, err)
		}
		return p
	}

	version, _, _ := proc("ghostty_version").Call()
	if version == 0 {
		t.Fatal("ghostty_version returned NULL")
	}
	versionBytes := make([]byte, 0, 64)
	for i := uintptr(0); ; i++ {
		b := *(*byte)(unsafe.Pointer(version + i))
		if b == 0 {
			break
		}
		versionBytes = append(versionBytes, b)
		if len(versionBytes) > 256 {
			t.Fatal("ghostty_version returned an unterminated/implausibly long string")
		}
	}
	versionString := string(versionBytes)
	t.Logf("loaded official Ghostty VT library version %q from %s", versionString, dllPath)
	if strings.TrimSpace(versionString) == "" {
		t.Fatal("ghostty_version returned an empty version")
	}

	var terminal uintptr
	result, _, _ := proc("ghostty_terminal_new").Call(
		0, uintptr(unsafe.Pointer(&terminal)), 80, 24,
	)
	if result != 0 || terminal == 0 {
		t.Fatalf("ghostty_terminal_new(80x24) result=%d terminal=%#x", result, terminal)
	}
	defer proc("ghostty_terminal_free").Call(terminal)

	// Windows syscall.NewCallback supplies a native callback trampoline. Keep
	// all callback state in Go memory and copy the borrowed upstream bytes
	// before returning to Ghostty.
	const userdata = uintptr(0x1234)
	result, _, _ = proc("ghostty_terminal_set").Call(terminal, 1, userdata)
	if result != 0 {
		t.Fatalf("set Ghostty userdata result=%d", result)
	}
	var ptyReply []byte
	writePTY := syscall.NewCallback(func(gotTerminal, gotUserdata, data, length uintptr) uintptr {
		if gotTerminal != terminal || gotUserdata != userdata {
			t.Errorf("write_pty callback handles mismatch: terminal=%#x userdata=%#x", gotTerminal, gotUserdata)
			return 0
		}
		if length != 0 {
			ptyReply = append(ptyReply, unsafe.Slice((*byte)(unsafe.Pointer(data)), int(length))...)
		}
		return 0
	})
	result, _, _ = proc("ghostty_terminal_set").Call(terminal, 2, writePTY)
	if result != 0 {
		t.Fatalf("install Go write_pty callback result=%d", result)
	}

	input := []byte("pure Go ABI smoke: hello\r\n\x1b[31mred\x1b[0m\x1b[5n")
	proc("ghostty_terminal_vt_write").Call(
		terminal, uintptr(unsafe.Pointer(&input[0])), uintptr(len(input)),
	)
	runtime.KeepAlive(input)
	if !bytes.Equal(ptyReply, []byte("\x1b[0n")) {
		t.Fatalf("Ghostty DSR reply = %q, want %q", ptyReply, "\x1b[0n")
	}

	result, _, _ = proc("ghostty_terminal_resize").Call(terminal, 100, 30, 8, 16)
	if result != 0 {
		t.Fatalf("ghostty_terminal_resize(100x30) result=%d", result)
	}

	var renderState uintptr
	result, _, _ = proc("ghostty_render_state_new").Call(
		0, uintptr(unsafe.Pointer(&renderState)),
	)
	if result != 0 || renderState == 0 {
		t.Fatalf("ghostty_render_state_new result=%d state=%#x", result, renderState)
	}
	defer proc("ghostty_render_state_free").Call(renderState)

	result, _, _ = proc("ghostty_render_state_update").Call(renderState, terminal)
	if result != 0 {
		t.Fatalf("ghostty_render_state_update result=%d", result)
	}
}
