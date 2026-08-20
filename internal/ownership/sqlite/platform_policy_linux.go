//go:build linux

package sqlite

import "syscall"

func filesystemLocalityFor(root string) filesystemLocality {
	var stat syscall.Statfs_t
	if syscall.Statfs(root, &stat) != nil {
		return localityUnknown
	}
	switch stat.Type {
	case 0xEF53, 0x58465342, 0x9123683E, 0x01021994:
		return localityLocal
	case 0x6969, 0x517B, 0xFF534D42:
		return localityNetwork
	case 0x65735546:
		return localityShared
	}
	return localityUnknown
}
