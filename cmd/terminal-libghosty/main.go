package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bombless/terminal-libghosty/internal/ssh"
)

func main() {
	var target, identity string
	var port int
	flag.StringVar(&target, "ssh", "", "SSH target in user@host form (required)")
	flag.StringVar(&identity, "identity", "", "SSH private key file")
	flag.IntVar(&port, "port", 0, "SSH port override (default: target/default SSH config)")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "terminal-libghosty - Windows libghostty SSH terminal host\\n\\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: terminal-libghosty --ssh user@host[:port] [--identity keyfile] [--port port]\\n\\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if target == "" {
		flag.Usage()
		os.Exit(2)
	}
	args, err := ssh.BuildArgs(ssh.Options{Target: target, IdentityFile: identity, Port: port})
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid SSH session:", err)
		os.Exit(2)
	}
	fmt.Fprintln(os.Stderr, "SSH target validated. Interactive ConPTY/libghostty host is not wired in this bootstrap build.")
	fmt.Fprintf(os.Stderr, "Would launch: ssh")
	for _, arg := range args { fmt.Fprintf(os.Stderr, " %q", arg) }
	fmt.Fprintln(os.Stderr)
	os.Exit(1)
}
