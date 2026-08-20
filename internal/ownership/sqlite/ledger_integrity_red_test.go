package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"testing"

	"bac-nexus/internal/source"
)

func TestLedgerVerifierRunsQuickCheckForEveryOpen(t *testing.T) {
	ledger := openIntegrityLedger(t)
	defer ledger.Close()

	result := runLedgerIntegrityVerifier(context.Background(), ledger.db)
	if result.stage != verificationQuickCheck || result.outcome != verificationPassed {
		t.Fatalf("quick verification result = %#v; want quick check passed", result)
	}
}

func TestLedgerVerifierRunsProportionalIntegrityCheck(t *testing.T) {
	ledger := openIntegrityLedger(t)
	defer ledger.Close()

	result := runLedgerIntegrityVerifier(context.Background(), ledger.db)
	if result.stage != verificationIntegrityCheck || result.outcome != verificationPassed {
		t.Fatalf("proportional verification result = %#v; want integrity check passed", result)
	}
}

func TestLedgerVerifierRejectsCorruption(t *testing.T) {
	ledger := openIntegrityLedger(t)
	defer ledger.Close()
	if _, err := ledger.db.Exec(`PRAGMA writable_schema = ON`); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.db.Exec(`DELETE FROM sqlite_master WHERE name = 'ownership'`); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.db.Exec(`PRAGMA writable_schema = OFF`); err != nil {
		t.Fatal(err)
	}

	result := runLedgerIntegrityVerifier(context.Background(), ledger.db)
	if result.outcome != verificationCorrupt {
		t.Fatalf("corrupt verification result = %#v; want corrupt", result)
	}
}

func TestLedgerVerifierRejectsInconclusiveOutput(t *testing.T) {
	ledger := openIntegrityLedger(t)
	defer ledger.Close()

	result := runLedgerIntegrityVerifier(context.Background(), ledger.db)
	if result.outcome != verificationInconclusive {
		t.Fatalf("inconclusive verification result = %#v; want inconclusive", result)
	}
}

func TestLedgerVerifierRejectsCancelledChecks(t *testing.T) {
	ledger := openIntegrityLedger(t)
	defer ledger.Close()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	result := runLedgerIntegrityVerifier(cancelled, ledger.db)
	if result.outcome != verificationInconclusive {
		t.Fatalf("cancelled verification result = %#v; want inconclusive", result)
	}
}

func TestLedgerVerifierRejectsOversizedDatabase(t *testing.T) {
	ledger := openIntegrityLedger(t)
	defer ledger.Close()
	if _, err := ledger.db.Exec(`CREATE TABLE padding (data BLOB)`); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.db.Exec(`INSERT INTO padding (data) VALUES (?)`, bytes.Repeat([]byte{1}, 4*1024*1024+1)); err != nil {
		t.Fatal(err)
	}

	result := runLedgerIntegrityVerifier(context.Background(), ledger.db)
	if result.outcome != verificationBoundExceeded {
		t.Fatalf("oversized verification result = %#v; want bound exceeded", result)
	}
}

func TestLedgerVerifierRejectsOverflowingSize(t *testing.T) {
	ledger := openIntegrityLedger(t)
	defer ledger.Close()
	if _, err := ledger.db.Exec(`PRAGMA page_size = 65536`); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.db.Exec(`PRAGMA max_page_count = 2147483647`); err != nil {
		t.Fatal(err)
	}

	result := runLedgerIntegrityVerifier(context.Background(), ledger.db)
	if result.outcome != verificationBoundExceeded {
		t.Fatalf("overflow verification result = %#v; want bound exceeded", result)
	}
}

func TestOpenMapsInjectedVerifierOutcomes(t *testing.T) {
	for _, tt := range []struct {
		name    string
		result  verificationResult
		wantErr bool
	}{
		{name: "not run", result: verificationResult{outcome: verificationNotRun}, wantErr: true},
		{name: "passed", result: verificationResult{stage: verificationIntegrityCheck, outcome: verificationPassed}},
		{name: "corrupt", result: verificationResult{stage: verificationQuickCheck, outcome: verificationCorrupt}, wantErr: true},
		{name: "inconclusive", result: verificationResult{stage: verificationQuickCheck, outcome: verificationInconclusive}, wantErr: true},
		{name: "bound exceeded", result: verificationResult{stage: verificationQuickCheck, outcome: verificationBoundExceeded}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			original := runLedgerIntegrityVerifier
			runLedgerIntegrityVerifier = func(context.Context, *sql.DB) verificationResult { return tt.result }
			t.Cleanup(func() { runLedgerIntegrityVerifier = original })

			ledger, err := testOpen(t.TempDir())
			if ledger != nil {
				defer ledger.Close()
			}
			if (errors.Is(err, source.ErrOwnershipInvalid)) != tt.wantErr {
				t.Fatalf("Open() error = %v; want ownership invalid = %t", err, tt.wantErr)
			}
		})
	}
}

func openIntegrityLedger(t *testing.T) *Ledger {
	t.Helper()
	ledger, err := testOpen(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return ledger
}
