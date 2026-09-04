//go:build windows

package configuration

import (
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

// This intentionally does not create a junction: that requires Windows
// privileges not guaranteed on hosted runners. The real API is exercised on
// ordinary paths and the injected reparse bit proves the rejection branch.
func TestBundleWindowsPathEvidence(t *testing.T) {
	directory := t.TempDir()
	file, err := os.CreateTemp(directory, "artifact")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	real := bundleWindowsPathEvidence{attributes: windows.GetFileAttributes}
	if !real.approved(directory, true) || !real.approved(file.Name(), false) {
		t.Fatal("ordinary Windows components were rejected")
	}
	reparse := bundleWindowsPathEvidence{attributes: func(*uint16) (uint32, error) { return windows.FILE_ATTRIBUTE_REPARSE_POINT, nil }}
	if reparse.approved(directory, true) || reparse.approved(file.Name(), false) {
		t.Fatal("native reparse evidence was accepted")
	}
}
