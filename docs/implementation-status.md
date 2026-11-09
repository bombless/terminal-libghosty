# Implementation status

## Existing application

- SSH profile manager GUI and profile persistence.
- SSH target validation and argument construction.
- Strict host-key checking and forced PTY requested for SSH sessions.
- The right-hand workspace is tabbed. The SSH CONFIG tab always remains available for editing profiles; Connect and double-clicking a saved profile each create a new embedded Win32 child-window host in a session tab, so multiple SSH sessions can run at once.
- **NEW PWSH** and `--shell pwsh` create the same embedded host with `internal/terminal.StartPowerShell()`, which runs local `pwsh.exe` in ConPTY.
- Each session tab has a close button that terminates its host process and destroys the tab container; when a session exits on its own, the parent removes the tab automatically.

## Ghostty VT integration

- The project uses the public `libghostty-vt` C ABI. Production builds statically link `ghostty-vt-static.lib`; the upstream internal `ghostty.h` embedder API is not used.
- The original `native/ghostty_host.c` is the compatibility reference for terminal creation, VT feed, WRITE_PTY callback, render-state iteration, UTF-8 graphemes, resolved colors, style flags, selection, cursor and resize.
- `internal/terminal/ghostty_host_windows.go` implements that host behavior in Go. With cgo enabled it calls the statically linked library through a small C shim; with cgo disabled it falls back to Windows DLL calls for development and ABI testing. It does not decode packed Ghostty cell bits.
- The Ghostty DLL used by the working native C reference and its passing CTest suite is the compatibility target. The exact upstream Git pin is not a migration blocker.

## ConPTY and Go Session

- `internal/terminal/conpty_windows.go` implements ConPTY creation, process launch, Windows argv quoting, pipe I/O, resize, process polling, output-EOF notification and cleanup.
- `internal/terminal/session_windows.go` composes the Go ConPTY manager and Ghostty host while preserving the public Session methods: `StartSSH`, `StartPowerShell`, `Start`, `Write`, `Resize`, `PollExit`, `Snapshot`, `SnapshotText`, and `Close`.
- The Go Session preserves the original C initialization order: create Ghostty and install its callback, resize the VT, then start ConPTY. The input callback waits until the ConPTY handle is published so an early terminal query can be answered.
- VT feed, resize and snapshot are serialized. ConPTY writes have their own lock. Close stops and joins the ConPTY reader before destroying the Ghostty host, without holding the VT lock while joining.
- `StartSSH` continues to use `internal/ssh.BuildArgs()`, resolves Windows `ssh.exe`, and starts it without a shell. `StrictHostKeyChecking=yes` remains enabled.
- `StartPowerShell` resolves `pwsh.exe` from `PATH` and starts it as `pwsh.exe -NoLogo`, also without a shell wrapper.
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

- `ghostty-vt-static.lib` built with Zig 0.17.0
  (`zig build -Demit-lib-vt -Dtarget=x86_64-windows-gnu -Doptimize=ReleaseFast`)
  and staged to `build/native-static`.
- `CGO_ENABLED=1 CC="zig cc -target x86_64-windows-gnu" go build ./...`: passed.
- `CGO_ENABLED=1` `go test ./...`: passed without setting `GHOSTTY_VT_DLL`,
  exercising the statically linked terminal path end to end.
- `CGO_ENABLED=0` with `GHOSTTY_VT_DLL`: build and the same tests still pass,
  confirming the dynamic development/ABI fallback.
- `build.ps1` produced `dist/terminal-libghosty.exe` and removed the old DLL
  from `dist/`. The exe imports only `ntdll.dll`, `KERNEL32.dll` and the
  Windows UCRT api-set DLLs; `ghostty-vt.dll` is not imported.
- The GUI executable launches and remains running in a smoke test.
- `go vet -unsafeptr=false ./...`: passed. Default `go vet ./...` still
  reports unsafe.Pointer diagnostics in existing Windows GUI/D2D FFI code.
- The original native CTest baseline: 4/4 passed
  (`libghostty_vt_smoke`, `ghostty_host_bridge`, `windows_conpty_smoke`,
  `ssh_conpty_ghostty_e2e`).

## Remaining work

1. Improve the GDI renderer: font selection, DPI-aware cell metrics, complete
   style flags (bold/italic/underline), wide-cell layout, scrollback, and
   cursor shape/blink. The current host draws Ghostty cell colors, selection,
   graphemes, and a basic cursor.
2. Complete keyboard handling for function keys and modifiers, then add
   focus reporting, mouse reporting, local selection, clipboard, and paste.
   IME input (inline composition string, candidate positioning and commit into
   the ConPTY session) is implemented for the Microsoft Pinyin IME.
3. Test authenticated remote SSH sessions, host-key first-use/mismatch,
   remote disconnects, Unicode/TUI applications, alternate screen, resize,
   long output, and handle cleanup.

## Scope

The production SSH -> Go ConPTY -> Ghostty VT path is implemented and tested.
The profile manager spawns one host per session tab; the host draws Ghostty
cell snapshots (colors, selection, graphemes, cursor), routes text/navigation
input plus IME composition, and tracks its tab container geometry.
This is a working first integration, not full terminal parity: clipboard,
mouse reporting/selection, scrollback, DPI/font handling, and advanced styles
remain outstanding. A live authenticated remote session has not yet been
accepted; current runtime tests use a closed loopback port.
