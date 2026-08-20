package sqlite

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"bac-nexus/internal/source"
)

func TestLedgerFilesystemPolicyRejectsRootOutsideApplicationData(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		apply func(*filesystemEvidence)
	}{
		{"outside application data", func(e *filesystemEvidence) { e.ApplicationDataRoot = filepath.Join(t.TempDir(), "application-data") }},
		{"network", func(e *filesystemEvidence) { e.Locality = localityNetwork }},
		{"shared", func(e *filesystemEvidence) { e.Locality = localityShared }},
		{"unproven owner", func(e *filesystemEvidence) { e.Owner = proofNo }},
		{"unproven permissions", func(e *filesystemEvidence) { e.Permissions = proofNo }},
		{"symlink", func(e *filesystemEvidence) { e.LinkSafe = proofNo }},
		{"windows reparse point", func(e *filesystemEvidence) { e.ReparseSafe = proofNo }},
		{"unavailable", func(e *filesystemEvidence) { e.Available = proofNo }},
		{"unknown locality", func(e *filesystemEvidence) { e.Locality = localityUnknown }},
		{"unknown owner", func(e *filesystemEvidence) { e.Owner = proofUnknown }},
		{"unknown permissions", func(e *filesystemEvidence) { e.Permissions = proofUnknown }},
		{"unknown link state", func(e *filesystemEvidence) { e.LinkSafe = proofUnknown }},
		{"contradictory locality", func(e *filesystemEvidence) { e.Locality = localityContradictory }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			evidence := approvedFilesystemEvidence(root)
			testCase.apply(&evidence)
			assertPolicyRejects(t, root, evidence)
		})
	}
}

func TestLedgerFilesystemPolicySupplementalRealSymlinkEvidence(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "ledger-link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("runner cannot create a symlink/reparse point: %v", err)
	}
	ledger, err := Open(link)
	if ledger != nil {
		_ = ledger.Close()
	}
	if !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("Open(real symlink root) error = %v, want %v", err, source.ErrOwnershipInvalid)
	}
}

func TestLedgerFilesystemPolicySupplementalRealRestrictiveModeEvidence(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows mode bits do not prove ACL ownership or restrictiveness")
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	ledger, err := Open(root)
	if ledger != nil {
		_ = ledger.Close()
	}
	if !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("Open(real world-readable root) error = %v, want %v", err, source.ErrOwnershipInvalid)
	}
}

func assertPolicyRejects(t *testing.T, root string, evidence filesystemEvidence) {
	t.Helper()
	ledger, err := open(root, evidence)
	if ledger != nil {
		_ = ledger.Close()
	}
	if !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("open filesystem-policy error = %v, want %v", err, source.ErrOwnershipInvalid)
	}
}

func approvedFilesystemEvidence(root string) filesystemEvidence {
	return filesystemEvidence{
		Available:           proofYes,
		ApplicationDataRoot: root,
		Locality:            localityLocal,
		Owner:               proofYes,
		Permissions:         proofYes,
		LinkSafe:            proofYes,
		ReparseSafe:         proofYes,
	}
}
