package mapepirestdio

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestJARDiscovery(t *testing.T) {
	tests := []struct {
		name         string
		arrange      func(*testing.T, string) (func(string) error, func(string) (string, error))
		wantStatus   DiscoveryStatus
		wantVerified int
		wantRejected int
		wantPath     bool
	}{
		{
			name: "unique valid candidate",
			arrange: func(t *testing.T, root string) (func(string) error, func(string) (string, error)) {
				writeDiscoveryCandidate(t, root, codeForIBMiExtensionDirectory, serverJARRelativePath)
				return verifyDiscoveryFixture, nil
			},
			wantStatus: DiscoveryFound, wantVerified: 1, wantPath: true,
		},
		{
			name: "no candidate",
			arrange: func(*testing.T, string) (func(string) error, func(string) (string, error)) {
				return verifyDiscoveryFixture, nil
			},
			wantStatus: DiscoveryNotFound,
		},
		{
			name: "exact candidate with bad hash",
			arrange: func(t *testing.T, root string) (func(string) error, func(string) (string, error)) {
				writeDiscoveryCandidate(t, root, codeForIBMiExtensionDirectory, serverJARRelativePath)
				return func(string) error { return errors.New("checksum mismatch") }, nil
			},
			wantStatus: DiscoveryNotFound, wantRejected: 1,
		},
		{
			name: "exact candidate unreadable by verifier",
			arrange: func(t *testing.T, root string) (func(string) error, func(string) (string, error)) {
				writeDiscoveryCandidate(t, root, codeForIBMiExtensionDirectory, serverJARRelativePath)
				return func(string) error { return os.ErrPermission }, nil
			},
			wantStatus: DiscoveryNotFound, wantRejected: 1,
		},
		{
			name: "unrelated publishers versions and paths are ignored",
			arrange: func(t *testing.T, root string) (func(string) error, func(string) (string, error)) {
				writeDiscoveryCandidate(t, root, "other.code-for-ibmi-3.0.12", serverJARRelativePath)
				writeDiscoveryCandidate(t, root, "halcyontechltd.code-for-ibmi-3.0.11", serverJARRelativePath)
				writeDiscoveryCandidate(t, root, codeForIBMiExtensionDirectory, "other/mapepire-server-2.3.5.jar")
				writeDiscoveryCandidate(t, root, codeForIBMiExtensionDirectory+"-malicious", serverJARRelativePath)
				return verifyDiscoveryFixture, nil
			},
			wantStatus: DiscoveryNotFound, wantRejected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			verify, canonicalize := tt.arrange(t, root)
			result := (JARDiscovery{ExtensionsRoot: root, Verify: verify, canonicalize: canonicalize}).Discover()
			if result.Status != tt.wantStatus || result.VerifiedCandidateCount != tt.wantVerified || result.RejectedCandidateCount != tt.wantRejected {
				t.Fatalf("result = %#v", result)
			}
			if (result.Path != "") != tt.wantPath {
				t.Fatalf("path presence = %v, want %v", result.Path != "", tt.wantPath)
			}
			if result.Path != "" && !filepath.IsAbs(result.Path) {
				t.Fatalf("discovered path is not absolute: %q", result.Path)
			}
		})
	}
}

