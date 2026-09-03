//go:build linux

package localstate

import (
	"os"

	"golang.org/x/sys/unix"
)

func inspectManagedDirectory(root string, components []string) (Evidence, error) {
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return Evidence{}, ErrUnsafePath
	}
	defer func() { _ = unix.Close(fd) }()
	var rootStat unix.Stat_t
	if unix.Fstat(fd, &rootStat) != nil || int(rootStat.Uid) != os.Geteuid() || !localFilesystem(fd) {
		return Evidence{}, ErrUnsafePath
	}
	for _, component := range components {
		if err := unix.Mkdirat(fd, component, 0o700); err != nil && err != unix.EEXIST {
			return Evidence{}, ErrUnsafePath
		}
		next, err := unix.Openat(fd, component, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if err != nil {
			return Evidence{}, ErrUnsafePath
		}
		var byHandle, byName unix.Stat_t
		stable := unix.Fstat(next, &byHandle) == nil && unix.Fstatat(fd, component, &byName, unix.AT_SYMLINK_NOFOLLOW) == nil && byHandle.Dev == byName.Dev && byHandle.Ino == byName.Ino
		valid := stable && int(byHandle.Uid) == os.Geteuid() && byHandle.Dev == rootStat.Dev && byHandle.Mode&0o777 == 0o700 && localFilesystem(next)
		unix.Close(fd)
		fd = next
		if !valid {
			return Evidence{}, ErrUnsafePath
		}
	}
	return Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}, nil
}

func localFilesystem(fd int) bool {
	var stat unix.Statfs_t
	if unix.Fstatfs(fd, &stat) != nil {
		return false
	}
	switch stat.Type {
	case 0xEF53, 0x58465342, 0x9123683E, 0x01021994:
		return true
	default:
		return false
	}
}

func createManagedFile(path string) (Evidence, error) {
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return Evidence{}, ErrUnsafePath
	}
	defer unix.Close(fd)
	var byHandle, byName unix.Stat_t
	if unix.Fstat(fd, &byHandle) != nil || unix.Lstat(path, &byName) != nil || byHandle.Dev != byName.Dev || byHandle.Ino != byName.Ino || int(byHandle.Uid) != os.Geteuid() || byHandle.Mode&unix.S_IFMT != unix.S_IFREG || byHandle.Mode&0o777 != 0o600 || !localFilesystem(fd) {
		return Evidence{}, ErrUnsafePath
	}
	return Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}, nil
}
