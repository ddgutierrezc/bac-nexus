package configuration

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBundleArtifactResolverAcceptsOnlyCanonicalTopology(t *testing.T) {
	_, executable, jar := bundleTopology(t, runtime.GOOS, runtime.GOARCH)
	resolver := bundleArtifactResolver{executable: func() (string, error) { return executable, nil }, goos: runtime.GOOS, goarch: runtime.GOARCH}
	got, err := resolver.Resolve()
	if err != nil || got != jar {
		t.Fatalf("Resolve() = %q, %v; want %q", got, err, jar)
	}
}

func TestBundleArtifactResolverRejectsUnsafeTopology(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, string, string, string) string
		goos   string
	}{
		{name: "relative executable", goos: runtime.GOOS, mutate: func(_ *testing.T, _, executable, _ string) string { return filepath.Base(executable) }},
		{name: "wrong platform", goos: "other", mutate: func(_ *testing.T, _ string, executable string, _ string) string { return executable }},
		{name: "wrong executable", goos: runtime.GOOS, mutate: func(_ *testing.T, _ string, executable string, _ string) string {
			return filepath.Join(filepath.Dir(executable), "other")
		}},
		{name: "missing jar", goos: runtime.GOOS, mutate: func(t *testing.T, _ string, executable, jar string) string { remove(t, jar); return executable }},
		{name: "jar symlink", goos: runtime.GOOS, mutate: func(t *testing.T, root, executable, jar string) string {
			remove(t, jar)
			symlink(t, filepath.Join(root, "target"), jar)
			return executable
		}},
		{name: "component symlink", goos: runtime.GOOS, mutate: func(t *testing.T, root, executable, _ string) string {
			components := filepath.Join(root, "components")
			rename(t, components, filepath.Join(root, "target"))
			symlink(t, filepath.Join(root, "target"), components)
			return executable
		}},
		{name: "executable symlink", goos: runtime.GOOS, mutate: func(t *testing.T, root, executable, _ string) string {
			rename(t, executable, filepath.Join(root, "target"))
			symlink(t, filepath.Join(root, "target"), executable)
			return executable
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, executable, jar := bundleTopology(t, runtime.GOOS, runtime.GOARCH)
			executable = tt.mutate(t, root, executable, jar)
			resolver := bundleArtifactResolver{executable: func() (string, error) { return executable, nil }, goos: tt.goos, goarch: runtime.GOARCH}
			if _, err := resolver.Resolve(); err == nil {
				t.Fatal("unsafe topology was accepted")
			}
		})
	}
}

func TestBundleArtifactResolverRejectsRootSymlink(t *testing.T) {
	root, executable, _ := bundleTopology(t, runtime.GOOS, runtime.GOARCH)
	linkedRoot := root + "-link"
	if err := os.Symlink(root, linkedRoot); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}
	executable = strings.Replace(executable, root, linkedRoot, 1)
	resolver := bundleArtifactResolver{executable: func() (string, error) { return executable, nil }, goos: runtime.GOOS, goarch: runtime.GOARCH}
	if _, err := resolver.Resolve(); err == nil {
		t.Fatal("root symlink was accepted")
	}
}

func bundleTopology(t *testing.T, goos, goarch string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	platform := filepath.Join(root, "platforms", goos+"-"+goarch)
	if err := os.MkdirAll(filepath.Join(root, "components", "mapepire", "2.3.6"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(platform, 0o700); err != nil {
		t.Fatal(err)
	}
	name := "nexus"
	if goos == "windows" {
		name += ".exe"
	}
	executable := filepath.Join(platform, name)
	jar := filepath.Join(root, "components", "mapepire", "2.3.6", "mapepire-server.jar")
	for _, path := range []string{executable, jar} {
		if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root, executable, jar
}

func remove(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

func rename(t *testing.T, old, new string) {
	t.Helper()
	if err := os.Rename(old, new); err != nil {
		t.Fatal(err)
	}
}

func symlink(t *testing.T, old, new string) {
	t.Helper()
	if err := os.Symlink(old, new); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}
}
