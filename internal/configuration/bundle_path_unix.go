//go:build !windows

package configuration

import (
	"io/fs"
	"os"
)

func bundlePathApproved(path string, directory bool) bool {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&fs.ModeSymlink != 0 || info.Mode()&fs.ModeIrregular != 0 {
		return false
	}
	if directory {
		return info.IsDir()
	}
	return info.Mode().IsRegular()
}
