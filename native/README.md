# Native libghostty adapter

`ghostty_host.h` is the project-owned ABI consumed by the Go control plane. The implementation is intentionally not stubbed with guessed upstream calls. Before implementing `ghostty_host.c`:

1. Pin a libghostty commit/tag that ships a supported embedding header and Windows build.
2. Confirm the upstream surface creation, renderer, event loop, key/mouse, clipboard, and resize APIs from that exact header.
3. Implement this adapter using those declarations and the upstream-required allocator/platform callbacks.
4. Add a Windows CI job that links the real library and exercises a surface smoke test.

A parser-only `libghostty-vt` build is not a substitute for the full renderer/surface API required by this application.
