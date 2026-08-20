//go:build !linux && !darwin && !windows

package sqlite

func filesystemEvidenceFor(string) filesystemEvidence {
	return filesystemEvidence{ApplicationDataRoot: applicationDataRoot()}
}
