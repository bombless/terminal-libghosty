# Implementation status

## Existing application

- SSH profile manager GUI and profile persistence.
- SSH target validation and argument construction.
- Strict host-key checking and forced PTY requested for SSH sessions.
- The profile manager's Connect action now creates an embedded Win32 child-window host in the existing right-hand panel. The helper calls `internal/terminal.StartSSH()` with the selected profile and native DLL directory; it no longer launches `wt.exe`.

## Upstream Ghostty integration

- Inspected upstream `ghostty-org/ghostty` at
  `5813c9461d5af78b6307a1254d00b1bc190eaccd`.
- Added a Windows CMake integration pinned to that exact revision.
- Added a C ABI smoke test using upstream `<ghostty/vt.h>` declarations.
- The selected Windows-supported component is `libghostty-vt`: terminal state,
  VT parsing, render-state snapshots, cell/style iteration, and input encoders.
- The upstream `ghostty.h` internal embedder API is not a general-purpose
  Windows embedding API. This project provides its own Win32 host and renderer
  around the supported VT library rather than treating the macOS-oriented
  internal API as portable.
- The local Ghostty source snapshot used for the successful build reports
  Ghostty 1.4.1-dev / libghostty-vt 1.1.0-dev and has no Git metadata. It cannot
  be confirmed to match the CMake pin; validate against the exact pinned
  revision before shipping.

## Native SSH / ConPTY / Ghostty VT bridge

- Added `native/ssh_terminal.h/.c` to own ConPTY and the Ghostty VT host as one
  session. ConPTY reader output is fed into the VT parser; Ghostty's synchronous
  `WRITE_PTY` callback writes terminal replies back into ConPTY.
- The session ABI serializes VT feed/snapshot/resize and exposes input writes,
  resize, exit polling, cell/text snapshots, and close.
- Added `internal/terminal.StartSSH`, which reuses `internal/ssh.BuildArgs()`,
  resolves Windows `ssh.exe`, and starts it through the native DLL without a
  shell. `StrictHostKeyChecking=yes` remains enabled.
- End-to-end tests cover local `cmd.exe` input/output, a DSR query whose
  Ghostty-generated reply is consumed by a child process, resize, exit, and the
  real Windows `ssh.exe -V` process through the same bridge.
- The Go Windows wrapper runtime tests load the DLL, run `ssh.exe -V`, and
  verify `StartSSH()` reports the expected closed-loopback-port failure through
  the VT snapshot. During development set `TERMINAL_LIBGHOSTY_NATIVE_DIR` to
  the directory containing `terminal_libghosty_session.dll` and `ghostty-vt.dll`.

## Remaining work

1. Validate the used API against the exact pinned Ghostty revision; the local
   source snapshot has no Git metadata and cannot establish pin identity.
2. Improve the GDI renderer: font selection, DPI-aware cell metrics, complete
   style flags (bold/italic/underline), wide-cell layout, scrollback, and
   cursor shape/blink. The current host draws Ghostty cell colors, selection,
   graphemes, and a basic cursor.
3. Complete keyboard handling for function keys and modifiers, then add IME,
   focus reporting, mouse reporting, local selection, clipboard, and paste.
4. Test authenticated remote SSH sessions, host-key first-use/mismatch,
   remote disconnects, Unicode/TUI applications, alternate screen, resize,
   long output, and handle cleanup.

## Verified Windows build

- Release CMake build succeeds using Zig 0.17.0, LLVM-MinGW Clang, CMake 3.25.1,
  and the local Ghostty source directory.
- `ctest --test-dir build/native-local -C Release --output-on-failure`: 4/4 passed.
- `go test ./...`: passed, including native Windows `ssh.exe -V` and `StartSSH()` loopback tests.
- `go vet ./...`: passed.
- The Go wrapper uses the standard Go Windows syscall/DLL interface; cgo and
  gccgo are not required for this session bridge.

## Scope

The native SSH → ConPTY → Ghostty VT bridge is implemented and tested, and
the profile manager now embeds a Win32 child host that calls `StartSSH()`.
The host paints Ghostty cell snapshots (colors, selection, graphemes, cursor),
routes basic text/navigation input, and resizes the session with its panel.
This is a working first integration, not full terminal parity: IME, clipboard,
mouse reporting/selection, scrollback, DPI/font handling, and advanced styles
remain outstanding. A live authenticated remote session has not yet been
accepted; current runtime tests use a closed loopback port.
