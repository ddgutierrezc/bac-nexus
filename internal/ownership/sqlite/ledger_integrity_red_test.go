package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"bac-nexus/internal/source"
)

func TestOpenInvokesVerifierForInitializedNewLedger(t *testing.T) {
	called := false
	original := runLedgerIntegrityVerifier
	runLedgerIntegrityVerifier = func(_ context.Context, db *sql.DB) verificationResult {
		called = true
		var schema string
		if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'ownership'`).Scan(&schema); err != nil || schema != ownershipSchema {
			t.Fatalf("new ledger was not initialized before verification: schema = %q, error = %v", schema, err)
		}
		return verificationResult{outcome: verificationPassed}
	}
	t.Cleanup(func() { runLedgerIntegrityVerifier = original })

	ledger, err := testOpen(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
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
	original := runLedgerIntegrityVerifier
	runLedgerIntegrityVerifier = func(context.Context, *sql.DB) verificationResult {
		called = true
		return verificationResult{outcome: verificationPassed}
	}
	t.Cleanup(func() { runLedgerIntegrityVerifier = original })

	ledger, err = testOpen(root)
	if ledger != nil {
		defer ledger.Close()
	}
	if !called {
		t.Fatal("existing ledger verifier was not invoked before metadata acceptance")
	}
	if !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("Open() error = %v; want ownership invalid after metadata rejection", err)
	}
}

func TestOpenAllowsInjectedPassedVerifierResult(t *testing.T) {
	called := false
	original := runLedgerIntegrityVerifier
	runLedgerIntegrityVerifier = func(context.Context, *sql.DB) verificationResult {
		called = true
		return verificationResult{outcome: verificationPassed}
	}
	t.Cleanup(func() { runLedgerIntegrityVerifier = original })

	ledger, err := testOpen(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v; want successful passed verification", err)
	}
	defer ledger.Close()
	if !called {
		t.Fatal("passed verifier result was not observed")
	}
}

func TestOpenRejectsInjectedNotRunVerifierResult(t *testing.T) {
	called := false
	original := runLedgerIntegrityVerifier
	runLedgerIntegrityVerifier = func(context.Context, *sql.DB) verificationResult {
		called = true
		return verificationResult{outcome: verificationNotRun}
	}
	t.Cleanup(func() { runLedgerIntegrityVerifier = original })

	ledger, err := testOpen(t.TempDir())
	if ledger != nil {
		defer ledger.Close()
	}
	if !called {
		t.Fatal("not-run verifier result was not observed")
	}
	if !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("Open() error = %v; want ownership invalid", err)
	}
}

func TestOpenRejectsInjectedCorruptVerifierResult(t *testing.T) {
	called := false
	original := runLedgerIntegrityVerifier
	runLedgerIntegrityVerifier = func(context.Context, *sql.DB) verificationResult {
		called = true
		return verificationResult{outcome: verificationCorrupt}
	}
	t.Cleanup(func() { runLedgerIntegrityVerifier = original })

	ledger, err := testOpen(t.TempDir())
	if ledger != nil {
		defer ledger.Close()
	}
	if !called {
		t.Fatal("corrupt verifier result was not observed")
	}
	if !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("Open() error = %v; want ownership invalid", err)
	}
}

func TestOpenRejectsInjectedInconclusiveVerifierResult(t *testing.T) {
	called := false
	original := runLedgerIntegrityVerifier
	runLedgerIntegrityVerifier = func(context.Context, *sql.DB) verificationResult {
		called = true
		return verificationResult{outcome: verificationInconclusive}
	}
	t.Cleanup(func() { runLedgerIntegrityVerifier = original })

	ledger, err := testOpen(t.TempDir())
	if ledger != nil {
		defer ledger.Close()
	}
	if !called {
		t.Fatal("inconclusive verifier result was not observed")
	}
	if !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("Open() error = %v; want ownership invalid", err)
	}
}

func TestOpenRejectsInjectedBoundExceededVerifierResult(t *testing.T) {
	called := false
	original := runLedgerIntegrityVerifier
	runLedgerIntegrityVerifier = func(context.Context, *sql.DB) verificationResult {
		called = true
		return verificationResult{outcome: verificationBoundExceeded}
	}
	t.Cleanup(func() { runLedgerIntegrityVerifier = original })

	ledger, err := testOpen(t.TempDir())
	if ledger != nil {
		defer ledger.Close()
	}
	if !called {
		t.Fatal("bound-exceeded verifier result was not observed")
	}
	if !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("Open() error = %v; want ownership invalid", err)
	}
}

func TestLedgerIntegrityVerifierEndsWithRealIntegrityCheck(t *testing.T) {
	ledger, err := testOpen(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()

	result := runLedgerIntegrityVerifier(context.Background(), ledger.db)
	want := verificationResult{stage: verificationIntegrityCheck, outcome: verificationPassed}
	if result != want {
		t.Fatalf("verification result = %#v, want %#v", result, want)
	}
}
