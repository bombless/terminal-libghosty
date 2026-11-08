.PHONY: test build

ifeq ($(OS),Windows_NT)
CGO_ENV = set CGO_ENABLED=1&&
else
CGO_ENV = CGO_ENABLED=1
endif

test:
	$(CGO_ENV) go test ./...

build:
	$(CGO_ENV) go build -ldflags="-H=windowsgui" -o dist/terminal-libghosty.exe ./cmd/terminal-libghosty
	$(CGO_ENV) go build -ldflags="-H=windowsgui" -o dist/terminal-host.exe ./cmd/terminal-host
