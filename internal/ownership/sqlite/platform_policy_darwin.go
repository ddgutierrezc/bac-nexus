//go:build darwin

package sqlite

import "syscall"

func filesystemLocalityFor(root string) filesystemLocality {
	var stat syscall.Statfs_t
	if syscall.Statfs(root, &stat) != nil {
		return localityUnknown
	}
	name := make([]byte, 0, len(stat.Fstypename))
	for _, character := range stat.Fstypename {
		if character == 0 {
			break
		}
		name = append(name, byte(character))
	}
	switch string(name) {
	case "apfs", "hfs", "ufs", "zfs":
		return localityLocal
	case "nfs", "smbfs", "afpfs":
		return localityNetwork
	case "osxfuse", "webdav":
		return localityShared
	}
	return localityUnknown
}
