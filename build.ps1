# Build one executable; the GUI forks itself with --server for terminal hosting.
New-Item -ItemType Directory -Force dist | Out-Null
$env:CGO_ENABLED = "0"
go build -ldflags="-H=windowsgui" -o dist/terminal-libghosty.exe ./cmd/terminal-libghosty
Copy-Item build/native-local/ghostty-vt.dll dist/ghostty-vt.dll
Remove-Item dist/terminal-host.exe -ErrorAction SilentlyContinue
