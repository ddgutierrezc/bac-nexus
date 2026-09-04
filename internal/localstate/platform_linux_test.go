//go:build linux

package localstate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPlatformRejectsSubstitutedManagedFileHandles(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "BAC Nexus", "onboarding-diagnostics")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "ONB-0123456789abcdef0123456789abcdef.json")
	replacement := filepath.Join(directory, "ONB-fedcba9876543210fedcba9876543210.json")
	for _, name := range []string{path, replacement} {
		if err := os.WriteFile(name, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	approved := Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}
	platform := Platform{
		UserConfigDir: func() (string, error) { return root, nil },
		inspect:       func(string, []string) (Evidence, error) { return approved, nil },
		open:          func(string, int, os.FileMode) (*os.File, error) { return os.Open(replacement) },
	}
	for _, flags := range []int{os.O_RDONLY, os.O_WRONLY} {
		if file, err := platform.OpenManagedFile(path, flags, "BAC Nexus", "onboarding-diagnostics", filepath.Base(path)); !errors.Is(err, ErrUnsafePath) || file != nil {
			t.Fatalf("OpenManagedFile(flags=%d) = %v, %v; want substituted identity rejected", flags, file, err)
		}
	}
}

func TestPlatformDoesNotRemoveSubstitutedLink(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "BAC Nexus", "onboarding-diagnostics")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "ONB-0123456789abcdef0123456789abcdef.json")
	target := filepath.Join(directory, "replacement.json")
	if err := os.WriteFile(target, []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	platform := NewPlatform(func() (string, error) { return root, nil })
	if err := platform.RemoveManagedFile(path, "BAC Nexus", "onboarding-diagnostics", filepath.Base(path)); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("RemoveManagedFile() error = %v, want unsafe path", err)
	}
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("substituted link was removed: %v", err)
	}
}
