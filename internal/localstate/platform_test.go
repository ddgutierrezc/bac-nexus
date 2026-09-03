package localstate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPlatformRejectsUnsafeOrContradictoryEvidence(t *testing.T) {
	root := filepath.Join(t.TempDir(), "config")
	for _, tc := range []struct {
		name     string
		evidence Evidence
	}{
		{"symlink", Evidence{Available: true, LinkSafe: false, Local: true, Owned: true, Restrictive: true}},
		{"race changed handle", Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: false}},
		{"unsupported", Evidence{}},
		{"contradictory locality", Evidence{Available: true, LinkSafe: true, Local: false, Owned: true, Restrictive: true, HandleStable: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			platform := Platform{UserConfigDir: func() (string, error) { return root, nil }, inspect: func(string, []string) (Evidence, error) { return tc.evidence, nil }}
			if _, err := platform.VerifyManagedDirectory(filepath.Join(root, "BAC Nexus", "ownership"), "BAC Nexus", "ownership"); !errors.Is(err, ErrUnsafePath) {
				t.Fatalf("VerifyManagedDirectory() error = %v, want %v", err, ErrUnsafePath)
			}
		})
	}
}

func TestPlatformCreatesAndReinspectsRestrictiveManagedDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	platform := NewPlatform(func() (string, error) { return root, nil })
	path := filepath.Join(root, "BAC Nexus", "ownership")
	evidence, err := platform.VerifyManagedDirectory(path, "BAC Nexus", "ownership")
	if err != nil {
		t.Fatalf("VerifyManagedDirectory() error = %v", err)
	}
	if !evidence.approved() {
		t.Fatalf("evidence = %#v, want complete native evidence", evidence)
	}
	for _, directory := range []string{filepath.Join(root, "BAC Nexus"), path} {
		info, err := os.Stat(directory)
		if err != nil || info.Mode().Perm() != 0o700 {
			t.Fatalf("managed directory %q = %v, %v; want mode 0700", directory, info, err)
		}
	}
}

func TestPlatformCreatesAndReinspectsRestrictiveManagedFile(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	platform := NewPlatform(func() (string, error) { return root, nil })
	path := filepath.Join(root, "BAC Nexus", "ownership", "ownership.db")
	evidence, err := platform.CreateManagedFile(path, "BAC Nexus", "ownership", "ownership.db")
	if err != nil {
		t.Fatalf("CreateManagedFile() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 || !evidence.approved() {
		t.Fatalf("managed file = %v, %v, %#v; want regular mode 0600 with complete evidence", info, err, evidence)
	}
}

func TestPlatformRejectsPathsOutsideExactManagedComponents(t *testing.T) {
	root := t.TempDir()
	platform := Platform{UserConfigDir: func() (string, error) { return root, nil }, inspect: func(string, []string) (Evidence, error) {
		return Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}, nil
	}}
	if _, err := platform.VerifyManagedDirectory(filepath.Join(root, "other", "ownership"), "BAC Nexus", "ownership"); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("outside managed components error = %v, want %v", err, ErrUnsafePath)
	}
}
