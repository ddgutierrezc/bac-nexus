package sqlite

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bac-nexus/internal/source"
)

func TestLedgerCreatesOnlyApprovedSchemaAndPragmas(t *testing.T) {
	ledger, err := testOpen(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()

	var applicationID, userVersion, timeout, synchronous int
	var journal string
	if err := ledger.db.QueryRow(`PRAGMA application_id`).Scan(&applicationID); err != nil {
		t.Fatal(err)
	}
	if err := ledger.db.QueryRow(`PRAGMA user_version`).Scan(&userVersion); err != nil {
		t.Fatal(err)
	}
	if err := ledger.db.QueryRow(`PRAGMA journal_mode`).Scan(&journal); err != nil {
		t.Fatal(err)
	}
	if err := ledger.db.QueryRow(`PRAGMA busy_timeout`).Scan(&timeout); err != nil {
		t.Fatal(err)
	}
	if err := ledger.db.QueryRow(`PRAGMA synchronous`).Scan(&synchronous); err != nil {
		t.Fatal(err)
	}
	if applicationID != 1111573326 || userVersion != 1 || journal != "delete" || synchronous != 3 || timeout != 250 {
		t.Fatalf("pragmas = %d/%d/%s/%d/%d", applicationID, userVersion, journal, synchronous, timeout)
	}
	var schema string
	if err := ledger.db.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'ownership'`).Scan(&schema); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"token BLOB PRIMARY KEY", "remote_path TEXT UNIQUE", "version INTEGER", "profile TEXT", "target_digest BLOB", "created_at TEXT"} {
		if !strings.Contains(schema, field) {
			t.Fatalf("schema does not contain %q: %s", field, schema)
		}
	}
	for _, forbidden := range []string{"source", "credential", "command", "cursor", "host", "user", "model"} {
		if strings.Contains(strings.ToLower(schema), forbidden) {
			t.Fatalf("schema contains forbidden field %q: %s", forbidden, schema)
		}
	}
}

func TestLedgerRejectsPersistentFormatMismatch(t *testing.T) {
	root := t.TempDir()
	ledger, err := testOpen(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.db.Exec(`PRAGMA application_id = 0`); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := testOpen(root); !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("application mismatch error = %v", err)
	}
}

func TestLedgerRejectsUnsafeRootAndSchemaMismatch(t *testing.T) {
	unsafeRoot := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(unsafeRoot, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(unsafeRoot); !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("unsafe root error = %v", err)
	}

	root := t.TempDir()
	ledger, err := testOpen(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.db.Exec(`DROP TABLE ownership`); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := testOpen(root); !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("schema mismatch error = %v", err)
	}
}

func TestLedgerAdmissionIsBoundedIdempotentAndFailClosed(t *testing.T) {
	ledger, err := testOpen(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	record := source.OwnershipRecord{Token: bytes.Repeat([]byte{1}, 16), RemotePath: "/home/nexus/tmp/a", Profile: "approved", TargetDigest: bytes.Repeat([]byte{2}, 32), CreatedAt: time.Now().UTC()}
	if err := ledger.Admit(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ledger.Admit(cancelled, record); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled admission error = %v", err)
	}
	if err := ledger.Admit(context.Background(), record); err != nil {
		t.Fatalf("idempotent admit: %v", err)
	}
	changed := record
	changed.RemotePath = "/home/nexus/tmp/other"
	if err := ledger.Admit(context.Background(), changed); !errors.Is(err, source.ErrOwnershipMismatch) {
		t.Fatalf("mismatch error = %v", err)
	}
	for i := 2; i <= 64; i++ {
		record.Token[0] = byte(i)
		record.RemotePath = "/home/nexus/tmp/" + string(rune(i))
		if err := ledger.Admit(context.Background(), record); err != nil {
			t.Fatalf("admit %d: %v", i, err)
		}
	}
	record.Token[0] = 65
	if err := ledger.Admit(context.Background(), record); !errors.Is(err, source.ErrOwnershipCapacity) {
		t.Fatalf("row 65 error = %v", err)
	}
}

func testOpen(root string) (*Ledger, error) { return open(root, approvedFilesystemEvidence(root)) }
