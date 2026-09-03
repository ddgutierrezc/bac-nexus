package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"bac-nexus/internal/release"
)

func TestRunCreatesAndVerifiesReleaseManifest(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join("build", "v1-mcp-foundation", "v1.2.3", "linux-amd64", "nexus")
	manifest := filepath.Join("build", "v1-mcp-foundation", "v1.2.3", "linux-amd64", "nexus.manifest.json")
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(path)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, path), []byte("nexus binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	if err := run([]string{"-binary", path, "-manifest", manifest, "-version", "v1.2.3", "-revision", "abc123", "-goos", "linux", "-goarch", "amd64"}); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var got release.Manifest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if err := release.VerifyManifest(got, path, []byte("nexus binary"), release.Identity{Version: "v1.2.3", Revision: "abc123"}); err != nil {
		t.Fatalf("VerifyManifest() error = %v", err)
	}
}

func TestRunRejectsIncompleteIdentity(t *testing.T) {
	if err := run([]string{"-binary", "build/v1-mcp-foundation/v1.2.3/linux-amd64/nexus"}); err == nil {
		t.Fatal("run() error = nil, want incomplete identity rejection")
	}
}

func TestRunRejectsManifestOutsideApprovedReleasePath(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join("build", "v1-mcp-foundation", "v1.2.3", "linux-amd64", "nexus")
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(path)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, path), []byte("nexus binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	if err := run([]string{"-binary", path, "-manifest", "handoff.json", "-version", "v1.2.3", "-revision", "abc123", "-goos", "linux", "-goarch", "amd64"}); err == nil {
		t.Fatal("run() error = nil, want manifest path rejection")
	}
}

func TestRunRejectsUnapprovedIdentityBeforeReadingOrWriting(t *testing.T) {
	root := t.TempDir()
	validBinary := filepath.Join("build", "v1-mcp-foundation", "v1.2.3", "linux-amd64", "nexus")
	validManifest := filepath.Join("build", "v1-mcp-foundation", "v1.2.3", "linux-amd64", "nexus.manifest.json")
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(validBinary)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, validBinary), []byte("nexus binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	previousReadFile := readFile
	previousWriteFile := writeFile
	t.Cleanup(func() {
		readFile = previousReadFile
		writeFile = previousWriteFile
	})

	tests := []struct {
		name     string
		binary   string
		version  string
		revision string
		goos     string
		goarch   string
	}{
		{name: "alternate binary", binary: validBinary + "-copy", version: "v1.2.3", revision: "abc123", goos: "linux", goarch: "amd64"},
		{name: "version traversal", binary: validBinary, version: "v1.2.3/../escape", revision: "abc123", goos: "linux", goarch: "amd64"},
		{name: "empty revision", binary: validBinary, version: "v1.2.3", goos: "linux", goarch: "amd64"},
		{name: "revision newline", binary: validBinary, version: "v1.2.3", revision: "abc123\n", goos: "linux", goarch: "amd64"},
		{name: "revision control", binary: validBinary, version: "v1.2.3", revision: "abc123\x00", goos: "linux", goarch: "amd64"},
		{name: "revision whitespace", binary: validBinary, version: "v1.2.3", revision: "abc 123", goos: "linux", goarch: "amd64"},
		{name: "revision separator", binary: validBinary, version: "v1.2.3", revision: "refs/heads/main", goos: "linux", goarch: "amd64"},
		{name: "revision traversal", binary: validBinary, version: "v1.2.3", revision: "abc/../123", goos: "linux", goarch: "amd64"},
		{name: "GOOS traversal", binary: validBinary, version: "v1.2.3", revision: "abc123", goos: "linux/..", goarch: "amd64"},
		{name: "GOARCH control", binary: validBinary, version: "v1.2.3", revision: "abc123", goos: "linux", goarch: "amd64\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readCalls, writeCalls := 0, 0
			readFile = func(path string) ([]byte, error) {
				readCalls++
				return previousReadFile(path)
			}
			writeFile = func(path string, data []byte, mode os.FileMode) error {
				writeCalls++
				return previousWriteFile(path, data, mode)
			}
			err := run([]string{"-binary", tt.binary, "-manifest", validManifest, "-version", tt.version, "-revision", tt.revision, "-goos", tt.goos, "-goarch", tt.goarch})
			if !errors.Is(err, errInvalidReleaseIdentity) {
				t.Fatalf("run() error = %v, want invalid release identity", err)
			}
			if readCalls != 0 || writeCalls != 0 {
				t.Fatalf("rejected identity invoked read=%d write=%d, want zero output mutation", readCalls, writeCalls)
			}
			if _, err := os.Stat(validManifest); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("manifest exists or stat failed after rejected identity: %v", err)
			}
		})
	}
}

func TestRunSanitizesFilesystemFailures(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join("build", "v1-mcp-foundation", "v1.2.3", "linux-amd64", "nexus")
	manifest := filepath.Join("build", "v1-mcp-foundation", "v1.2.3", "linux-amd64", "nexus.manifest.json")
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(binary)), 0o755); err != nil {
		t.Fatal(err)
	}

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	args := []string{"-binary", binary, "-manifest", manifest, "-version", "v1.2.3", "-revision", "abc123", "-goos", "linux", "-goarch", "amd64"}
	if err := run(args); !errors.Is(err, errReleaseBinaryUnavailable) || err.Error() != errReleaseBinaryUnavailable.Error() {
		t.Fatalf("run() binary read error = %v, want stable %q", err, errReleaseBinaryUnavailable)
	}

	if err := os.WriteFile(binary, []byte("nexus binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(manifest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := run(args); !errors.Is(err, errReleaseManifestUnavailable) || err.Error() != errReleaseManifestUnavailable.Error() {
		t.Fatalf("run() manifest write error = %v, want stable %q", err, errReleaseManifestUnavailable)
	}
}
