@echo off
setlocal
set "ROOT=%~dp0.."
set "EXE=%ROOT%\dist\terminal-libghosty.exe"

if not exist "%EXE%" (
    echo Building terminal-libghosty...
    pushd "%ROOT%"
    go build -ldflags="-H=windowsgui" -o "%EXE%" ./cmd/terminal-libghosty
    popd
)

start "" "%EXE%"
