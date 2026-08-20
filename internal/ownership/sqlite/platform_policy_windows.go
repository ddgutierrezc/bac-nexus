//go:build windows

package sqlite

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

func filesystemEvidenceFor(root string) filesystemEvidence {
	evidence := filesystemEvidence{ApplicationDataRoot: applicationDataRoot()}
	path, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return evidence
	}
	attributes, err := windows.GetFileAttributes(path)
	if err != nil {
		return evidence
	}
	evidence.Available, evidence.LinkSafe, evidence.ReparseSafe = proofYes, proofYes, proofYes
	if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		evidence.LinkSafe, evidence.ReparseSafe = proofNo, proofNo
		return evidence
	}
	evidence.Locality = windowsFilesystemLocality(path)
	security, err := windows.GetNamedSecurityInfo(root, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil || security == nil || !security.IsValid() {
		return evidence
	}
	owner, _, err := security.Owner()
	user, userErr := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || userErr != nil || owner == nil || user.User.Sid == nil || !owner.Equals(user.User.Sid) {
		return evidence
	}
	evidence.Owner = proofYes
	evidence.Permissions = restrictiveDACL(security, owner)
	return evidence
}

func windowsFilesystemLocality(path *uint16) filesystemLocality {
	volume := make([]uint16, windows.MAX_LONG_PATH)
	if windows.GetVolumePathName(path, &volume[0], uint32(len(volume))) != nil {
		return localityUnknown
	}
	if windows.GetDriveType(&volume[0]) == windows.DRIVE_REMOTE {
		return localityNetwork
	}
	if windows.GetDriveType(&volume[0]) != windows.DRIVE_FIXED {
		return localityUnknown
	}
	var serial, length, flags uint32
	name := make([]uint16, windows.MAX_PATH)
	if windows.GetVolumeInformation(&volume[0], nil, 0, &serial, &length, &flags, &name[0], uint32(len(name))) != nil {
		return localityUnknown
	}
	return localityLocal
}

func restrictiveDACL(security *windows.SECURITY_DESCRIPTOR, owner *windows.SID) proof {
	control, _, err := security.Control()
	dacl, _, daclErr := security.DACL()
	if err != nil || daclErr != nil || dacl == nil || control&windows.SE_DACL_PRESENT == 0 {
		return proofNo
	}
	ownerAllowed := false
	for index := uint16(0); index < dacl.AceCount; index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if windows.GetAce(dacl, uint32(index), &ace) != nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return proofNo
		}
		if !owner.Equals((*windows.SID)(unsafe.Pointer(&ace.SidStart))) {
			return proofNo
		}
		ownerAllowed = true
	}
	if ownerAllowed {
		return proofYes
	}
	return proofNo
}
