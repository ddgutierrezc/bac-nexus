//go:build windows

package localstate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlatformVerifiesExistingWindowsWritableFileByNativeEvidence(t *testing.T) {
	root := t.TempDir()
	platform := NewPlatform(func() (string, error) { return root, nil })
	path := filepath.Join(root, "BAC Nexus", "onboarding-diagnostics", "ONB-0123456789abcdef0123456789abcdef.json")
	if _, err := platform.CreateManagedFile(path, "BAC Nexus", "onboarding-diagnostics", filepath.Base(path)); err != nil {
		t.Fatalf("CreateManagedFile() error = %v", err)
	}
	if err := os.Chmod(path, 0o666); err != nil {
		t.Fatal(err)
	}
	file, err := platform.OpenManagedFile(path, os.O_RDONLY, "BAC Nexus", "onboarding-diagnostics", filepath.Base(path))
	if err != nil {
		t.Fatalf("OpenManagedFile() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
