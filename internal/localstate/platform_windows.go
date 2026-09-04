//go:build windows

package localstate

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows has no portable Unix-style directory descriptor API in the standard
// library. This adapter therefore opens each explicitly managed component,
// rejects reparse points, and compares the component before and after security
// inspection. Any unavailable or changed evidence is rejected.
func inspectManagedDirectory(root string, components []string) (Evidence, error) {
	current := root
	for _, component := range components {
		current = filepath.Join(current, component)
		if err := os.Mkdir(current, 0o700); err != nil && !os.IsExist(err) {
			return Evidence{}, ErrUnsafePath
		}
		if !windowsPathApproved(current, true) {
			return Evidence{}, ErrUnsafePath
		}
	}
	return Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}, nil
}

func createManagedFile(path string) (Evidence, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return Evidence{}, ErrUnsafePath
	}
	if err := file.Close(); err != nil || !windowsPathApproved(path, false) {
		return Evidence{}, ErrUnsafePath
	}
	return Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}, nil
}

func verifyManagedOpenFile(path string, file *os.File) (Evidence, error) {
	if file == nil || !windowsPathApproved(path, false) {
		return Evidence{}, ErrUnsafePath
	}
	handle, err := file.Stat()
	current, currentErr := os.Lstat(path)
	if err != nil || currentErr != nil || !handle.Mode().IsRegular() || !os.SameFile(handle, current) {
		return Evidence{}, ErrUnsafePath
	}
	return Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}, nil
}

type fileDispositionInfo struct{ DeleteFile byte }

var setFileInformationByHandle = syscall.NewLazyDLL("kernel32.dll").NewProc("SetFileInformationByHandle")

func removeManagedFile(path, _ string) error {
	wide, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return ErrUnsafePath
	}
	handle, err := windows.CreateFile(wide, windows.DELETE|windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return ErrUnsafePath
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		_ = windows.CloseHandle(handle)
		return ErrUnsafePath
	}
	defer file.Close()
	if evidence, err := verifyManagedOpenFile(path, file); err != nil || !evidence.approved() {
		return ErrUnsafePath
	}
	disposition := fileDispositionInfo{DeleteFile: 1}
	result, _, callErr := setFileInformationByHandle.Call(uintptr(handle), uintptr(4), uintptr(unsafe.Pointer(&disposition)), unsafe.Sizeof(disposition))
	if result == 0 || callErr != syscall.Errno(0) {
		return ErrUnsafePath
	}
	return nil
}

func windowsPathApproved(path string, directory bool) bool {
	before, err := os.Lstat(path)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || (!directory && !before.Mode().IsRegular()) {
		return false
	}
	wide, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attributes, err := windows.GetFileAttributes(wide)
	if err != nil || attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || (directory && attributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0) {
		return false
	}
	if !windowsLocalVolume(wide) || !windowsCurrentUserOnly(path) {
		return false
	}
	after, err := os.Lstat(path)
	return err == nil && os.SameFile(before, after)
}

func windowsLocalVolume(path *uint16) bool {
	volume := make([]uint16, windows.MAX_LONG_PATH)
	if windows.GetVolumePathName(path, &volume[0], uint32(len(volume))) != nil || windows.GetDriveType(&volume[0]) != windows.DRIVE_FIXED {
		return false
	}
	var serial, length, flags uint32
	name := make([]uint16, windows.MAX_PATH)
	return windows.GetVolumeInformation(&volume[0], nil, 0, &serial, &length, &flags, &name[0], uint32(len(name))) == nil
}

func windowsCurrentUserOnly(path string) bool {
	security, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil || security == nil || !security.IsValid() {
		return false
	}
	owner, _, err := security.Owner()
	user, userErr := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || userErr != nil || owner == nil || user.User.Sid == nil || !owner.Equals(user.User.Sid) {
		return false
	}
	control, _, err := security.Control()
	dacl, _, daclErr := security.DACL()
	if err != nil || daclErr != nil || dacl == nil || control&windows.SE_DACL_PRESENT == 0 || dacl.AceCount == 0 {
		return false
	}
	for index := uint16(0); index < dacl.AceCount; index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if windows.GetAce(dacl, uint32(index), &ace) != nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || !owner.Equals((*windows.SID)(unsafe.Pointer(&ace.SidStart))) {
			return false
		}
	}
	return true
}
