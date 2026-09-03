package sqlite

import (
	"errors"
	"testing"

	"bac-nexus/internal/localstate"
	"bac-nexus/internal/source"
)

type rejectedSecurePathPlatform struct{}

func (rejectedSecurePathPlatform) VerifyManagedDirectory(string, ...string) (localstate.Evidence, error) {
	return localstate.Evidence{}, localstate.ErrUnsafePath
}

func (rejectedSecurePathPlatform) CreateManagedFile(string, ...string) (localstate.Evidence, error) {
	return localstate.Evidence{}, localstate.ErrUnsafePath
}

func TestLedgerOpenRejectsUnavailableSecurePathEvidence(t *testing.T) {
	previous := securePathPlatform
	securePathPlatform = rejectedSecurePathPlatform{}
	t.Cleanup(func() { securePathPlatform = previous })
	ledger, err := Open(t.TempDir())
	if ledger != nil {
		_ = ledger.Close()
	}
	if !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("Open() error = %v, want %v", err, source.ErrOwnershipInvalid)
	}
}

type fileRejectedSecurePathPlatform struct{ called bool }

func (*fileRejectedSecurePathPlatform) VerifyManagedDirectory(string, ...string) (localstate.Evidence, error) {
	return localstate.Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}, nil
}

func (p *fileRejectedSecurePathPlatform) CreateManagedFile(string, ...string) (localstate.Evidence, error) {
	p.called = true
	return localstate.Evidence{}, localstate.ErrUnsafePath
}

func TestLedgerOpenRejectsUnavailableManagedFileEvidence(t *testing.T) {
	previous := securePathPlatform
	platform := &fileRejectedSecurePathPlatform{}
	securePathPlatform = platform
	t.Cleanup(func() { securePathPlatform = previous })
	ledger, err := Open(t.TempDir())
	if ledger != nil {
		_ = ledger.Close()
	}
	if !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("Open() error = %v, want %v", err, source.ErrOwnershipInvalid)
	}
	if !platform.called {
		t.Fatal("Open() did not require managed-file evidence")
	}
}
