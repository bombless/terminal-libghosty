# Implementation status

## Existing application

- SSH profile manager GUI and profile persistence.
- SSH target validation and argument construction.
- Strict host-key checking and forced PTY requested for SSH sessions.
- The profile manager's Connect action creates an embedded Win32 child-window host in the existing right-hand panel and calls `internal/terminal.StartSSH()`; it no longer launches `wt.exe`.

## Ghostty VT integration

- The project uses the public `libghostty-vt` C ABI from the official `ghostty-vt.dll`; the upstream internal `ghostty.h` embedder API is not used.
- The original `native/ghostty_host.c` is the compatibility reference for terminal creation, VT feed, WRITE_PTY callback, render-state iteration, UTF-8 graphemes, resolved colors, style flags, selection, cursor and resize.
- `internal/terminal/ghostty_host_windows.go` now implements that host behavior in Go using Windows DLL calls. It does not use cgo and does not decode packed Ghostty cell bits.
- The Ghostty DLL used by the working native C reference and its passing CTest suite is the compatibility target. The exact upstream Git pin is not a migration blocker.

## ConPTY and Go Session

- `internal/terminal/conpty_windows.go` implements ConPTY creation, process launch, Windows argv quoting, pipe I/O, resize, process polling, output-EOF notification and cleanup.
- `internal/terminal/session_windows.go` composes the Go ConPTY manager and Ghostty host while preserving the public Session methods: `StartSSH`, `Start`, `Write`, `Resize`, `PollExit`, `Snapshot`, `SnapshotText`, and `Close`.
- The Go Session preserves the original C initialization order: create Ghostty and install its callback, resize the VT, then start ConPTY. The input callback waits until the ConPTY handle is published so an early terminal query can be answered.
- VT feed, resize and snapshot are serialized. ConPTY writes have their own lock. Close stops and joins the ConPTY reader before destroying the Ghostty host, without holding the VT lock while joining.
- `StartSSH` continues to use `internal/ssh.BuildArgs()`, resolves Windows `ssh.exe`, and starts it without a shell. `StrictHostKeyChecking=yes` remains enabled.
- The Go application no longer loads or depends on `terminal_libghosty_session.dll`.

## Compatibility tests

The original C implementation, headers, CMake targets and C tests remain in
`native/` as a runnable reference baseline.

Equivalent Go tests cover:

- Ghostty PTY callback and DSR 5 -> DSR 0 reply.
- UTF-8 grapheme content, ANSI bold/green foreground color and cursor blink.
- Ghostty snapshot cell/text buffers and resize dimensions.
- ConPTY argv quoting, input/output round-trip, long output, process exit,
  output-EOF callback timing, failed startup and concurrent Close.
- A child process that sends a DSR query and verifies the Ghostty response
  arrives through ConPTY input.
- Real Windows `ssh.exe -V` output in the Ghostty snapshot.
- SSH connection-refused output and styled cell snapshot through StartSSH.

## Verified Windows build

- `CGO_ENABLED=0 go test ./...`: passed.
- `CGO_ENABLED=0 go build ./...`: passed.
- `go vet -unsafeptr=false ./...`: passed. Default `go vet ./...` still
  reports unsafe.Pointer diagnostics in existing Windows GUI/D2D FFI code.
- The original native CTest baseline: 4/4 passed
  (`libghostty_vt_smoke`, `ghostty_host_bridge`, `windows_conpty_smoke`,
  `ssh_conpty_ghostty_e2e`).
- Both Windows GUI executables build with `-H=windowsgui` and cgo disabled.
  The official `ghostty-vt.dll` was copied beside them in `dist/`.

## Remaining work

1. Improve the GDI renderer: font selection, DPI-aware cell metrics, complete
   style flags (bold/italic/underline), wide-cell layout, scrollback, and
   cursor shape/blink. The current host draws Ghostty cell colors, selection,
   graphemes, and a basic cursor.
2. Complete keyboard handling for function keys and modifiers, then add IME,
   focus reporting, mouse reporting, local selection, clipboard, and paste.
3. Test authenticated remote SSH sessions, host-key first-use/mismatch,
   remote disconnects, Unicode/TUI applications, alternate screen, resize,
   long output, and handle cleanup.

## Scope

The production SSH -> Go ConPTY -> Ghostty VT path is implemented and tested.
The profile manager embeds a Win32 child host that calls `StartSSH()`. The
host paints Ghostty cell snapshots (colors, selection, graphemes, cursor),
routes basic text/navigation input, and resizes the session with its panel.
This is a working first integration, not full terminal parity: IME, clipboard,
mouse reporting/selection, scrollback, DPI/font handling, and advanced styles
remain outstanding. A live authenticated remote session has not yet been
accepted; current runtime tests use a closed loopback port.
