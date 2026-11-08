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
	$(CGO_ENV) go build -ldflags="-H=windowsgui" -o dist/terminal-host.exe ./cmd/terminal-host
	cmake -E copy_if_different build/native-local/ghostty-vt.dll dist/ghostty-vt.dll
