package ssh

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestBuildArgs(t *testing.T) {
	got, err := BuildArgs(Options{Target:"dev@example.com:2222", IdentityFile:"C:/keys/id_ed25519"})
	if err != nil { t.Fatal(err) }
	want := []string{"-tt", "-o", "RequestTTY=force", "-o", "StrictHostKeyChecking=yes", "-i", filepath.Clean("C:/keys/id_ed25519"), "-p", "2222", "--", "dev@example.com"}
	if !reflect.DeepEqual(got, want) { t.Fatalf("got %#v, want %#v", got, want) }
}

func TestRejectUnsafeTargets(t *testing.T) {
	for _, target := range []string{"", "example.com", "-oProxyCommand=bad@host", "u@host;calc", "u@host\\n", "u@host\nname", "u@host\n", "u@host\x00name", "u@host:0", "u@host:65536", "u@[::1"} {
		if _, err := BuildArgs(Options{Target:target}); err == nil { t.Errorf("accepted unsafe target %q", target) }
	}
}

func TestBuildArgsAllowsOrdinaryHostCharacters(t *testing.T) {
	got, err := BuildArgs(Options{Target: "runner@node10.example.net"})
	if err != nil { t.Fatalf("valid hostname rejected: %v", err) }
	if got[len(got)-1] != "runner@node10.example.net" { t.Fatalf("unexpected destination: %#v", got) }
}

func TestRejectControlCharactersInIdentityPath(t *testing.T) {
	for _, path := range []string{"key\nfile", "key\rfile", "key\x00file"} {
		if _, err := BuildArgs(Options{Target: "user@example.com", IdentityFile: path}); err == nil {
			t.Errorf("accepted identity path with control character %q", path)
		}
	}
}
