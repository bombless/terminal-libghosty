# Implementation status

## Landed in bootstrap

- Product architecture and TUI compatibility requirements.
- SSH target validation and argv construction (no shell interpolation).
- SSH uses forced PTY allocation and explicitly keeps strict host-key checking enabled.
- Project-owned C ABI boundary for the native terminal host.
- gccgo build/test targets for the pure-Go bootstrap CLI.

## Not implemented yet

- Windows GUI/window lifecycle and terminal surface renderer.
- ConPTY create/resize/close and bidirectional pipe pumps.
- Binding to the upstream libghostty embedding API.
- Session startup and interactive event loop.
- Windows integration tests and Codex TUI compatibility tests.

## Blocking dependency decision

The implementation must first pin a Windows-capable libghostty release/revision that exposes the full embedding/surface API. A terminal VT parser alone cannot render a Windows terminal surface. The adapter must be written against the real upstream header rather than speculative function names. The build also needs a verified gccgo-to-C ABI strategy for the selected Windows GCC distribution; the bootstrap intentionally avoids cgo and does not pretend the Go/C bridge is already linked.
