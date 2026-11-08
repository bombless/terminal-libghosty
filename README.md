# terminal-libghosty

A Windows SSH connection manager with a graphical profile picker and an SSH terminal launcher. See [desktop UI instructions](docs/desktop-ui.md).

The GUI Connect action now starts `internal/terminal.StartSSH()` through a small Win32 child-window host embedded in the profile manager. The host renders copied Ghostty cell snapshots (including cell foreground/background colors, selection and cursor), routes basic keyboard input, and synchronizes ConPTY/Ghostty grid resize. SSH argv still comes from `internal/ssh.BuildArgs()` with strict host-key verification. This is an initial embedded renderer: IME, clipboard, mouse reporting/selection, font/DPI handling, and full style fidelity still need work before production acceptance. See [implementation status](docs/implementation-status.md).

The Go application uses the standard Go toolchain with cgo enabled (`CGO_ENABLED=1`); gccgo is not used. `libghostty-vt` is built separately with Zig, and the native Windows host is built with a Windows C compiler. See [native build notes](native/README.md).
