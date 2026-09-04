package configuration

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"bac-nexus/internal/connectors/ibmi/mapepirestdio"
)

// bundleArtifactResolver accepts only the executable's fixed bundle topology.
// It deliberately does not inspect the current directory, environment, or profile.
type bundleArtifactResolver struct {
	executable func() (string, error)
	goos       string
	goarch     string
}

func newBundleArtifactResolver() bundleArtifactResolver {
	return bundleArtifactResolver{executable: os.Executable, goos: runtime.GOOS, goarch: runtime.GOARCH}
}

func (r bundleArtifactResolver) Resolve() (string, error) {
	if r.executable == nil || r.goos == "" || r.goarch == "" {
		return "", errors.New("bundle artifact resolver is unavailable")
	}
	executable, err := r.executable()
	if err != nil || !filepath.IsAbs(executable) || filepath.Clean(executable) != executable {
		return "", errors.New("Nexus executable path is unsafe")
	}
	name := "nexus"
	if r.goos == "windows" {
		name += ".exe"
	}
	platform := r.goos + "-" + r.goarch
	if filepath.Base(executable) != name || filepath.Base(filepath.Dir(executable)) != platform || filepath.Base(filepath.Dir(filepath.Dir(executable))) != "platforms" {
		return "", errors.New("Nexus executable is outside the bundle topology")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(executable)))
	if err := requireBundleDirectory(root); err != nil {
		return "", err
	}
	for _, path := range []string{
		filepath.Join(root, "platforms"),
		filepath.Join(root, "platforms", platform),
		filepath.Join(root, "components"),
		filepath.Join(root, "components", "mapepire"),
		filepath.Join(root, "components", "mapepire", mapepirestdio.ServerVersion),
	} {
		if err := requireBundleDirectory(path); err != nil {
			return "", err
		}
	}
	if err := requireBundleFile(executable); err != nil {
		return "", err
	}
	jar := filepath.Join(root, "components", "mapepire", mapepirestdio.ServerVersion, "mapepire-server.jar")
	if err := requireBundleFile(jar); err != nil {
		return "", err
	}
	return jar, nil
}

func requireBundleDirectory(path string) error {
	if !bundlePathApproved(path, true) {
		return errors.New("Nexus bundle directory is unsafe")
	}
	return nil
}

func requireBundleFile(path string) error {
	if !bundlePathApproved(path, false) {
		return errors.New("Nexus bundle file is unsafe")
	}
	return nil
}
