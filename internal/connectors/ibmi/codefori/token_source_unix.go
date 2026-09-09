//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package codefori

import (
	"os"
	"syscall"
)

func privateTokenEntry(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(stat.Uid) == os.Getuid() && info.Mode().Perm()&0o077 == 0
}
