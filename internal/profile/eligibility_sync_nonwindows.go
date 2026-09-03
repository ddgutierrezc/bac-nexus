//go:build !windows

package profile

import "os"

func syncEligibilityPublication(root, _ string) error {
	directory, err := os.Open(root)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
