.PHONY: test build

ifeq ($(OS),Windows_NT)
CGO_ENV = set CGO_ENABLED=0&&
else
CGO_ENV = CGO_ENABLED=0
endif

test:
	$(CGO_ENV) go test ./...

build:
	$(CGO_ENV) go build -ldflags="-H=windowsgui" -o dist/terminal-libghosty.exe ./cmd/terminal-libghosty
	cmake -E copy_if_different build/native-local/ghostty-vt.dll dist/ghostty-vt.dll
	$(if $(filter Windows_NT,$(OS)),powershell -NoProfile -Command "Remove-Item dist/terminal-host.exe -ErrorAction SilentlyContinue",rm -f dist/terminal-host.exe)
