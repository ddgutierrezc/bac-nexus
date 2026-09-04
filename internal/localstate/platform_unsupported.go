//go:build !linux && !windows

package localstate

import "os"

func inspectManagedDirectory(string, []string) (Evidence, error) {
	return Evidence{}, ErrUnsafePath
}

func createManagedFile(string) (Evidence, error) { return Evidence{}, ErrUnsafePath }

func verifyManagedOpenFile(string, *os.File) (Evidence, error) { return Evidence{}, ErrUnsafePath }

func removeManagedFile(string, string) error { return ErrUnsafePath }
