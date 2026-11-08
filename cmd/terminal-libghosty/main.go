package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/bombless/terminal-libghosty/internal/gui"
)

func init() {
	runtime.LockOSThread()
}

func main() {
	var target, identity string
	var port int
	var server bool
	var parent string
	flag.BoolVar(&server, "server", false, "run the internal terminal server process")
	flag.StringVar(&parent, "parent", "", "parent window handle for the terminal server")
	flag.StringVar(&target, "ssh", "", "SSH target in user@host[:port] form")
	flag.StringVar(&identity, "identity", "", "SSH private key file")
	flag.IntVar(&port, "port", 0, "SSH port override")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "terminal-libghosty - Windows Direct2D libghostty SSH terminal\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: terminal-libghosty [--ssh user@host[:port]] [--identity keyfile] [--port port]\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if server {
		if err := gui.RunHost(parent, target, identity, port); err != nil {
			fmt.Fprintf(os.Stderr, "Terminal server error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := gui.RunWithOptions(target, identity, port); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting terminal: %v\n", err)
		os.Exit(1)
	}
}
