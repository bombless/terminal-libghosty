.PHONY: test build static icon

ZIG ?= zig
WINDRES ?= windres
CGO_CC ?= $(ZIG) cc -target x86_64-windows-gnu
NATIVE_STATIC := build/native-static

ifeq ($(OS),Windows_NT)
CGO_ENV = set CGO_ENABLED=1&& set "CC=$(CGO_CC)"&&
else
CGO_ENV = CGO_ENABLED=1 CC='$(CGO_CC)'
endif

static:
	cd ghostty && "$(ZIG)" build -Demit-lib-vt -Dtarget=x86_64-windows-gnu -Doptimize=ReleaseFast
	cmake -E make_directory $(NATIVE_STATIC)/include
	cmake -E copy_if_different ghostty/zig-out/lib/ghostty-vt-static.lib $(NATIVE_STATIC)/libghostty-vt-static.a
	cmake -E copy_directory ghostty/include/ghostty $(NATIVE_STATIC)/include/ghostty

# Regenerate the embedded exe icon after editing assets/terminal-libghosty.ico.
# The resulting .syso is committed so builds do not need a resource compiler.
icon:
	$(WINDRES) -O coff --target=pe-x86-64 -i cmd/terminal-libghosty/app.rc -o cmd/terminal-libghosty/rsrc_windows_amd64.syso

test: static
	$(CGO_ENV) go test ./...

build: static
	cmake -E make_directory dist
	$(CGO_ENV) go build -ldflags="-H=windowsgui" -o dist/terminal-libghosty.exe ./cmd/terminal-libghosty
	cmake -E copy_if_different ghostty/LICENSE dist/LICENSE-ghostty
	$(if $(filter Windows_NT,$(OS)),powershell -NoProfile -Command "Remove-Item dist/ghostty-vt.dll -ErrorAction SilentlyContinue",rm -f dist/ghostty-vt.dll)
