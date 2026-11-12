//go:build windows

package terminal

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestZZConPTYFlagHRESULT(t *testing.T) {
	var inRead, inWrite, outRead, outWrite syscall.Handle
	security := securityAttributes{Length: uint32(unsafe.Sizeof(securityAttributes{})), InheritHandle: 1}
	mustPipe := func(read, write *syscall.Handle) {
		ok, _, _ := procCreatePipe.Call(uintptr(unsafe.Pointer(read)), uintptr(unsafe.Pointer(write)), uintptr(unsafe.Pointer(&security)), 0)
		if ok == 0 { t.Fatalf("CreatePipe failed: %v", syscall.GetLastError()) }
	}
	mustPipe(&inRead, &inWrite)
	defer procCloseHandle.Call(uintptr(inRead)); defer procCloseHandle.Call(uintptr(inWrite))
	mustPipe(&outRead, &outWrite)
	defer procCloseHandle.Call(uintptr(outRead)); defer procCloseHandle.Call(uintptr(outWrite))
	coord := uintptr(uint32(80) | uint32(24)<<16)
	var pseudo syscall.Handle
	hr, _, _ := procCreatePseudoConsole.Call(coord, uintptr(inRead), uintptr(outWrite), 0x8, uintptr(unsafe.Pointer(&pseudo)))
	t.Logf("CreatePseudoConsole flags=0x8 HRESULT=0x%08x", uint32(hr))
	if int32(hr) < 0 {
		hr, _, _ = procCreatePseudoConsole.Call(coord, uintptr(inRead), uintptr(outWrite), 0, uintptr(unsafe.Pointer(&pseudo)))
		t.Logf("fallback flags=0 HRESULT=0x%08x", uint32(hr))
	} else {
		t.Log("fallback=false final_flags=0x8")
	}
	if int32(hr) >= 0 && pseudo != 0 { procClosePseudoConsole.Call(uintptr(pseudo)) }
	if int32(hr) < 0 { t.Fatalf("both ConPTY creation attempts failed: HRESULT=0x%08x", uint32(hr)) }
}

func TestZZConPTYAPCWriteAPCMatrix(t *testing.T) {
	sizes := []int{4194304}
	modes := []string{"text", "raw-single", "raw-4096"}
	for _, size := range sizes {
		for _, mode := range modes {
			for repeat := 1; repeat <= 1; repeat++ {
				expected := []byte("\x1b_Ga=T,f=100;" + strings.Repeat("A", size) + "\x1b\\")
				var script strings.Builder
				fmt.Fprintf(&script, "$n=%d; $body='A' * $n; $p=[Text.Encoding]::ASCII.GetBytes(([char]27 + '_Ga=T,f=100;' + $body + [char]27 + [char]92)); ", size)
				script.WriteString("$o=[Console]::OpenStandardOutput(); ")
				if mode == "text" {
					script.WriteString("[Console]::Out.Write('START'); [Console]::Out.Write([Text.Encoding]::ASCII.GetString($p)); [Console]::Out.Write('END'); [Console]::Out.Flush(); ")
				} else {
					script.WriteString("$m=[Text.Encoding]::ASCII.GetBytes('START'); $e=[Text.Encoding]::ASCII.GetBytes('END'); $o.Write($m,0,$m.Length); ")
					if mode == "raw-single" {
						script.WriteString("$o.Write($p,0,$p.Length); ")
					} else {
						script.WriteString("for($i=0;$i -lt $p.Length;$i+=4096){$c=[Math]::Min(4096,$p.Length-$i);$o.Write($p,$i,$c)}; ")
					}
					script.WriteString("$o.Write($e,0,$e.Length); $o.Flush(); ")
				}
				script.WriteString("Start-Sleep -Milliseconds 1500; exit 0")
				var mu sync.Mutex
				var output bytes.Buffer
				s, err := newConPTY("pwsh.exe", []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script.String()}, 80, 24, func(p []byte) {
					mu.Lock(); output.Write(p); mu.Unlock()
				}, nil)
				if err != nil { t.Fatalf("size=%d mode=%s repeat=%d create: %v", size, mode, repeat, err) }
				deadline := time.Now().Add(12*time.Second)
				exited := false
				for time.Now().Before(deadline) {
					ok, code, pollErr := s.PollExit()
					if pollErr != nil { s.Close(); t.Fatalf("size=%d mode=%s poll: %v", size, mode, pollErr) }
					if ok { exited = true; if code != 0 { s.Close(); t.Fatalf("size=%d mode=%s exit=%d", size, mode, code) }; break }
					time.Sleep(10*time.Millisecond)
				}
				if !exited { s.Close(); t.Fatalf("size=%d mode=%s timed out", size, mode) }
				s.Close()
				mu.Lock(); captured := append([]byte(nil), output.Bytes()...); mu.Unlock()
				start := bytes.Index(captured, []byte("START"))
				end := -1
				if start >= 0 { rel := bytes.Index(captured[start+5:], []byte("END")); if rel >= 0 { end = start+5+rel } }
				var got []byte
				if start >= 0 && end >= 0 { got = captured[start+5:end] }
				wantHash, gotHash := sha256.Sum256(expected), sha256.Sum256(got)
				first := -1
				for i := 0; i < len(expected) && i < len(got); i++ { if expected[i] != got[i] { first = i; break } }
				if first < 0 && len(expected) != len(got) { first = min(len(expected),len(got)) }
				t.Logf("size=%d mode=%s run=%d expected=%d captured=%d expected_sha256=%x captured_sha256=%x first_mismatch=%d total_stdout=%d", size, mode, repeat, len(expected), len(got), wantHash, gotHash, first, len(captured))
			}
		}
	}
}
