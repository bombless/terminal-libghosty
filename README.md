# terminal-libghosty

A Windows-native SSH terminal host built around libghostty, with a GCC Go control plane and ConPTY-backed remote sessions.

> Status: bootstrap scaffold. The repository was empty when this project was initialized. The native libghostty embedding ABI and Windows support must be validated against the exact libghostty revision shipped by the build before claiming full compatibility.

## Goals

- Windows 10/11 x64 terminal window with a native rendering/input host.
- Connect directly to an SSH target selected when creating a session (`user@host[:port]`).
- Use Windows OpenSSH (`ssh.exe`) inside ConPTY so SSH authentication, agent forwarding, host-key verification, and SSH config remain standard.
- Use libghostty for terminal emulation/rendering rather than interpreting ANSI output in the Go layer.
- Keep the application/control plane in Go and build it with `gccgo`; isolate the unstable native libghostty ABI behind a small C bridge.
- Preserve interactive TUI behavior: alternate screen, cursor/mouse modes, bracketed paste, true color, resize, Unicode and key encoding.

## Architecture

```text
Windows UI / session picker
        |
     Go (gccgo)
        |  narrow C ABI
   C host adapter  <---->  libghostty (pinned native build)
        |
 Windows ConPTY <----> ssh.exe <----> remote shell / Codex TUI
```

The SSH process is the PTY child. Its stdin/stdout are connected to the ConPTY pipes; the ConPTY output is fed to libghostty and libghostty-generated input is written back to the PTY input pipe. Terminal resize events must call `ResizePseudoConsole` and update the libghostty surface size together.

## Build prerequisites

- Windows 10 1809+ or Windows 11 (ConPTY).
- GCC toolchain with `gccgo`, plus a C compiler/linker targeting the same architecture.
- A libghostty build exposing the embedding API for Windows, including its headers and import/static libraries. Set `GHOSTTY_INCLUDE_DIR` and `GHOSTTY_LIB_DIR` in the build environment.
- Windows OpenSSH Client (`ssh.exe`).

The libghostty C ABI is version-sensitive. Do not substitute guessed symbols: implement `native/ghostty_host.c` against the pinned upstream header and fail the build if the required ABI is unavailable.

## SSH target format

Examples:

```powershell
terminal-libghosty.exe --ssh user@example.com
terminal-libghosty.exe --ssh user@example.com:2222
terminal-libghosty.exe --ssh user@example.com --identity C:\\Users\\me\\.ssh\\id_ed25519
```

The application passes the target as an argument (never through a shell command string), uses `ssh -tt`, and leaves host-key checking enabled. Do not accept arbitrary shell fragments as an SSH target.

## TUI compatibility checklist

- [ ] ConPTY creation, pipe lifecycle, and resize propagation.
- [ ] libghostty surface lifecycle, rendering, and input forwarding.
- [ ] Alternate screen and full-screen applications (Codex, vim, htop, etc.).
- [ ] Unicode/IME, wide cells, combining marks, and true color.
- [ ] Mouse reporting, bracketed paste, focus reporting, and clipboard policy.
- [ ] Windows terminal DPI changes and font fallback.
- [ ] SSH host-key prompts, password/keyboard-interactive auth, and agent forwarding.

## Repository layout

- `cmd/terminal-libghosty`: GCC Go entry point and CLI/session validation.
- `internal/ssh`: safe SSH argument construction.
- `internal/pty`: Windows ConPTY lifecycle (native implementation to follow).
- `native`: C ABI boundary for libghostty.
- `docs`: architecture and build notes.

## Current limitations

This initial commit establishes the product contract and safe SSH target handling. It is not yet a functioning graphical terminal: the Windows ConPTY and libghostty host adapter need implementation and validation against an available Windows-capable libghostty SDK. Do not treat the TUI checklist as passing until exercised on Windows with Codex and other full-screen TUIs.
