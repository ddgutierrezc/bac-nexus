//go:build !linux && !windows

package localstate

func inspectManagedDirectory(string, []string) (Evidence, error) {
	return Evidence{}, ErrUnsafePath
}

func createManagedFile(string) (Evidence, error) { return Evidence{}, ErrUnsafePath }
