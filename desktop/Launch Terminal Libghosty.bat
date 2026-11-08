@echo off
taskkill /F /IM terminal-host.exe >nul 2>&1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0TerminalLibghosty.ps1" 1>>"%~dp0gui-start.stdout.log" 2>>"%~dp0gui-start.stderr.log"
