# Native Windows terminal compatibility baseline

The original C implementation in this directory is the known-working reference
for the Go migration. Keep its source, headers, CMake targets and tests runnable:
they are useful for checking that the Go replacement preserves the same
ConPTY, Ghostty, SSH, callback, snapshot and resize behavior.

The production application now uses:

- `internal/terminal/conpty_windows.go` for ConPTY and child-process lifecycle;
- `internal/terminal/ghostty_host_windows.go` for the Ghostty VT adapter;
- `internal/terminal/session_windows.go` to compose the Go components;
- the official `ghostty-vt.dll` for VT parsing and render-state generation.

The Go application is built with `CGO_ENABLED=0` and does not load
`terminal_libghosty_session.dll). The original C session DLL remains useful
only for running the native reference program and its CTest suite.

## Upstream boundary

- Upstream: https://github.com/ghostty-org/ghostty
- The current native C build uses the local Ghostty source configured through
  `FETCHCONTENT_SOURCE_DIR_GHOSTTY`.
- The CMake Git pin is build provenance, not a blocker for Go compatibility.
  The working native C program, its headers, and its passing tests define the
  behavior the Go implementation must match.
- The official Ghostty GUI and internal `ghostty.h` embedder API are not used.
  The supported VT engine boundary is the public `libghostty-vt` C ABI.

## Native C reference implementation

The C implementation is intentionally retained:

- `windows_conpty.h/.c`: Windows ConPTY pipes, process creation, argv quoting,
  resize, output callback, process polling and close.
- `ghostty_host.h/.c`: Ghostty terminal/render-state ownership, VT feed,
  PTY write callback, UTF-8 cell snapshots, colors, style flags and cursor.
- `ssh_terminal.h/.c`: the reference composition of ConPTY and Ghostty,
  including DSR replies and SSH process output.

These files are not used by the production Go Session. Do not delete them as
part of the Go migration; use them to compare behavior when changing the Go
adapter or the Ghostty runtime DLL.

## Go compatibility coverage

The Go tests port the behavior of the original C tests:

- `TestGhosttyHostMatchesNativeBridgeBehavior`: DSR callback, UTF-8 graphemes,
  ANSI bold/green color, cursor blink, snapshot and resize.
- `TestSessionGhosttyDSRReplyRoundTrip`: child process asks CSI 5n and receives
  CSI 0n from Ghostty through ConPTY stdin.
- `TestNativeSSHVersionE2E`: the real `ssh.exe -V` output is visible through
  the Ghostty text snapshot.
- `TestStartSSHBuildArgsReachConPTY`: SSH closed-port error output and styled
  cell snapshot.
- ConPTY tests cover quoting, process I/O, long output, exit polling, output-EOF
  callback timing, failed startup and concurrent close.

The original CTest suite remains the comparison baseline.

## Build and smoke test the C reference

From a Visual Studio Developer PowerShell or another Windows C compiler shell:

```powershell
cmake -S native -B build/native -A x64
cmake --build build/native --config Release
ctest --test-dir build/native -C Release --output-on-failure
```

Requirements: CMake 3.19+, Zig 0.17.0 on PATH, Git, and a Windows C compiler.
The ConPTY module requires Windows 10 version 1809 (build 17763) or newer.

The CMake project builds the original C reference program and tests, including
`terminal_libghosty_session.dll`. This is separate from the production Go
application and is retained for regression comparison.

## Build the production Go application

From the repository root:

```powershell
$env:CGO_ENABLED = '0'
go test ./...
go build -ldflags="-H=windowsgui" -o dist/terminal-libghosty.exe ./cmd/terminal-libghosty
go build -ldflags="-H=windowsgui" -o dist/terminal-host.exe ./cmd/terminal-host
cmake -E copy_if_different build/native-local/ghostty-vt.dll dist/ghostty-vt.dll
```

The official `ghostty-vt.dll` must be beside the executable, or supplied via
`GHOSTTY_VT_DLL` / `TERMINAL_LIBGHOSTY_NATIVE_DIR`. No project-owned session
DLL, cgo, or C compiler is required to build the Go application.
