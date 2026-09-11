//go:build linux

package codefori

import (
	"io"
	"os"
	"syscall"
)

// readPrivateTokenFile opens the final path without following a symlink, then
// validates the opened descriptor before reading it. This closes the Lstat to
// ReadFile replacement window for Linux, where Companion is routinely hosted.
func readPrivateTokenFile(path string, maximum int) ([]byte, bool) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, false
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || !privateTokenEntry(info) {
		return nil, false
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(maximum)+1))
	return data, err == nil && len(data) <= maximum
}
