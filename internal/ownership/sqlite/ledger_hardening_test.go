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
	root := t.TempDir()
	evidence := approvedFilesystemEvidence(root)
	evidence.ApplicationDataRoot = filepath.Join(t.TempDir(), "application-data")
	assertPolicyRejects(t, root, evidence)
}

func TestLedgerFilesystemPolicyRejectsNetworkOrSharedRoots(t *testing.T) {
	root := t.TempDir()
	for _, testCase := range []struct {
		name  string
		apply func(*filesystemEvidence)
	}{
		{name: "network", apply: func(evidence *filesystemEvidence) { evidence.Local = false }},
		{name: "shared", apply: func(evidence *filesystemEvidence) { evidence.Shared = true }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			evidence := approvedFilesystemEvidence(root)
			testCase.apply(&evidence)
			assertPolicyRejects(t, root, evidence)
		})
	}
}

func TestLedgerFilesystemPolicyRejectsUnprovenOwnerOrPermissions(t *testing.T) {
	root := t.TempDir()
	for _, testCase := range []struct {
		name  string
		apply func(*filesystemEvidence)
	}{
		{name: "owner", apply: func(evidence *filesystemEvidence) { evidence.OwnerVerified = false }},
		{name: "permissions", apply: func(evidence *filesystemEvidence) { evidence.PermissionsRestrictive = false }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			evidence := approvedFilesystemEvidence(root)
			testCase.apply(&evidence)
			assertPolicyRejects(t, root, evidence)
		})
	}
}

func TestLedgerFilesystemPolicyRejectsSymlinkEvidence(t *testing.T) {
	root := t.TempDir()
	evidence := approvedFilesystemEvidence(root)
	evidence.Symlink = true
	assertPolicyRejects(t, root, evidence)
}

func TestLedgerFilesystemPolicyRejectsWindowsReparseEvidence(t *testing.T) {
	root := t.TempDir()
	evidence := approvedFilesystemEvidence(root)
	evidence.WindowsReparsePoint = true
	assertPolicyRejects(t, root, evidence)
}

func TestLedgerFilesystemPolicyFailsClosedForUnknownUnavailableOrContradictoryEvidence(t *testing.T) {
	root := t.TempDir()
	for _, testCase := range []struct {
		name  string
		apply func(*filesystemEvidence)
	}{
		{name: "unavailable", apply: func(evidence *filesystemEvidence) { evidence.Available = false }},
		{name: "unknown locality", apply: func(evidence *filesystemEvidence) { evidence.LocalKnown = false }},
		{name: "unknown owner", apply: func(evidence *filesystemEvidence) { evidence.OwnerKnown = false }},
		{name: "unknown permissions", apply: func(evidence *filesystemEvidence) { evidence.PermissionsKnown = false }},
		{name: "unknown link state", apply: func(evidence *filesystemEvidence) { evidence.LinkKnown = false }},
		{name: "contradictory locality", apply: func(evidence *filesystemEvidence) { evidence.Shared = true }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
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
		Available:              true,
		ApplicationDataRoot:    root,
		LocalKnown:             true,
		Local:                  true,
		Shared:                 false,
		OwnerKnown:             true,
		OwnerVerified:          true,
		PermissionsKnown:       true,
		PermissionsRestrictive: true,
		LinkKnown:              true,
		Symlink:                false,
		WindowsReparsePoint:    false,
	}
}
