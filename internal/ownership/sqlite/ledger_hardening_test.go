package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"bac-nexus/internal/source"
	moderncsqlite "modernc.org/sqlite"
)

func TestLedgerHardeningRejectsSymlinkOrReparseRoot(t *testing.T) {
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
		t.Fatalf("Open(symlink root) error = %v, want %v", err, source.ErrOwnershipInvalid)
	}
}

func TestLedgerHardeningRejectsUnprovenRootPermissions(t *testing.T) {
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
		t.Fatalf("Open(world-readable root) error = %v, want %v", err, source.ErrOwnershipInvalid)
	}
}

func TestLedgerHardeningRejectsConfiguredNetworkRoot(t *testing.T) {
	root := os.Getenv("NEXUS_NETWORK_TEST_ROOT")
	if root == "" {
		t.Skip("runner has no mounted untrusted network root to prove this boundary")
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		t.Skipf("configured network root is not usable: %v", err)
	}
	ledger, err := Open(root)
	if ledger != nil {
		_ = ledger.Close()
	}
	if !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("Open(network root) error = %v, want %v", err, source.ErrOwnershipInvalid)
	}
}

func TestLedgerHardeningHonorsCancellationDuringProcessContention(t *testing.T) {
	root, ready, release := t.TempDir(), filepath.Join(t.TempDir(), "ready"), filepath.Join(t.TempDir(), "release")
	ledger, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	child := exec.Command(os.Args[0], "-test.run=^TestLedgerHardeningProcessHelper$")
	child.Env = append(os.Environ(), "LEDGER_HARDENING_CHILD=1", "LEDGER_ROOT="+root, "LEDGER_READY="+ready, "LEDGER_RELEASE="+release)
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.WriteFile(release, nil, 0o600); _ = child.Wait() }()
	for deadline := time.Now().Add(time.Second); ; time.Sleep(5 * time.Millisecond) {
		if _, err := os.Stat(ready); err == nil {
			break
		} else if time.Now().After(deadline) {
			t.Fatalf("child did not acquire the SQLite lock: %v", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := ledger.Admit(ctx, hardeningRecord()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Admit while child holds BEGIN IMMEDIATE error = %v, want deadline exceeded", err)
	}
}

func TestLedgerHardeningResolvesAmbiguousCommitByExactReadback(t *testing.T) {
	root := t.TempDir()
	ledger, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	driverName := "sqlite-ambiguous-commit-" + strconv.Itoa(int(atomic.AddUint64(&driverSequence, 1)))
	sql.Register(driverName, ambiguousCommitDriver{Driver: &moderncsqlite.Driver{}})
	db, err := sql.Open(driverName, filepath.Join(root, "ownership.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ledger.db = db
	record := hardeningRecord()
	err = ledger.Admit(context.Background(), record)
	var rows int
	if readbackErr := db.QueryRow(`SELECT count(*) FROM ownership WHERE token = ?`, record.Token).Scan(&rows); readbackErr != nil {
		t.Fatal(readbackErr)
	}
	if rows != 1 {
		t.Fatalf("COMMIT seam did not persist the exact token; rows = %d", rows)
	}
	if err != nil {
		t.Fatalf("Admit returned ambiguous COMMIT error after exact readback: %v", err)
	}
}

func TestLedgerHardeningRejectsDatabaseThatFailsQuickCheck(t *testing.T) {
	root := t.TempDir()
	ledger, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.db.Exec(`PRAGMA writable_schema = ON; UPDATE sqlite_master SET rootpage = 999 WHERE type = 'table' AND name = 'ownership'; PRAGMA writable_schema = OFF`); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root); !errors.Is(err, source.ErrOwnershipInvalid) {
		t.Fatalf("Open(quick-check-corrupt database) error = %v, want %v", err, source.ErrOwnershipInvalid)
	}
}

func TestLedgerHardeningProcessHelper(t *testing.T) {
	if os.Getenv("LEDGER_HARDENING_CHILD") != "1" {
		return
	}
	db, err := sql.Open("sqlite", filepath.Join(os.Getenv("LEDGER_ROOT"), "ownership.db"))
	if err != nil {
		os.Exit(2)
	}
	defer db.Close()
	conn, err := db.Conn(context.Background())
	if err != nil {
		os.Exit(3)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
		os.Exit(4)
	}
	if err := os.WriteFile(os.Getenv("LEDGER_READY"), nil, 0o600); err != nil {
		os.Exit(5)
	}
	for {
		if _, err := os.Stat(os.Getenv("LEDGER_RELEASE")); err == nil {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func hardeningRecord() source.OwnershipRecord {
	return source.OwnershipRecord{Token: bytes.Repeat([]byte{7}, 16), RemotePath: "/home/nexus/tmp/hardening", Profile: "approved", TargetDigest: bytes.Repeat([]byte{8}, 32), CreatedAt: time.Unix(0, 0).UTC()}
}

var driverSequence uint64

var errAmbiguousCommit = errors.New("simulated transport loss after COMMIT")

type ambiguousCommitDriver struct{ driver.Driver }

func (d ambiguousCommitDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.Driver.Open(name)
	return ambiguousCommitConn{Conn: conn}, err
}

type ambiguousCommitConn struct{ driver.Conn }

func (c ambiguousCommitConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	execer, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	result, err := execer.ExecContext(ctx, query, args)
	if err == nil && strings.EqualFold(strings.TrimSpace(query), "COMMIT") {
		return result, errAmbiguousCommit
	}
	return result, err
}
