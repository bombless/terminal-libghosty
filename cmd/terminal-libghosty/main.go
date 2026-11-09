package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/bombless/terminal-libghosty/internal/gui"
)

func init() {
	runtime.LockOSThread()
}

func main() {
	prepareConsole()

	var target, identity, shell string
	var port int
	var server bool
	var parent string
	flag.BoolVar(&server, "server", false, "run the internal terminal server process")
	flag.StringVar(&parent, "parent", "", "parent window handle for the terminal server")
	flag.StringVar(&target, "ssh", "", "SSH target in user@host[:port] form")
	flag.StringVar(&identity, "identity", "", "SSH private key file")
	flag.IntVar(&port, "port", 0, "SSH port override")
	flag.StringVar(&shell, "shell", "", "start a local shell instead of SSH (pwsh)")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "terminal-libghosty - Windows Direct2D libghostty terminal\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: terminal-libghosty [--ssh user@host[:port]] [--identity keyfile] [--port port]\n")
		fmt.Fprintf(flag.CommandLine.Output(), "       terminal-libghosty --shell pwsh\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if shell != "" && !strings.EqualFold(shell, "pwsh") && !strings.EqualFold(shell, "pwsh.exe") {
		reportFatal("Unsupported shell %q: only pwsh is available", shell)
		os.Exit(1)
	}
	if shell != "" && target != "" {
		reportFatal("--shell and --ssh cannot be combined")
		os.Exit(1)
	}

	if server {
		if err := gui.RunHost(parent, target, identity, port, shell); err != nil {
			reportFatal("Terminal server error: %v", err)
			os.Exit(1)
		}
		return
	}

	if err := gui.RunWithOptions(target, identity, port, shell); err != nil {
		reportFatal("Error starting terminal: %v", err)
		os.Exit(1)
	}
}
