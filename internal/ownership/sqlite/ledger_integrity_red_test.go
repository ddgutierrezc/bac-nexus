package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bac-nexus/internal/source"
)

func TestOpenInvokesVerifierForInitializedNewLedger(t *testing.T) {
	called := false
	replaceLedgerIntegrityVerifier(t, func(_ context.Context, db *sql.DB) verificationResult {
		called = true
		var schema string
		if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'ownership'`).Scan(&schema); err != nil || schema != ownershipSchema {
			t.Fatalf("new ledger was not initialized before verification: schema = %q, error = %v", schema, err)
		}
		return verificationResult{outcome: verificationPassed}
	})

	openIntegrityLedger(t)
	if !called {
		t.Fatal("new ledger verifier was not invoked after initialization")
	}
}

func TestOpenInvokesVerifierBeforeExistingMetadataAcceptance(t *testing.T) {
	root := t.TempDir()
	ledger, err := testOpen(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", filepath.Join(root, "ownership.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 0`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	called := false
	replaceLedgerIntegrityVerifier(t, func(context.Context, *sql.DB) verificationResult {
		called = true
		return verificationResult{outcome: verificationPassed}
	})

	ledger, err = testOpen(root)
	if ledger != nil {
		t.Cleanup(func() { _ = ledger.Close() })
	}
	if !called {
		t.Fatal("existing ledger verifier was not invoked before metadata acceptance")
	}
	if !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("Open() error = %v; want ownership invalid after metadata rejection", err)
	}
}

func TestOpenAllowsInjectedPassedVerifierResult(t *testing.T) {
	assertOpenVerifierResult(t, verificationPassed, false)
}

func TestOpenRejectsInjectedNotRunVerifierResult(t *testing.T) {
	assertOpenVerifierResult(t, verificationNotRun, true)
}

func TestOpenRejectsInjectedCorruptVerifierResult(t *testing.T) {
	assertOpenVerifierResult(t, verificationCorrupt, true)
}

func TestOpenRejectsInjectedInconclusiveVerifierResult(t *testing.T) {
	assertOpenVerifierResult(t, verificationInconclusive, true)
}

func TestOpenRejectsInjectedBoundExceededVerifierResult(t *testing.T) {
	assertOpenVerifierResult(t, verificationBoundExceeded, true)
}

func assertOpenVerifierResult(t *testing.T, outcome verificationOutcome, rejected bool) {
	t.Helper()
	called := false
	replaceLedgerIntegrityVerifier(t, func(context.Context, *sql.DB) verificationResult {
		called = true
		return verificationResult{outcome: outcome}
	})
	ledger, err := testOpen(t.TempDir())
	if ledger != nil {
		t.Cleanup(func() { _ = ledger.Close() })
	}
	if !called || rejected != errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("called = %t, error = %v, rejected = %t", called, err, rejected)
	}
}

func TestLedgerIntegrityVerifierEndsWithRealIntegrityCheck(t *testing.T) {
	ledger := openIntegrityLedger(t)

	result := runLedgerIntegrityVerifier(context.Background(), ledger.db)
	want := verificationResult{stage: verificationIntegrityCheck, outcome: verificationPassed}
	if result != want {
		t.Fatalf("verification result = %#v, want %#v", result, want)
	}
}

func TestLedgerIntegrityVerifierHandlesBoundedQueryEdges(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		query func(string) ([]string, error)
		want  verificationResult
	}{
		{"ordered", func(query string) ([]string, error) {
			if query == "PRAGMA page_count" {
				return []string{"1"}, nil
			}
			if query == "PRAGMA page_size" {
				return []string{"4096"}, nil
			}
			return []string{"ok"}, nil
		}, verificationResult{verificationIntegrityCheck, verificationPassed}},
		{"absent", func(string) ([]string, error) { return nil, nil }, verificationResult{verificationQuickCheck, verificationInconclusive}},
		{"multiple", func(string) ([]string, error) { return []string{"ok", "ok"}, nil }, verificationResult{verificationQuickCheck, verificationInconclusive}},
		{"failure", func(string) ([]string, error) { return nil, errors.New("blocked") }, verificationResult{verificationQuickCheck, verificationInconclusive}},
		{"overflow", func(query string) ([]string, error) {
			if query == "PRAGMA page_count" {
				return []string{"9223372036854775807"}, nil
			}
			if query == "PRAGMA page_size" {
				return []string{"2"}, nil
			}
			return []string{"ok"}, nil
		}, verificationResult{verificationQuickCheck, verificationBoundExceeded}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var queries []string
			replaceLedgerIntegrityQuery(t, func(_ context.Context, _ *sql.DB, query string) ([]string, error) {
				queries = append(queries, query)
				return testCase.query(query)
			})
			result := verifyLedgerIntegrity(context.Background(), nil)
			if result != testCase.want {
				t.Fatalf("result = %#v, want %#v", result, testCase.want)
			}
			if testCase.name == "ordered" && strings.Join(queries, ",") != "PRAGMA quick_check(1),PRAGMA page_count,PRAGMA page_size,PRAGMA integrity_check(1)" {
				t.Fatalf("queries = %q", queries)
			}
		})
	}
}

func TestLedgerIntegrityVerifierRejectsOversizedLedger(t *testing.T) {
	ledger := openIntegrityLedger(t)
	if _, err := ledger.db.Exec("CREATE TABLE padding (data BLOB); INSERT INTO padding VALUES (zeroblob(4194305))"); err != nil {
		t.Fatal(err)
	}
	if result := verifyLedgerIntegrity(context.Background(), ledger.db); result.outcome != verificationBoundExceeded {
		t.Fatalf("result = %#v", result)
	}
}

func TestLedgerIntegrityVerifierClassifiesRealCorruption(t *testing.T) {
	root := t.TempDir()
	ledger, err := testOpen(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ownership.db"), []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(root, "ownership.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if result := verifyLedgerIntegrity(context.Background(), db); result.outcome != verificationCorrupt {
		t.Fatalf("result = %#v", result)
	}
}

func TestLedgerIntegrityVerifierCancellationReleasesBlockingQuery(t *testing.T) {
	started := make(chan struct{})
	replaceLedgerIntegrityQuery(t, func(ctx context.Context, _ *sql.DB, _ string) ([]string, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	results := make(chan verificationResult, 1)
	go func() { results <- verifyLedgerIntegrity(ctx, nil) }()
	<-started
	cancel()
	if result := <-results; result.outcome != verificationInconclusive {
		t.Fatalf("result = %#v", result)
	}
}

func openIntegrityLedger(t *testing.T) *Ledger {
	t.Helper()
	ledger, err := testOpen(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := ledger.Close(); err != nil {
			t.Error(err)
		}
	})
	return ledger
}

func replaceLedgerIntegrityVerifier(t *testing.T, verifier func(context.Context, *sql.DB) verificationResult) {
	t.Helper()
	original := runLedgerIntegrityVerifier
	runLedgerIntegrityVerifier = verifier
	t.Cleanup(func() { runLedgerIntegrityVerifier = original })
}

func replaceLedgerIntegrityQuery(t *testing.T, query func(context.Context, *sql.DB, string) ([]string, error)) {
	t.Helper()
	original := queryLedgerIntegrity
	queryLedgerIntegrity = query
	t.Cleanup(func() { queryLedgerIntegrity = original })
}
