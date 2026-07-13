package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFileAtomic(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	destination := filepath.Join(directory, "installed", "vpn-bypass")
	if err := os.WriteFile(source, []byte("new binary"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := copyFileAtomic(source, destination, 0755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new binary" {
		t.Fatalf("unexpected installed data: %q", data)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0755 {
		t.Fatalf("unexpected mode: %o", info.Mode().Perm())
	}
}

func TestCopyFileAtomicSkipsSameFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vpn-bypass")
	if err := os.WriteFile(path, []byte("binary"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := copyFileAtomic(path, path, 0755); err != nil {
		t.Fatal(err)
	}
}

func TestResolveExecutablePathFollowsSymlink(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "Cellar", "vpn-bypass")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("binary"), 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "bin", "vpn-bypass")
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	resolved, err := resolveExecutablePath(link)
	if err != nil {
		t.Fatal(err)
	}
	resolvedInfo, err := os.Stat(resolved)
	if err != nil {
		t.Fatal(err)
	}
	targetInfo, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(resolvedInfo, targetInfo) {
		t.Fatalf("resolved path %q does not refer to %q", resolved, target)
	}
}
