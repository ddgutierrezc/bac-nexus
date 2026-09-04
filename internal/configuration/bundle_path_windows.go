//go:build windows

package configuration

import (
	"io/fs"
	"os"

	"golang.org/x/sys/windows"
)

// bundleWindowsPathEvidence keeps the native attribute lookup injectable only
// for deterministic Windows tests; production creates an immutable value.
type bundleWindowsPathEvidence struct {
	attributes func(*uint16) (uint32, error)
}

func bundlePathApproved(path string, directory bool) bool {
	return bundleWindowsPathEvidence{attributes: windows.GetFileAttributes}.approved(path, directory)
}

func (e bundleWindowsPathEvidence) approved(path string, directory bool) bool {
	before, err := os.Lstat(path)
	if err != nil || e.attributes == nil || before.Mode()&fs.ModeSymlink != 0 || before.Mode()&fs.ModeIrregular != 0 || (directory && !before.IsDir()) || (!directory && !before.Mode().IsRegular()) {
		return false
	}
	wide, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attributes, err := e.attributes(wide)
	if err != nil || attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || (directory && attributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0) {
		return false
	}
	after, err := os.Lstat(path)
	return err == nil && os.SameFile(before, after)
}
