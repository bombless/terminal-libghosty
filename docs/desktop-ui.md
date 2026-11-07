# Windows desktop UI

Run `desktop/Launch Terminal Libghosty.bat` to open the graphical SSH connection manager. Add and save `user@host` profiles, optional port overrides and identity files. Profiles are stored in `%APPDATA%\TerminalLibghosty\connections.json`. Connect opens a Windows Terminal tab using the system OpenSSH client with TTY allocation and strict host-key checking.

This is a functional profile-manager GUI and SSH launcher. The interactive terminal is currently hosted by Windows Terminal; in-process ConPTY + libghostty rendering and gccgo integration are not implemented. This desktop does not have gccgo installed, and Go module downloads were blocked, so a native Go GUI binary could not be built here.
