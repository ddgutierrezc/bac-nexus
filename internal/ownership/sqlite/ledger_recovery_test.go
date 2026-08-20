package sqlite

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"bac-nexus/internal/source"
)

type recoveryLister interface {
	listRecovery(context.Context) ([]source.OwnershipRecord, error)
}

func TestLedgerListsBoundedValidatedRecoveryRows(t *testing.T) {
	t.Run("returns exact valid rows in creation order", func(t *testing.T) {
		ledger := openRecoveryLedger(t)
		want := []source.OwnershipRecord{recoveryRecord(1), recoveryRecord(2)}
		for _, record := range want {
			insertRecoveryRecord(t, ledger, record)
		}

		got, err := recoveryRows(t, ledger)
		if err != nil {
			t.Fatalf("list recovery rows: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("recovery rows = %#v; want %#v", got, want)
		}
	})

	t.Run("treats row sixty-five as overflow", func(t *testing.T) {
		ledger := openRecoveryLedger(t)
		for i := byte(1); i <= 65; i++ {
			insertRecoveryRecord(t, ledger, recoveryRecord(i))
		}

		got, err := recoveryRows(t, ledger)
		if !errors.Is(err, source.ErrOwnershipCapacity) {
			t.Fatalf("row 65 list error = %v; want ErrOwnershipCapacity", err)
		}
		if got != nil {
			t.Fatalf("row 65 recovery rows = %#v; want no rows", got)
		}
	})
}

func openRecoveryLedger(t *testing.T) *Ledger {
	t.Helper()
	ledger, err := testOpen(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := ledger.Close(); err != nil {
			t.Errorf("close ledger: %v", err)
		}
	})
	return ledger
}

func recoveryRows(t *testing.T, ledger *Ledger) ([]source.OwnershipRecord, error) {
	t.Helper()
	lister, ok := any(ledger).(recoveryLister)
	if !ok {
		t.Fatal("Ledger does not implement bounded recovery listing")
	}
	return lister.listRecovery(context.Background())
}

func insertRecoveryRecord(t *testing.T, ledger *Ledger, record source.OwnershipRecord) {
	t.Helper()
	_, err := ledger.db.Exec(
		`INSERT INTO ownership (token, remote_path, version, profile, target_digest, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		record.Token,
		record.RemotePath,
		1,
		record.Profile,
		record.TargetDigest,
		record.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		t.Fatal(err)
	}
}

func recoveryRecord(token byte) source.OwnershipRecord {
	return source.OwnershipRecord{
		Token:        bytes.Repeat([]byte{token}, 16),
		RemotePath:   fmt.Sprintf("/home/nexus/.bac-nexus/tmp/recovery-%03d.utf8", token),
		Profile:      "approved",
		TargetDigest: bytes.Repeat([]byte{token}, 32),
		CreatedAt:    time.Date(2026, 8, 20, 0, 0, int(token), 0, time.UTC),
	}
}
