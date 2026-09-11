//go:build !linux

package codefori

import "os"

// Windows has no portable no-follow descriptor in the standard library. Its
// current-principal application-data boundary still rejects symlink and
// non-regular paths before reading; it does not claim independent DACL proof.
func readPrivateTokenFile(path string, maximum int) ([]byte, bool) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || !privateTokenEntry(info) {
		return nil, false
	}
	data, err := os.ReadFile(path)
	return data, err == nil && len(data) <= maximum
}