func TestJARDiscoveryMatchesOnlyExactExtensionDirectory(t *testing.T) {
	spoofs := []string{
		codeForIBMiExtensionDirectory + "-malicious",
		"halcyontechltd.code-for-ibmi-3.0.120",
		"Halcyontechltd.code-for-ibmi-3.0.12",
		codeForIBMiExtensionDirectory + "-universal",
		codeForIBMiExtensionDirectory + ".suffix",
	}
	for _, spoof := range spoofs {
		t.Run(spoof, func(t *testing.T) {
			root := t.TempDir()
			writeDiscoveryCandidate(t, root, spoof, serverJARRelativePath)
			result := (JARDiscovery{ExtensionsRoot: root, Verify: verifyDiscoveryFixture}).Discover()
			if result.Status != DiscoveryNotFound || result.VerifiedCandidateCount != 0 || result.RejectedCandidateCount != 0 || result.InspectionFailed {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func TestJARDiscoveryRejectsLinkedTraversal(t *testing.T) {
	t.Run("extensions root link", func(t *testing.T) {
		parent := t.TempDir()
		target := t.TempDir()
		writeDiscoveryCandidate(t, target, codeForIBMiExtensionDirectory, serverJARRelativePath)
		root := filepath.Join(parent, "extensions")
		if err := os.Symlink(target, root); err != nil {
			t.Skipf("directory symlink creation is unavailable: %v", err)
		}
		result := (JARDiscovery{ExtensionsRoot: root, Verify: verifyDiscoveryFixture}).Discover()
		if result.Status != DiscoveryNotFound || !result.InspectionFailed {
			t.Fatalf("result = %#v", result)
		}
	})

	t.Run("matched extension directory link", func(t *testing.T) {
		root := t.TempDir()
		target := t.TempDir()
		writeDiscoveryCandidate(t, target, "", serverJARRelativePath)
		extension := filepath.Join(root, codeForIBMiExtensionDirectory)
		if err := os.Symlink(target, extension); err != nil {
			t.Skipf("directory symlink creation is unavailable: %v", err)
		}
		result := (JARDiscovery{ExtensionsRoot: root, Verify: verifyDiscoveryFixture}).Discover()
		if result.Status != DiscoveryNotFound || !result.InspectionFailed {
			t.Fatalf("result = %#v", result)
		}
	})
}

func TestJARDiscoveryRejectsInjectedLinkedPathComponents(t *testing.T) {
	t.Run("extensions root ancestor", func(t *testing.T) {
		parent := t.TempDir()
		root := filepath.Join(parent, "extensions")
		if err := os.Mkdir(root, 0o700); err != nil {
			t.Fatal(err)
		}
		linked := filepath.Clean(parent)
		result := (JARDiscovery{
			ExtensionsRoot: root,
			Verify:         verifyDiscoveryFixture,
			lstat:          injectedLinkedLstat(t, linked),
		}).Discover()
		if result.Status != DiscoveryNotFound || !result.InspectionFailed {
			t.Fatalf("result = %#v", result)
		}
	})

	t.Run("matched extension directory", func(t *testing.T) {
		root := t.TempDir()
		extension := filepath.Join(root, codeForIBMiExtensionDirectory)
		writeDiscoveryCandidate(t, root, codeForIBMiExtensionDirectory, serverJARRelativePath)
		verifyCalled := false
		result := (JARDiscovery{
			ExtensionsRoot: root,
			Verify: func(string) error {
				verifyCalled = true
				return nil
			},
			lstat: injectedLinkedLstat(t, extension),
		}).Discover()
		if result.Status != DiscoveryNotFound || !result.InspectionFailed || verifyCalled {
			t.Fatalf("result = %#v, verifyCalled = %v", result, verifyCalled)
		}
	})
}

func TestJARDiscoveryRejectsNonregularAndSymlinkCandidates(t *testing.T) {
	t.Run("nonregular", func(t *testing.T) {
		root := t.TempDir()
		candidate := filepath.Join(root, codeForIBMiExtensionDirectory, filepath.FromSlash(serverJARRelativePath))
		if err := os.MkdirAll(candidate, 0o700); err != nil {
			t.Fatal(err)
		}
		result := (JARDiscovery{ExtensionsRoot: root, Verify: verifyDiscoveryFixture}).Discover()
		if result.Status != DiscoveryNotFound || result.RejectedCandidateCount != 1 {
			t.Fatalf("result = %#v", result)
		}
	})

	t.Run("symlink", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "target.jar")
		if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
		candidate := filepath.Join(root, codeForIBMiExtensionDirectory, filepath.FromSlash(serverJARRelativePath))
		if err := os.MkdirAll(filepath.Dir(candidate), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, candidate); err != nil {
			t.Skipf("symlink creation is unavailable: %v", err)
		}
		result := (JARDiscovery{ExtensionsRoot: root, Verify: verifyDiscoveryFixture}).Discover()
		if result.Status != DiscoveryNotFound || result.RejectedCandidateCount != 1 {
			t.Fatalf("result = %#v", result)
		}
	})
}

func TestJARDiscoveryClassifiesRootFailures(t *testing.T) {
	missing := (JARDiscovery{ExtensionsRoot: filepath.Join(t.TempDir(), "missing"), Verify: verifyDiscoveryFixture}).Discover()
	if missing.Status != DiscoveryNotFound || missing.InspectionFailed {
		t.Fatalf("missing root = %#v", missing)
	}
	invalid := (JARDiscovery{ExtensionsRoot: "", Verify: verifyDiscoveryFixture}).Discover()
	if invalid.Status != DiscoveryNotFound || !invalid.InspectionFailed {
		t.Fatalf("invalid discovery = %#v", invalid)
	}
}

func writeDiscoveryCandidate(t *testing.T, root, extension, relative string) string {
	t.Helper()
	path := filepath.Join(root, extension, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func verifyDiscoveryFixture(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("not a regular non-link file")
	}
	return nil
}

type linkedFileInfo struct{ os.FileInfo }

func (info linkedFileInfo) Mode() os.FileMode { return info.FileInfo.Mode() | os.ModeSymlink }

func injectedLinkedLstat(t *testing.T, linked string) func(string) (os.FileInfo, error) {
	t.Helper()
	return func(path string) (os.FileInfo, error) {
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		if filepath.Clean(path) == filepath.Clean(linked) {
			return linkedFileInfo{FileInfo: info}, nil
		}
		return info, nil
	}
}
