package sqlite

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"bac-nexus/internal/source"
)

func TestLedgerTransactionLockHelper(t *testing.T) {
	if os.Getenv("NEXUS_SQLITE_LOCK_HELPER") != "1" {
		return
	}

	db, err := sql.Open("sqlite", os.Getenv("NEXUS_SQLITE_LOCK_DATABASE"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("PRAGMA busy_timeout = 0"); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("NEXUS_SQLITE_LOCK_MODE") == "reader" {
		if _, err := db.Exec("BEGIN"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("SELECT count(*) FROM ownership"); err != nil {
			t.Fatal(err)
		}
	} else if _, err := db.Exec("BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}
	fmt.Println("locked")
	duration, err := time.ParseDuration(os.Getenv("NEXUS_SQLITE_LOCK_DURATION"))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(duration)
	if _, err := db.Exec("COMMIT"); err != nil {
		t.Fatal(err)
	}
}

func TestLedgerAdmissionUsesExactRetrySchedule(t *testing.T) {
	ledger, err := testOpen(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()

	lock := startLedgerLock(t, ledger, "exclusive", 1100*time.Millisecond)
	defer waitLedgerLock(t, lock)
	started := time.Now()
	err = ledger.Admit(context.Background(), transactionRecord(1))
	elapsed := time.Since(started)
	if err != nil {
		t.Fatalf("Admit during SQLite contention = %v; want retry after exactly 25ms, 50ms, and 100ms", err)
	}
	if elapsed < time.Second || elapsed > 1500*time.Millisecond {
		t.Fatalf("Admit elapsed %s; want bounded 25ms+50ms+100ms retry schedule without extra delay", elapsed)
	}
}

func TestLedgerAdmissionCancellationInterruptsRetryBackoff(t *testing.T) {
	ledger, err := testOpen(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()

	lock := startLedgerLock(t, ledger, "exclusive", time.Second)
	defer waitLedgerLock(t, lock)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(275*time.Millisecond, cancel)
	started := time.Now()
	err = ledger.Admit(ctx, transactionRecord(2))
	elapsed := time.Since(started)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Admit cancellation error = %v; want context.Canceled while retrying", err)
	}
	if elapsed < 250*time.Millisecond || elapsed > 500*time.Millisecond {
		t.Fatalf("Admit cancellation elapsed %s; want prompt interruption of retry backoff", elapsed)
	}
}

func TestLedgerAdmissionReadsBackAfterAmbiguousCommit(t *testing.T) {
	ledger, err := testOpen(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()

	lock := startLedgerLock(t, ledger, "reader", 300*time.Millisecond)
	defer waitLedgerLock(t, lock)
	record := transactionRecord(3)
	if err := ledger.Admit(context.Background(), record); err != nil {
		t.Fatalf("Admit after ambiguous COMMIT = %v; want exact-token readback and retry", err)
	}
	var count int
	if err := ledger.db.QueryRow("SELECT count(*) FROM ownership WHERE token = ?", record.Token).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("exact-token readback count = %d; want one committed ownership row without duplicate mutation", count)
	}
}

func TestLedgerOpenRejectsIntegrityCheckFailure(t *testing.T) {
	root := t.TempDir()
	ledger, err := testOpen(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, "ownership.db")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	contents[36], contents[37], contents[38], contents[39] = 0, 0, 0, 1
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := testOpen(root); !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("Open with inconsistent free-page count = %v; want deterministic %v when bounded quick_check cannot establish integrity", err, source.ErrOwnershipInvalid)
	}
}

func TestLedgerOpenRejectsCorruptOwnershipPages(t *testing.T) {
	root := t.TempDir()
	ledger, err := testOpen(root)
	if err != nil {
		t.Fatal(err)
	}
	for token := byte(10); token < 64; token++ {
		if err := ledger.Admit(context.Background(), transactionRecord(token)); err != nil {
			t.Fatal(err)
		}
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, "ownership.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA writable_schema = ON"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE sqlite_master SET rootpage = 3 WHERE type = 'table' AND name = 'ownership'"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA writable_schema = OFF"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := testOpen(root); !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("Open with corrupted ownership root page = %v; want deterministic %v", err, source.ErrOwnershipInvalid)
	}
}

func TestLedgerBoundsCooperatingProcessContention(t *testing.T) {
	ledger, err := testOpen(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()

	lock := startLedgerLock(t, ledger, "exclusive", time.Second)
	defer waitLedgerLock(t, lock)
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = ledger.Admit(ctx, transactionRecord(65))
	elapsed := time.Since(started)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Admit under cooperating-process contention = %v; want context deadline", err)
	}
	if elapsed < 350*time.Millisecond || elapsed > 650*time.Millisecond {
		t.Fatalf("contention elapsed %s; want bounded multi-process retry behavior", elapsed)
	}
}

func startLedgerLock(t *testing.T, ledger *Ledger, mode string, duration time.Duration) *exec.Cmd {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestLedgerTransactionLockHelper$")
	command.Env = append(os.Environ(),
		"NEXUS_SQLITE_LOCK_HELPER=1",
		"NEXUS_SQLITE_LOCK_DATABASE="+filepath.Join(ledgerRoot(ledger), "ownership.db"),
		"NEXUS_SQLITE_LOCK_MODE="+mode,
		"NEXUS_SQLITE_LOCK_DURATION="+duration.String(),
	)
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	ready, err := bufio.NewReader(output).ReadString('\n')
	if err != nil || ready != "locked\n" {
		_ = command.Process.Kill()
		t.Fatalf("SQLite lock helper readiness = %q, %v", ready, err)
	}
	return command
}

func waitLedgerLock(t *testing.T, command *exec.Cmd) {
	t.Helper()
	if err := command.Wait(); err != nil {
		t.Errorf("SQLite lock helper failed: %v", err)
	}
}

func ledgerRoot(ledger *Ledger) string {
	var path string
	if err := ledger.db.QueryRow("PRAGMA database_list").Scan(new(int), new(string), &path); err != nil {
		panic(err)
	}
	return filepath.Dir(path)
}

func transactionRecord(token byte) source.OwnershipRecord {
	return source.OwnershipRecord{
		Token:        bytes.Repeat([]byte{token}, 16),
		RemotePath:   "/home/nexus/tmp/transaction-" + strconv.Itoa(int(token)),
		Profile:      "approved",
		TargetDigest: bytes.Repeat([]byte{token}, 32),
		CreatedAt:    time.Date(2026, 8, 20, 0, 0, int(token), 0, time.UTC),
	}
}
