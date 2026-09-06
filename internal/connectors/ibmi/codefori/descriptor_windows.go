//go:build windows

package codefori

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	maxDescriptorBytes = 512
	windowsFullControl = windows.ACCESS_MASK(0x001f01ff)
)

// NewWindowsClient supplies the platform descriptor reader. Every uncertain
// filesystem or ACL result is presented to the caller as Companion unavailable.
func NewWindowsClient() *Client {
	return NewClient(newWindowsDescriptorReader(defaultWindowsDescriptorSource()))
}

// NewPlatformClient selects the guarded Windows reader for Companion mode.
func NewPlatformClient() *Client {
	return NewWindowsClient()
}

type windowsDescriptorSource struct {
	path     func() (directory, file string, ok bool)
	validate func(path string, directory bool) bool
	readFile func(path string) ([]byte, error)
}

func defaultWindowsDescriptorSource() windowsDescriptorSource {
	return windowsDescriptorSource{
		path: func() (string, string, bool) {
			root := os.Getenv("LOCALAPPDATA")
			if root == "" || !filepath.IsAbs(root) {
				return "", "", false
			}
			directory := filepath.Join(root, "BAC Nexus", "companion-v1")
			return directory, filepath.Join(directory, "descriptor.json"), true
		},
		validate: validateWindowsDescriptorPath,
		readFile: readValidatedWindowsDescriptor,
	}
}

func newWindowsDescriptorReader(source windowsDescriptorSource) DescriptorReader {
	return func() (Descriptor, error) {
		if source.path == nil || source.validate == nil || source.readFile == nil {
			return Descriptor{}, errors.New("descriptor reader unavailable")
		}
		directory, file, ok := source.path()
		if !ok || !source.validate(directory, true) || !source.validate(file, false) {
			return Descriptor{}, errors.New("descriptor validation failed")
		}
		data, err := source.readFile(file)
		if err != nil || len(data) == 0 || len(data) > maxDescriptorBytes {
			return Descriptor{}, errors.New("descriptor read failed")
		}
		descriptor, ok := decodeWindowsDescriptor(data)
		if !ok {
			return Descriptor{}, errors.New("descriptor contents invalid")
		}
		return descriptor, nil
	}
}

func validateWindowsDescriptorPath(path string, directory bool) bool {
	before, err := os.Lstat(path)
	if err != nil || before.Mode()&fs.ModeSymlink != 0 || before.Mode()&fs.ModeIrregular != 0 || (directory && !before.IsDir()) || (!directory && !before.Mode().IsRegular()) {
		return false
	}
	wide, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attributes, err := windows.GetFileAttributes(wide)
	if err != nil || attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || (directory && attributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0) || (!directory && attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0) {
		return false
	}
	security, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil || !exactWindowsOwnerDACL(security, directory) {
		return false
	}
	after, err := os.Lstat(path)
	return err == nil && os.SameFile(before, after)
}

func readValidatedWindowsDescriptor(path string) ([]byte, error) {
	wide, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(wide, windows.FILE_READ_DATA|windows.READ_CONTROL, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	ownedHandle := true
	defer func() {
		if ownedHandle {
			_ = windows.CloseHandle(handle)
		}
	}()

	var information windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &information); err != nil || information.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || information.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return nil, errors.New("descriptor handle is unsafe")
	}
	security, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil || !exactWindowsOwnerDACL(security, false) {
		return nil, errors.New("descriptor handle security is unsafe")
	}
	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		return nil, errors.New("descriptor handle unavailable")
	}
	ownedHandle = false
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, maxDescriptorBytes+1))
}

func exactWindowsOwnerDACL(security *windows.SECURITY_DESCRIPTOR, directory bool) bool {
	if security == nil || !security.IsValid() {
		return false
	}
	owner, _, err := security.Owner()
	if err != nil || owner == nil {
		return false
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil || !owner.Equals(user.User.Sid) {
		return false
	}
	control, _, err := security.Control()
	if err != nil || control&windows.SE_DACL_PRESENT == 0 || control&windows.SE_DACL_PROTECTED == 0 {
		return false
	}
	dacl, _, err := security.DACL()
	if err != nil || dacl == nil || dacl.AceCount != 1 {
		return false
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if windows.GetAce(dacl, 0, &ace) != nil || ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Mask != windowsFullControl {
		return false
	}
	if ace.Header.AceFlags&windows.INHERITED_ACE != 0 || ace.Header.AceFlags != expectedWindowsACEFlags(directory) {
		return false
	}
	return owner.Equals((*windows.SID)(unsafe.Pointer(&ace.SidStart)))
}

func expectedWindowsACEFlags(directory bool) uint8 {
	if directory {
		return windows.OBJECT_INHERIT_ACE | windows.CONTAINER_INHERIT_ACE
	}
	return windows.NO_INHERITANCE
}

func decodeWindowsDescriptor(data []byte) (Descriptor, bool) {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return Descriptor{}, false
	}
	seen := map[string]bool{}
	var descriptor Descriptor
	for decoder.More() {
		name, err := decoder.Token()
		if err != nil {
			return Descriptor{}, false
		}
		key, ok := name.(string)
		if !ok || seen[key] {
			return Descriptor{}, false
		}
		seen[key] = true
		switch key {
		case "version":
			if err := decoder.Decode(&descriptor.Version); err != nil {
				return Descriptor{}, false
			}
		case "generation":
			if err := decoder.Decode(&descriptor.Generation); err != nil {
				return Descriptor{}, false
			}
		case "token":
			if err := decoder.Decode(&descriptor.Token); err != nil {
				return Descriptor{}, false
			}
		default:
			return Descriptor{}, false
		}
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') || decoder.Decode(&struct{}{}) != io.EOF {
		return Descriptor{}, false
	}
	return descriptor, len(seen) == 3 && descriptor.Version == protocolVersion && validLowerHex(descriptor.Generation, 32) && validLowerHex(descriptor.Token, 64)
}

func validLowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
