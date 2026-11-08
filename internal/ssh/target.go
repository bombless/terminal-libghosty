package ssh

import (
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var userPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// Options describes one SSH session. Target is user@host or user@host:port.
type Options struct {
	Target       string
	IdentityFile string
	Port         int
}

// BuildArgs returns argv for CreateProcess/exec.Command. It deliberately does
// not invoke a shell and never disables SSH host-key verification.
func BuildArgs(o Options) ([]string, error) {
	if strings.ContainsAny(o.Target, "\r\n\x00") {
		return nil, fmt.Errorf("target must be user@host[:port]")
	}
	target := strings.TrimSpace(o.Target)
	if target == "" || strings.Contains(target, " ") {
		return nil, fmt.Errorf("target must be user@host[:port]")
	}
	parts := strings.Split(target, "@")
	if len(parts) != 2 || !userPattern.MatchString(parts[0]) {
		return nil, fmt.Errorf("target must contain a valid user and host")
	}
	user, hostPort := parts[0], parts[1]
	host := hostPort
	port := o.Port
	if strings.HasPrefix(hostPort, "[") {
		h, p, err := net.SplitHostPort(hostPort)
		if err == nil {
			host = h
			parsed, e := strconv.Atoi(p)
			if e != nil || parsed < 1 || parsed > 65535 {
				return nil, fmt.Errorf("port must be between 1 and 65535")
			}
			if port != 0 && port != parsed {
				return nil, fmt.Errorf("conflicting ports")
			}
			port = parsed
		} else if strings.HasSuffix(hostPort, "]") {
			host = strings.Trim(hostPort, "[]")
		} else {
			return nil, fmt.Errorf("invalid bracketed IPv6 target")
		}
	} else if strings.Count(hostPort, ":") == 1 {
		h, p, err := net.SplitHostPort(hostPort)
		if err != nil {
			return nil, fmt.Errorf("invalid host:port")
		}
		host = h
		parsed, err := strconv.Atoi(p)
		if err != nil || parsed < 1 || parsed > 65535 {
			return nil, fmt.Errorf("port must be between 1 and 65535")
		}
		if port != 0 && port != parsed {
			return nil, fmt.Errorf("conflicting ports")
		}
		port = parsed
	}
	if host == "" || !regexp.MustCompile(`^[a-zA-Z0-9.:-]+$`).MatchString(host) || strings.Contains(host, "@") || strings.HasPrefix(host, "-") {
		return nil, fmt.Errorf("invalid host")
	}
	if strings.Contains(host, "%") {
		return nil, fmt.Errorf("IPv6 zone identifiers are not supported in target syntax")
	}
	if port < 0 || port > 65535 || port == 0 && o.Port != 0 {
		return nil, fmt.Errorf("port must be between 1 and 65535")
	}
	args := []string{"-tt", "-o", "RequestTTY=force", "-o", "StrictHostKeyChecking=yes"}
	if o.IdentityFile != "" {
		if strings.ContainsAny(o.IdentityFile, "\r\n\x00") {
			return nil, fmt.Errorf("invalid identity file path")
		}
		args = append(args, "-i", filepath.Clean(o.IdentityFile))
	}
	if port != 0 {
		args = append(args, "-p", strconv.Itoa(port))
	}
	args = append(args, "--", user+"@"+host)
	return args, nil
}

// ParseTarget validates the authority without accepting URL schemes or shell syntax.
func ParseTarget(s string) error { _, err := BuildArgs(Options{Target: s}); return err }

// Keep url imported in this package's API docs explicit: URL-form targets are rejected.
