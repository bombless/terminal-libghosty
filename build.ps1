# Build the Ghostty VT static library and link it into the single executable.
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$knownZig = "D:\mcp-agent-workspace\tools\zig-0.17.0\zig-x86_64-windows-0.17.0\zig.exe"
$zig = if ($env:ZIG) { (Get-Command $env:ZIG -ErrorAction Stop).Source }
       elseif (Get-Command zig -ErrorAction SilentlyContinue) { (Get-Command zig).Source }
       elseif (Test-Path $knownZig) { $knownZig }
       else { throw "zig not found in PATH or at $knownZig" }

Push-Location ghostty
try {
    & $zig build -Demit-lib-vt -Dtarget=x86_64-windows-gnu -Doptimize=ReleaseFast
    if ($LASTEXITCODE -ne 0) { throw "zig build failed with exit code $LASTEXITCODE" }
} finally {
    Pop-Location
}

New-Item -ItemType Directory -Force build/native-static/include | Out-Null
Copy-Item ghostty/zig-out/lib/ghostty-vt-static.lib build/native-static/libghostty-vt-static.a -Force
Copy-Item ghostty/include/ghostty build/native-static/include/ -Recurse -Force

New-Item -ItemType Directory -Force dist | Out-Null
$env:CGO_ENABLED = "1"
if (-not $env:CC) {
    $zigCC = $zig -replace '\\', '/'
    $env:CC = "`"$zigCC`" cc -target x86_64-windows-gnu"
}
go build -ldflags="-H=windowsgui -extldflags=-Wl,--subsystem,windows" -o dist/terminal-libghosty.exe ./cmd/terminal-libghosty
if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" }
$distExe = Join-Path $PSScriptRoot "dist/terminal-libghosty.exe"
$bytes = [IO.File]::ReadAllBytes($distExe)
$pe = [BitConverter]::ToInt32($bytes, 0x3c)
$subsystem = [BitConverter]::ToUInt16($bytes, $pe + 0x5c)
if ($subsystem -ne 2) { throw "terminal-libghosty.exe is not a GUI subsystem binary (subsystem=$subsystem)" }
Copy-Item ghostty/LICENSE dist/LICENSE-ghostty -Force
if (Test-Path resources/conpty) {
    Copy-Item resources/conpty/* dist/ -Force
}
Remove-Item dist/ghostty-vt.dll -ErrorAction SilentlyContinue
