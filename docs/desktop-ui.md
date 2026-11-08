# Windows desktop UI

Run `desktop/Launch Terminal Libghosty.bat` or `dist/terminal-libghosty.exe` to open the graphical SSH connection manager. Add and save `user@host` profiles, optional port overrides and identity files. Profiles are stored in `%APPDATA%\TerminalLibghosty\connections.json`. Connect launches an embedded hardware-accelerated Direct2D terminal surface (`internal/d2d`, `internal/gui`). The terminal calls `internal/terminal.StartSSH()` and uses the system OpenSSH client through the Go ConPTY manager; strict host-key checking remains enabled. Use **BACK / DISCONNECT** to close the SSH session and return to profile editing.

The production application is built with `CGO_ENABLED=0`. ConPTY process management, Ghostty host integration and Session composition are implemented in Go. Only the official `ghostty-vt.dll` is required at runtime; place it beside the application or set `GHOSTTY_VT_DLL` / `TERMINAL_LIBGHOSTY_NATIVE_DIR`. The project-owned `terminal_libghosty_session.dll` is not used by the Go application. The original C implementation remains available as a compatibility reference and can still be built with CMake.

The Direct2D renderer consumes Ghostty cell snapshots, including colors, selection and cursor, and supports keyboard input and dynamic resize.
