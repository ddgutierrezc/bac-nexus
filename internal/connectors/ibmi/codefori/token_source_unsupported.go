//go:build !(aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || windows)

package codefori

import "os"

func privateTokenEntry(os.FileInfo) bool { return false }
