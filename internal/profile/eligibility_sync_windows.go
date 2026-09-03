//go:build windows

package profile

import "os"

func syncEligibilityPublication(_, live string) error {
	file, err := os.Open(live)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}
