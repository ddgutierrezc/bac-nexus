// Package localstate verifies the small set of local directories that BAC
// Nexus owns. It intentionally accepts explicit managed components rather
// than arbitrary filesystem paths.
package localstate

import (
	"errors"
	"os"
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

// ManagedFilePlatform is the explicit diagnostics file boundary. Opened
// handles are approved against their managed path before callers can use them.
type ManagedFilePlatform interface {
	SecurePathPlatform
	OpenManagedFile(path string, flags int, components ...string) (*os.File, error)
	RemoveManagedFile(path string, components ...string) error
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
	open          func(string, int, os.FileMode) (*os.File, error)
}

func NewPlatform(userConfigDir func() (string, error)) Platform {
	return Platform{UserConfigDir: userConfigDir}
}

var _ ManagedFilePlatform = Platform{}

// OpenManagedFile opens an existing file without creation or truncation and
// binds native security approval to the returned handle.
func (p Platform) OpenManagedFile(path string, flags int, components ...string) (*os.File, error) {
	if flags&(os.O_CREATE|os.O_EXCL|os.O_TRUNC) != 0 || flags&^(os.O_WRONLY|os.O_RDWR|os.O_APPEND) != 0 {
		return nil, ErrUnsafePath
	}
	if len(components) < 2 {
		return nil, ErrUnsafePath
	}
	file := components[len(components)-1]
	directories := components[:len(components)-1]
	if file == "" || file == "." || file == ".." || filepath.Base(file) != file {
		return nil, ErrUnsafePath
	}
	directory := filepath.Dir(path)
	if _, err := p.VerifyManagedDirectory(directory, directories...); err != nil {
		return nil, ErrUnsafePath
	}
	open := p.open
	if open == nil {
		open = os.OpenFile
	}
	handle, err := open(path, flags, 0)
	if err != nil {
		return nil, ErrUnsafePath
	}
	if evidence, verifyErr := verifyManagedOpenFile(path, handle); verifyErr != nil || !evidence.approved() {
		_ = handle.Close()
		return nil, ErrUnsafePath
	}
	return handle, nil
}

// RemoveManagedFile removes only an existing file admitted by the native
// managed-path implementation; it is not an arbitrary deletion primitive.
func (p Platform) RemoveManagedFile(path string, components ...string) error {
	if len(components) < 2 {
		return ErrUnsafePath
	}
	file := components[len(components)-1]
	directories := components[:len(components)-1]
	if file == "" || file == "." || file == ".." || filepath.Base(file) != file {
		return ErrUnsafePath
	}
	if _, err := p.VerifyManagedDirectory(filepath.Dir(path), directories...); err != nil {
		return ErrUnsafePath
	}
	return removeManagedFile(path, file)
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
