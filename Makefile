.PHONY: test build build-gccgo

test:
	go test ./...

build:
	go build -ldflags="-H=windowsgui" -o dist/terminal-libghosty.exe ./cmd/desktop

build-gccgo:
	go build -compiler gccgo -o dist/terminal-libghosty.exe ./cmd/desktop
