# Build terminal-libghosty for Windows x64.
# Requirements: Go 1.22+, Zig 0.17.0, and the initialized ghostty Git submodule.
[CmdletBinding()]
param([switch]$RunTests)

$ErrorActionPreference = "Stop"
Set-Location -LiteralPath $PSScriptRoot

function Stop-Build([string]$Message) { throw $Message }

# Resolve Zig from env:ZIG (path or command), PATH, then the known local install.
$zig = $null
if ($env:ZIG) {
    if (Test-Path -LiteralPath $env:ZIG -PathType Leaf) {
        $zig = (Resolve-Path -LiteralPath $env:ZIG).Path
    } else {
        $zigCommand = Get-Command $env:ZIG -ErrorAction SilentlyContinue
        if ($zigCommand) { $zig = $zigCommand.Source }
    }
}
if (-not $zig) {
    $zigCommand = Get-Command zig -ErrorAction SilentlyContinue
    if ($zigCommand) { $zig = $zigCommand.Source }
}
$knownZig = "D:\mcp-agent-workspace\tools\zig-0.17.0\zig-x86_64-windows-0.17.0\zig.exe"
if (-not $zig -and (Test-Path -LiteralPath $knownZig -PathType Leaf)) { $zig = $knownZig }
if (-not $zig) {
    Stop-Build "Zig 0.17.0 was not found. Install Zig 0.17.0 and add it to PATH, or set the ZIG environment variable to zig.exe."
}

$zigVersion = (& $zig version).Trim()
if ($LASTEXITCODE -ne 0) { Stop-Build "Unable to run Zig: $zig" }
if ($zigVersion -ne "0.17.0") { Stop-Build "This project build expects Zig 0.17.0; found $zigVersion ($zig)." }

$goVersion = (& go version)
if ($LASTEXITCODE -ne 0) { Stop-Build "Go is not installed or is not available in PATH." }
$goEnv = (& go env GOOS GOARCH)
if ($LASTEXITCODE -ne 0) { Stop-Build "Unable to query the Go toolchain." }
if ($goEnv[0] -ne "windows" -or $goEnv[1] -ne "amd64") {
    Stop-Build "Build from Windows amd64. Current Go target: $($goEnv -join '/')."
}
Write-Host "Go: $goVersion"
Write-Host "Zig: $zigVersion ($zig)"

# Check that the Git submodule is populated.
if (-not (Test-Path -LiteralPath "ghostty/build.zig" -PathType Leaf) -or
    -not (Test-Path -LiteralPath "ghostty/include/ghostty" -PathType Container)) {
    Stop-Build "The Ghostty submodule is missing or incomplete. Run: git submodule update --init --recursive"
}

Write-Host ""
Write-Host "[1/3] Building Ghostty VT static library..."
Push-Location ghostty
try {
    & $zig build -Demit-lib-vt -Dtarget=x86_64-windows-gnu -Doptimize=ReleaseFast
    if ($LASTEXITCODE -ne 0) { Stop-Build "Zig build failed with exit code $LASTEXITCODE." }
} finally {
    Pop-Location
}

$nativeLib = "ghostty/zig-out/lib/ghostty-vt-static.lib"
if (-not (Test-Path -LiteralPath $nativeLib -PathType Leaf)) {
    Stop-Build "Zig completed but the expected static library was not produced: $nativeLib"
}
$nativeDir = "build/native-static"
New-Item -ItemType Directory -Force "$nativeDir/include" | Out-Null
Copy-Item -LiteralPath $nativeLib -Destination "$nativeDir/libghostty-vt-static.a" -Force
Copy-Item -Path "ghostty/include/ghostty" -Destination "$nativeDir/include" -Recurse -Force

if ($RunTests) {
    Write-Host ""
    Write-Host "[2/3] Running Go tests..."
    $env:CGO_ENABLED = "1"
    $zigCC = $zig.Replace('\', '/')
    $env:CC = '"' + $zigCC + '" cc -target x86_64-windows-gnu'
    & go test ./...
    if ($LASTEXITCODE -ne 0) { Stop-Build "Go tests failed with exit code $LASTEXITCODE." }
} else {
    Write-Host ""
    Write-Host "[2/3] Skipping Go tests (pass -RunTests to enable)."
}

Write-Host ""
Write-Host "[3/3] Building Windows GUI executable..."
New-Item -ItemType Directory -Force "dist" | Out-Null
$env:CGO_ENABLED = "1"
$zigCC = $zig.Replace('\', '/')
$env:CC = '"' + $zigCC + '" cc -target x86_64-windows-gnu'
& go build -ldflags="-H=windowsgui -extldflags=-Wl,--subsystem,windows" -o "dist/terminal-libghosty.exe" ./cmd/terminal-libghosty
if ($LASTEXITCODE -ne 0) { Stop-Build "Go build failed with exit code $LASTEXITCODE." }

# Confirm that the output is a Windows GUI (not console) executable.
$exePath = Join-Path $PSScriptRoot "dist/terminal-libghosty.exe"
$bytes = [IO.File]::ReadAllBytes($exePath)
if ($bytes.Length -lt 256 -or $bytes[0] -ne 0x4D -or $bytes[1] -ne 0x5A) {
    Stop-Build "The generated file is not a valid Windows PE executable."
}
$peOffset = [BitConverter]::ToInt32($bytes, 0x3C)
if ($peOffset -lt 0 -or $peOffset + 0x60 -ge $bytes.Length -or
    [BitConverter]::ToUInt32($bytes, $peOffset) -ne 0x00004550) {
    Stop-Build "The generated executable has an invalid PE header."
}
$subsystem = [BitConverter]::ToUInt16($bytes, $peOffset + 0x5C)
if ($subsystem -ne 2) { Stop-Build "Expected a Windows GUI executable (subsystem 2), got subsystem $subsystem." }

# Package runtime support files alongside the application executable.
Copy-Item -LiteralPath "ghostty/LICENSE" -Destination "dist/LICENSE-ghostty" -Force
if (Test-Path -LiteralPath "resources/conpty" -PathType Container) {
    foreach ($runtimeFile in Get-ChildItem -LiteralPath "resources/conpty" -File) {
        $destination = Join-Path "dist" $runtimeFile.Name
        try {
            Copy-Item -LiteralPath $runtimeFile.FullName -Destination $destination -Force -ErrorAction Stop
        } catch {
            if (Test-Path -LiteralPath $destination -PathType Leaf) {
                Write-Warning "Could not replace $destination (it may be in use). Keeping the existing file."
            } else {
                throw
            }
        }
    }
}
Remove-Item -LiteralPath "dist/ghostty-vt.dll" -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "Build succeeded." -ForegroundColor Green
Write-Host "Output: $exePath"
Get-Item -LiteralPath $exePath | Select-Object FullName, Length, LastWriteTime
