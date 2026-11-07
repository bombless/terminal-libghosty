GO ?= go
GCCGO ?= gccgo

.PHONY: test build check-gccgo

test:
	$(GO) test ./...

# Uses Go's gccgo compiler backend; no cgo is used by the bootstrap CLI.
build: check-gccgo
	$(GO) build -compiler gccgo -o build/terminal-libghosty.exe ./cmd/terminal-libghosty

check-gccgo:
	@command -v $(GCCGO) >/dev/null 2>&1 || (echo "gccgo not found; install a GCC Go toolchain" >&2; exit 1)
