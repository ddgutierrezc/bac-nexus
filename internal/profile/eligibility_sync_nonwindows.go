//go:build !windows

package profile

import "os"

func syncEligibilityPublication(root, _ string) error {
	directory, err := os.Open(root)
	if err != nil {
		return err
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return err
	}
	return directory.Close()
}

func removeEligibility(root, path string) error {
	if err := os.Remove(path); err != nil {
		return err
	}
	return syncEligibilityPublication(root, path)
}
