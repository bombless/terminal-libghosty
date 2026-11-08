package gui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProfileStore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tlg_profile_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	origAppData := os.Getenv("APPDATA")
	os.Setenv("APPDATA", tempDir)
	defer os.Setenv("APPDATA", origAppData)

	profiles := LoadProfiles()
	if len(profiles) != 0 {
		t.Fatalf("expected 0 profiles initially, got %d", len(profiles))
	}

	testProfiles := []Profile{
		{Name: "Server 1", Target: "root@192.168.1.100", Port: 22},
		{Name: "Server 2", Target: "dev@example.com", Port: 2222, Identity: "C:\\keys\\id_rsa"},
	}
	if err := SaveProfiles(testProfiles); err != nil {
		t.Fatalf("SaveProfiles failed: %v", err)
	}

	loaded := LoadProfiles()
	if len(loaded) != 2 {
		t.Fatalf("expected 2 profiles loaded, got %d", len(loaded))
	}
	if loaded[0].Name != "Server 1" || loaded[1].Identity != "C:\\keys\\id_rsa" {
		t.Fatalf("loaded profile mismatch: %+v", loaded)
	}

	storePath := GetStorePath()
	if filepath.Dir(storePath) != filepath.Join(tempDir, "TerminalLibghosty") {
		t.Fatalf("unexpected store path: %s", storePath)
	}
}
