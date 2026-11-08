# Native Windows terminal host

The Windows host is built around the official Ghostty C API exposed by
`<ghostty/vt.h>` (the `libghostty-vt` component), not the macOS-only
`ghostty.h` internal embedder interface.

## Upstream boundary

- Upstream: https://github.com/ghostty-org/ghostty
- Intended pinned revision: `5813c9461d5af78b6307a1254d00b1bc190eaccd` (not yet verified against the local source tree; see [ABI design](ABI-DESIGN.md)).
- Local validation: `FETCHCONTENT_SOURCE_DIR_GHOSTTY` points at `D:/mcp-agent-workspace/ghostty-main`; that checkout has no Git metadata, so this does not prove pin compatibility.
- Build: upstream CMake wrapper invokes Zig 0.17 and produces
  `ghostty-vt.dll` plus import/static libraries on Windows.
- API: terminal state, VT parsing, render-state snapshots, row/cell/style
  iteration, keyboard/mouse encoders, selection, clipboard/paste helpers.

The official Ghostty GUI is not being ported. The old `include/ghostty.h`
header is explicitly an internal embedder API and is not a general-purpose
Windows embedding contract. The supported Windows route is to consume
`libghostty-vt` and supply the platform host and renderer in this project.

## Windows host responsibilities

1. Win32 window/control lifecycle and keyboard, mouse, IME, clipboard events.
2. Render `GhosttyRenderState` rows/cells/styles in an embedded Win32 surface.
3. Encode keyboard, paste, focus, and mouse input with the upstream VT API.
4. Connect the existing profile-manager GUI to `internal/terminal.StartSSH` and
   manage multiple session/window lifetimes.

## ConPTY process module

`windows_conpty.h/.c` is a Windows-only, project-owned process/PTY layer,
separate from the Ghostty VT bridge. It creates a ConPTY with synchronous
pipe handles, launches a UTF-16 executable/argv using
`STARTUPINFOEXW` + `PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE`, and exposes:

- `tlg_conpty_write` for raw terminal input bytes;
- `tlg_conpty_resize` for character-grid resize (zero dimensions are rejected);
- an output callback invoked on the dedicated reader worker thread;
- `tlg_conpty_poll_exit` and an exit callback;
- `tlg_conpty_close`, which terminates a still-running child, closes the
  pseudoconsole, joins the reader worker, and releases owned handles.

`windows_conpty.h/.c` is wrapped by `ssh_terminal.h/.c`, which owns one
ConPTY process and one Ghostty VT host as a single session. Its reader callback
feeds every PTY output chunk into `tlg_ghostty_host_feed_output`; Ghostty's
synchronous `WRITE_PTY` callback writes terminal replies back to ConPTY. The
session API serializes VT feed/snapshot/resize operations, keeps process I/O off
the UI thread, and exposes write, resize, exit polling, cell/text snapshots,
and close. `internal/terminal.StartSSH` first calls the existing
`ssh.BuildArgs()` (including `StrictHostKeyChecking=yes`) and passes the
resulting argv to the native DLL without invoking a shell.

The end-to-end CTest launches `cmd.exe` for input/output, launches a probe
child that issues DSR and verifies the Ghostty-generated reply reaches its
stdin, and launches the real Windows `ssh.exe -V` through the same session
path. The Go wrapper also exercises `StartSSH()` against a closed loopback
port and verifies OpenSSH's connection-refused output through the VT snapshot.
These tests validate process creation and the SSH/ConPTY/VT data path without
requiring remote credentials. They do not claim that an authenticated remote
host was reached. The profile manager now launches `cmd/terminal-host` from its
Connect action and embeds its Win32 child surface; the renderer uses Ghostty
cell snapshots for color, selection, graphemes, and cursor. IME, clipboard,
mouse reporting/selection, scrollback, DPI/font handling, and advanced styles
remain follow-up work.

## Build and smoke test

From a Visual Studio Developer PowerShell or another Windows C compiler shell:

```powershell
cmake -S native -B build/native -A x64
cmake --build build/native --config Release
ctest --test-dir build/native -C Release --output-on-failure
```

Requirements: CMake 3.19+, Zig 0.17.0 on PATH, Git, and a Windows C compiler. The ConPTY module requires Windows 10 version 1809 (build 17763) or newer.
The CMake project fetches the pinned upstream revision. It does not fetch
unbounded `main`.

`libghostty_vt_smoke` validates the actual upstream C ABI and VT state
handling. It is a dependency/integration test, not yet the embedded GUI
terminal. The native session bridge is built as `terminal_libghosty_session.dll`; the Win32 renderer and GUI profile-manager integration remain separate milestones.
