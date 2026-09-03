// Package localstate verifies the small set of local directories that BAC
// Nexus owns. It intentionally accepts explicit managed components rather
// than arbitrary filesystem paths.
package localstate

import (
	"errors"
	"path/filepath"
)

var ErrUnsafePath = errors.New("local state path is unsafe or unsupported")

// Evidence is the complete, fail-closed result of inspecting a managed path.
// A platform must prove every field before a caller may create local state.
type Evidence struct {
	Available, LinkSafe, Local, Owned, Restrictive, HandleStable bool
}

func (e Evidence) approved() bool {
	return e.Available && e.LinkSafe && e.Local && e.Owned && e.Restrictive && e.HandleStable
}

// SecurePathPlatform is the injectable boundary shared by durable local-state
// users. Native implementations inspect component handles; tests supply small
// fakes to prove fail-closed policy without claiming another OS's evidence.
type SecurePathPlatform interface {
	VerifyManagedDirectory(path string, components ...string) (Evidence, error)
	CreateManagedFile(path string, components ...string) (Evidence, error)
}

// CreateManagedFile creates one explicitly named file below an already
// verified managed directory and immediately reinspects its handle.
func (p Platform) CreateManagedFile(path string, components ...string) (Evidence, error) {
	if len(components) < 2 {
		return Evidence{}, ErrUnsafePath
	}
	file := components[len(components)-1]
	directories := components[:len(components)-1]
	if file == "" || file == "." || file == ".." || filepath.Base(file) != file {
		return Evidence{}, ErrUnsafePath
	}
	directory := filepath.Dir(path)
	if _, err := p.VerifyManagedDirectory(directory, directories...); err != nil {
		return Evidence{}, ErrUnsafePath
	}
	evidence, err := createManagedFile(path)
	if err != nil || !evidence.approved() {
		return Evidence{}, ErrUnsafePath
	}
	return evidence, nil
}

// Platform verifies a path below the current user's configuration root. The
// unexported inspector exists only to make platform-contract fakes deterministic.
type Platform struct {
	UserConfigDir func() (string, error)
	inspect       func(string, []string) (Evidence, error)
}

func NewPlatform(userConfigDir func() (string, error)) Platform {
	return Platform{UserConfigDir: userConfigDir}
}

func (p Platform) VerifyManagedDirectory(path string, components ...string) (Evidence, error) {
	if p.UserConfigDir == nil || len(components) == 0 {
		return Evidence{}, ErrUnsafePath
	}
	root, err := p.UserConfigDir()
	if err != nil || root == "" || !filepath.IsAbs(root) || !filepath.IsAbs(path) {
		return Evidence{}, ErrUnsafePath
	}
	for _, component := range components {
		if component == "" || component == "." || component == ".." || filepath.Base(component) != component {
			return Evidence{}, ErrUnsafePath
		}
	}
	want := filepath.Join(append([]string{root}, components...)...)
	if filepath.Clean(path) != filepath.Clean(want) {
		return Evidence{}, ErrUnsafePath
	}
	inspect := p.inspect
	if inspect == nil {
		inspect = inspectManagedDirectory
	}
	evidence, err := inspect(root, components)
	if err != nil || !evidence.approved() {
		return Evidence{}, ErrUnsafePath
	}
	return evidence, nil
}
