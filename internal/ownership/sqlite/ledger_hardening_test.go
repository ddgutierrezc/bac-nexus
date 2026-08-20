package sqlite

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"bac-nexus/internal/source"
)

type rootPolicy struct {
	ApplicationData string
	Network         func(string) bool
	Reparse         func(string) bool
	Restrictive     func(string) bool
}

type integrityPolicy struct {
	QuickCheckLimit     time.Duration
	IntegrityCheckLimit time.Duration
}

type hardenedLedger interface {
	ValidateRoot(context.Context, string, rootPolicy) error
	AdmitWithRetry(context.Context, source.OwnershipRecord) error
	ResolveCommit(context.Context, []byte, error) error
	VerifyIntegrity(context.Context, integrityPolicy) error
}

func TestLedgerHardeningRejectsUnsafeApplicationDataRoots(t *testing.T) {
	ledger := openLedgerForHardening(t)
	policy := rootPolicy{ApplicationData: t.TempDir(), Network: func(string) bool { return false }, Restrictive: func(string) bool { return true }}
	for _, root := range []string{
		filepath.Join(t.TempDir(), "outside-application-data"),
		filepath.Join(policy.ApplicationData, "shared"),
	} {
		if err := ledger.ValidateRoot(context.Background(), root, policy); !errors.Is(err, source.ErrOwnershipInvalid) {
			t.Fatalf("unsafe root %q error = %v", root, err)
		}
	}
}

func TestLedgerHardeningRejectsNetworkSymlinkAndReparseEscapes(t *testing.T) {
	ledger := openLedgerForHardening(t)
	root := filepath.Join(t.TempDir(), "ledger")
	for name, policy := range map[string]rootPolicy{
		"network": {ApplicationData: filepath.Dir(root), Network: func(string) bool { return true }, Restrictive: func(string) bool { return true }},
		"symlink": {ApplicationData: filepath.Dir(root), Reparse: func(string) bool { return true }, Restrictive: func(string) bool { return true }},
		"reparse": {ApplicationData: filepath.Dir(root), Reparse: func(string) bool { return true }, Restrictive: func(string) bool { return true }},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ledger.ValidateRoot(context.Background(), root, policy); !errors.Is(err, source.ErrOwnershipInvalid) {
				t.Fatalf("%s root error = %v", name, err)
			}
		})
	}
}

func TestLedgerHardeningRejectsUnprovenPermissions(t *testing.T) {
	ledger := openLedgerForHardening(t)
	root := filepath.Join(t.TempDir(), "ledger")
	policy := rootPolicy{ApplicationData: filepath.Dir(root), Restrictive: func(string) bool { return false }}
	if err := ledger.ValidateRoot(context.Background(), root, policy); !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("unproven permission error = %v", err)
	}
}

func TestLedgerHardeningRetriesBusyWithinContextBudget(t *testing.T) {
	ledger := openLedgerForHardening(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	record := hardeningRecord()
	if err := ledger.AdmitWithRetry(ctx, record); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("busy admission error = %v", err)
	}
}

func TestLedgerHardeningResolvesAmbiguousCommitByExactToken(t *testing.T) {
	ledger := openLedgerForHardening(t)
	token := bytes.Repeat([]byte{7}, 16)
	if err := ledger.ResolveCommit(context.Background(), token, errors.New("ambiguous commit")); err != nil {
		t.Fatalf("exact-token readback error = %v", err)
	}
	if err := ledger.ResolveCommit(context.Background(), bytes.Repeat([]byte{8}, 16), errors.New("ambiguous commit")); !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("absent-token readback error = %v", err)
	}
}

func TestLedgerHardeningBoundsQuickAndIntegrityChecks(t *testing.T) {
	ledger := openLedgerForHardening(t)
	policy := integrityPolicy{QuickCheckLimit: 25 * time.Millisecond, IntegrityCheckLimit: 100 * time.Millisecond}
	if err := ledger.VerifyIntegrity(context.Background(), policy); !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("unchecked integrity error = %v", err)
	}
}

func TestLedgerHardeningCoordinatesCooperatingProcesses(t *testing.T) {
	ledger := openLedgerForHardening(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := ledger.AdmitWithRetry(ctx, hardeningRecord()); err != nil {
		t.Fatalf("cooperating admission error = %v", err)
	}
}

func openLedgerForHardening(t *testing.T) hardenedLedger {
	t.Helper()
	ledger, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ledger.Close() })
	hardened, ok := any(ledger).(hardenedLedger)
	if !ok {
		t.Fatal("Ledger does not implement the required SQLite hardening contract")
	}
	return hardened
}

func hardeningRecord() source.OwnershipRecord {
	return source.OwnershipRecord{
		Token:        bytes.Repeat([]byte{7}, 16),
		RemotePath:   "/home/nexus/tmp/hardening",
		Profile:      "approved",
		TargetDigest: bytes.Repeat([]byte{8}, 32),
		CreatedAt:    time.Unix(0, 0).UTC(),
	}
}
