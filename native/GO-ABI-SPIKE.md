# Pure-Go Ghostty DLL ABI and native-C compatibility

Date: 2026-11-08

## Result

The Windows x64 ABI smoke test and the Go port of the native C host/session
behavior pass with `CGO_ENABLED=0`. The production Go Session now composes
the Go ConPTY manager with a direct `ghostty-vt.dll` adapter; it does not load
`terminal_libghosty_session.dll`.

The Go adapter preserves the behavior of the working native C implementation:

- terminal creation, destruction, VT feed, and resize;
- USERDATA and WRITE_PTY callback installation, including DSR 5 -> DSR 0 reply;
- render-state sizing/copy flow, UTF-8 graphemes, resolved RGB, style flags,
  selection and cursor fields;
- ConPTY process startup, input/output, exit polling, output-EOF callback,
  close and resize;
- SSH argv generation, `ssh.exe -V`, and closed-loopback-port error output.

The original C source, headers, CMake targets and tests are intentionally
retained as a runnable compatibility baseline. The CMake upstream Git pin is
not a prerequisite for this migration: the working C implementation and the
DLL used by its passing tests define the expected behavior.

## DLL observed on this machine

- Path: `build/native-local/ghostty-vt.dll`
- SHA-256: `7306477EFEF28F75D74BAAA11B553FF0FF49024393303DBEF33AF3042EAFD3BD`
- PE machine: AMD64 / x86-64
- Reported runtime version: `1.1.0-dev`
- Required exports used by the Go adapter: `ghostty_terminal_new/free/vt_write/resize/set`,
  `ghostty_render_state_new/free/update/get/clean`,
  `ghostty_render_state_row_iterator_new/free/next/get`,
  `ghostty_render_state_row_cells_new/free/next/get`
- Imports observed: `ntdll.dll`, `KERNEL32.dll`, `VCRUNTIME140.dll`,
  and Windows API-set CRT libraries.
- No Ghostty import library is needed: Go resolves the public exports at
  runtime from the DLL.

## Compatibility tests

Go tests equivalent to the native C tests:

- `TestGhosttyHostMatchesNativeBridgeBehavior`: DSR callback, UTF-8,
  ANSI bold/green color, cursor blink, resize and invalid dimensions.
- `TestSessionGhosttyDSRReplyRoundTrip`: child process asks CSI 5n and
  receives CSI 0n from Ghostty through ConPTY input.
- `TestNativeSSHVersionE2E`: `ssh.exe -V` output is visible in the
  Ghostty text snapshot.
- `TestStartSSHBuildArgsReachConPTY`: SSH closed-port error and styled cell
  snapshot.
- ConPTY tests cover argv quoting, input/output, long output, process polling,
  output-EOF callback, failed creation cleanup and concurrent Close.

The original CMake/CTest reference suite also passed 4/4 tests:
`libghostty_vt_smoke`, `ghostty_host_bridge`, `windows_conpty_smoke`,
and `ssh_conpty_ghostty_e2e`.

Reproduce the Go checks from the repository root in PowerShell:

```powershell
$env:CGO_ENABLED = '0'
go test ./...
go build ./...
```

Reproduce the native C compatibility baseline:

```powershell
ctest --test-dir build/native-local --output-on-failure
```

## Deployment

The Go application loads `ghostty-vt.dll` directly. Set
`GHOSTTY_VT_DLL` or `TERMINAL_LIBGHOSTY_NATIVE_DIR`, or place the DLL beside
the application. The Makefile copies the official DLL into `dist`.

The original C implementation remains in `native/` for regression comparison;
it is not part of the production Go application's runtime dependency chain.
