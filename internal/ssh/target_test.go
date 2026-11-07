package ssh

import (
	"reflect"
	"testing"
)

func TestBuildArgs(t *testing.T) {
	got, err := BuildArgs(Options{Target:"dev@example.com:2222", IdentityFile:"C:/keys/id_ed25519"})
	if err != nil { t.Fatal(err) }
	want := []string{"-tt", "-o", "RequestTTY=force", "-o", "StrictHostKeyChecking=yes", "-i", "C:/keys/id_ed25519", "-p", "2222", "--", "dev@example.com"}
	if !reflect.DeepEqual(got, want) { t.Fatalf("got %#v, want %#v", got, want) }
}

func TestRejectUnsafeTargets(t *testing.T) {
	for _, target := range []string{"", "example.com", "-oProxyCommand=bad@host", "u@host;calc", "u@host\\n", "u@host:0", "u@[::1"} {
		if _, err := BuildArgs(Options{Target:target}); err == nil { t.Errorf("accepted unsafe target %q", target) }
	}
}
