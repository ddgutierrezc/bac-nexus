//go:build linux || darwin

package sqlite

import (
	"os"
	"syscall"
)

func filesystemEvidenceFor(root string) filesystemEvidence {
	evidence := filesystemEvidence{ApplicationDataRoot: applicationDataRoot()}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() {
		return evidence
	}
	evidence.Available, evidence.LinkSafe, evidence.ReparseSafe = proofYes, proofYes, proofYes
	if info.Mode()&os.ModeSymlink != 0 {
		evidence.LinkSafe = proofNo
		return evidence
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) == os.Geteuid() {
		evidence.Owner = proofYes
	}
	if info.Mode().Perm()&0o077 == 0 {
		evidence.Permissions = proofYes
	}
	evidence.Locality = filesystemLocalityFor(root)
	return evidence
}
