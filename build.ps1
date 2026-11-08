# 1. 创建 dist 目录
New-Item -ItemType Directory -Force dist | Out-Null

# 2. 禁用 CGO
$env:CGO_ENABLED = "0"

# 3. 编译主程序
go build -ldflags="-H=windowsgui" -o dist/terminal-libghosty.exe ./cmd/terminal-libghosty

# 4. 编译 Host
go build -ldflags="-H=windowsgui" -o dist/terminal-host.exe ./cmd/terminal-host

# 5. 复制 Ghostty DLL
Copy-Item build/native-local/ghostty-vt.dll dist/ghostty-vt.dll
