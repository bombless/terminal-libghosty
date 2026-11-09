# Build the Ghostty VT static library and link it into the single executable.
$ErrorActionPreference = "Stop"

$zig = if ($env:ZIG) { (Get-Command $env:ZIG -ErrorAction Stop).Source } else { (Get-Command zig -ErrorAction Stop).Source }

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
go build -ldflags="-H=windowsgui" -o dist/terminal-libghosty.exe ./cmd/terminal-libghosty
if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" }
Copy-Item ghostty/LICENSE dist/LICENSE-ghostty -Force
Remove-Item dist/ghostty-vt.dll -ErrorAction SilentlyContinue
