package api

import (
	"os"
	"path/filepath"
	"testing"
)

func regWithNames(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "etc"), 0755)
	os.WriteFile(filepath.Join(dir, "etc", "peers.yaml"),
		[]byte("peers:\n  - slot: 0\n    name: lobby\n    key: a\n  - slot: 2\n    name: backpack-02\n    key: b\n"), 0600)
	os.Setenv("VLXsuite_DIR", dir)
}

func TestByID_Name(t *testing.T) {
	regWithNames(t)
	if h, err := ResolvePeerClientHostByID("lobby", "x"); err != nil || h != "10.1.10.2" {
		t.Fatalf("name lobby => %q, %v", h, err)
	}
	if h, err := ResolvePeerClientHostByID("backpack-02", "x"); err != nil || h != "10.1.12.2" {
		t.Fatalf("name backpack-02 => %q, %v", h, err)
	}
	if h, err := ResolvePeerClientHostByID("BACKPACK-02", "x"); err != nil || h != "10.1.12.2" {
		t.Fatalf("case-insensitive => %q, %v", h, err)
	}
	if _, err := ResolvePeerClientHostByID("ghost", "x"); err == nil {
		t.Fatal("unknown name should error")
	}
}

func TestByID_Slot(t *testing.T) {
	regWithNames(t)
	if h, err := ResolvePeerClientHostByID("2", "x"); err != nil || h != "10.1.12.2" {
		t.Fatalf("numeric slot 2 => %q, %v", h, err)
	}
}

func TestByID_NameWithoutRegistry(t *testing.T) {
	os.Setenv("VLXsuite_DIR", t.TempDir()) // no etc/peers.yaml
	if _, err := ResolvePeerClientHostByID("lobby", "x"); err == nil {
		t.Fatal("name without registry should error")
	}
	// numeric slot 0 still works in legacy mode
	if h, err := ResolvePeerClientHostByID("0", "10.9.9.9"); err != nil || h != "10.9.9.9" {
		t.Fatalf("legacy slot0 => %q, %v", h, err)
	}
}
