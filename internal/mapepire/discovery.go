package mapepire

import (
	"errors"
	"os"
	"path/filepath"
)

const (
	codeForIBMiExtensionDirectory = "halcyontechltd.code-for-ibmi-3.0.12"
	serverJARRelativePath         = "dist/mapepire-server-2.3.5.jar"
)

type DiscoveryStatus string

const (
	DiscoveryFound     DiscoveryStatus = "found"
	DiscoveryNotFound  DiscoveryStatus = "not-found"
	DiscoveryAmbiguous DiscoveryStatus = "ambiguous"
)

type DiscoveryResult struct {
	Status                 DiscoveryStatus
	Path                   string
	VerifiedCandidateCount int
	RejectedCandidateCount int
	InspectionFailed       bool
}

type JARDiscovery struct {
	ExtensionsRoot string
	Verify         func(string) error
	canonicalize   func(string) (string, error)
	lstat          func(string) (os.FileInfo, error)
}

func DiscoverInstalledServerJAR() DiscoveryResult {
	home, err := os.UserHomeDir()
	if err != nil {
		return DiscoveryResult{Status: DiscoveryNotFound, InspectionFailed: true}
	}
	return (JARDiscovery{
		ExtensionsRoot: filepath.Join(home, ".vscode", "extensions"),
		Verify:         VerifyServerJAR,
	}).Discover()
}

func (d JARDiscovery) Discover() DiscoveryResult {
	if d.ExtensionsRoot == "" || d.Verify == nil {
		return DiscoveryResult{Status: DiscoveryNotFound, InspectionFailed: true}
	}
	lstat := d.lstat
	if lstat == nil {
		lstat = os.Lstat
	}
	if err := rejectLinkedPathComponents(d.ExtensionsRoot, lstat); err != nil {
		return DiscoveryResult{Status: DiscoveryNotFound, InspectionFailed: !errors.Is(err, os.ErrNotExist)}
	}
	entries, err := os.ReadDir(d.ExtensionsRoot)
	if err != nil {
		return DiscoveryResult{Status: DiscoveryNotFound, InspectionFailed: !os.IsNotExist(err)}
	}
	canonicalize := d.canonicalize
	if canonicalize == nil {
		canonicalize = canonicalPath
	}
	verified := make(map[string]string)
	rejected := 0
	for _, entry := range entries {
		if !isCodeForIBMiDirectory(entry.Name()) {
			continue
		}
		extensionDirectory := filepath.Join(d.ExtensionsRoot, entry.Name())
		if err := rejectLinkedPathComponents(extensionDirectory, lstat); err != nil {
			return DiscoveryResult{Status: DiscoveryNotFound, InspectionFailed: true}
		}
		if !entry.IsDir() {
			return DiscoveryResult{Status: DiscoveryNotFound, InspectionFailed: true}
		}
		candidate := filepath.Join(extensionDirectory, filepath.FromSlash(serverJARRelativePath))
		if err := d.Verify(candidate); err != nil {
			rejected++
			continue
		}
		canonical, err := canonicalize(candidate)
		if err != nil {
			rejected++
			continue
		}
		verified[filepath.Clean(canonical)] = filepath.Clean(canonical)
	}
	result := DiscoveryResult{VerifiedCandidateCount: len(verified), RejectedCandidateCount: rejected}
	switch len(verified) {
	case 0:
		result.Status = DiscoveryNotFound
	case 1:
		result.Status = DiscoveryFound
		for _, candidate := range verified {
			result.Path = candidate
		}
	default:
		result.Status = DiscoveryAmbiguous
	}
	return result
}

func isCodeForIBMiDirectory(name string) bool {
	return name == codeForIBMiExtensionDirectory
}

func rejectLinkedPathComponents(path string, lstat func(string) (os.FileInfo, error)) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	volume := filepath.VolumeName(absolute)
	current := volume + string(os.PathSeparator)
	remainder := absolute[len(volume):]
	for len(remainder) > 0 {
		for len(remainder) > 0 && os.IsPathSeparator(remainder[0]) {
			remainder = remainder[1:]
		}
		if remainder == "" {
			break
		}
		nextSeparator := -1
		for index := 0; index < len(remainder); index++ {
			if os.IsPathSeparator(remainder[index]) {
				nextSeparator = index
				break
			}
		}
		component := remainder
		if nextSeparator >= 0 {
			component = remainder[:nextSeparator]
			remainder = remainder[nextSeparator+1:]
		} else {
			remainder = ""
		}
		current = filepath.Join(current, component)
		info, err := lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("linked path component is not allowed")
		}
	}
	return nil
}

func canonicalPath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	return filepath.Abs(resolved)
}
