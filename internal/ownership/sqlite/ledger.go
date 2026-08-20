package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bac-nexus/internal/source"

	sqliteDriver "modernc.org/sqlite"
)

const (
	applicationID   = 1111573326
	userVersion     = 1
	sqliteBusy      = 5
	maxLedgerBytes  = 4 << 20
	ownershipSchema = `CREATE TABLE ownership (token BLOB PRIMARY KEY CHECK(length(token) = 16), remote_path TEXT UNIQUE CHECK(length(CAST(remote_path AS BLOB)) BETWEEN 1 AND 1024), version INTEGER NOT NULL CHECK(version = 1), profile TEXT NOT NULL CHECK(length(profile) BETWEEN 1 AND 64), target_digest BLOB NOT NULL CHECK(length(target_digest) = 32), created_at TEXT NOT NULL CHECK(length(created_at) = 20))`
)

var transactionRetryDelays = [...]time.Duration{25 * time.Millisecond, 50 * time.Millisecond, 100 * time.Millisecond}

type verificationStage uint8

const (
	verificationNotStarted verificationStage = iota
	verificationQuickCheck
	verificationIntegrityCheck
)

type verificationOutcome uint8

const (
	verificationNotRun verificationOutcome = iota
	verificationPassed
	verificationCorrupt
	verificationInconclusive
	verificationBoundExceeded
)

type verificationResult struct {
	stage   verificationStage
	outcome verificationOutcome
}

var runLedgerIntegrityVerifier = verifyLedgerIntegrity

var queryLedgerIntegrity = func(ctx context.Context, db *sql.DB, query string) ([]string, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
		if len(values) == 2 {
			break
		}
	}
	return values, rows.Err()
}

func verifyLedgerIntegrity(parent context.Context, db *sql.DB) verificationResult {
	ctx, cancel := context.WithTimeout(parent, time.Second)
	defer cancel()
	quick, err := queryLedgerIntegrity(ctx, db, "PRAGMA quick_check(1)")
	if err != nil || len(quick) != 1 {
		return verificationResult{stage: verificationQuickCheck, outcome: verificationInconclusive}
	}
	if quick[0] != "ok" {
		return verificationResult{stage: verificationQuickCheck, outcome: verificationCorrupt}
	}
	pages, err := queryLedgerIntegrity(ctx, db, "PRAGMA page_count")
	if err != nil || len(pages) != 1 {
		return verificationResult{stage: verificationQuickCheck, outcome: verificationInconclusive}
	}
	pageSize, err := queryLedgerIntegrity(ctx, db, "PRAGMA page_size")
	if err != nil || len(pageSize) != 1 {
		return verificationResult{stage: verificationQuickCheck, outcome: verificationInconclusive}
	}
	pageCount, countErr := strconv.ParseInt(pages[0], 10, 64)
	size, sizeErr := strconv.ParseInt(pageSize[0], 10, 64)
	if countErr != nil || sizeErr != nil || pageCount < 0 || size <= 0 || pageCount > maxLedgerBytes/size {
		return verificationResult{stage: verificationQuickCheck, outcome: verificationBoundExceeded}
	}
	integrity, err := queryLedgerIntegrity(ctx, db, "PRAGMA integrity_check(1)")
	if err != nil || len(integrity) != 1 {
		return verificationResult{stage: verificationIntegrityCheck, outcome: verificationInconclusive}
	}
	if integrity[0] != "ok" {
		return verificationResult{stage: verificationIntegrityCheck, outcome: verificationCorrupt}
	}
	return verificationResult{stage: verificationIntegrityCheck, outcome: verificationPassed}
}

type Ledger struct{ db *sql.DB }

type proof uint8

const (
	proofUnknown proof = iota
	proofYes
	proofNo
)

type filesystemLocality uint8

const (
	localityUnknown filesystemLocality = iota
	localityLocal
	localityNetwork
	localityShared
	localityContradictory
)

type filesystemEvidence struct {
	Available           proof
	ApplicationDataRoot string
	Locality            filesystemLocality
	Owner               proof
	Permissions         proof
	LinkSafe            proof
	ReparseSafe         proof
}

var queryFilesystemEvidence = filesystemEvidenceFor

func Open(root string) (*Ledger, error) {
	return open(root, queryFilesystemEvidence(root))
}

func open(root string, evidence filesystemEvidence) (*Ledger, error) {
	if !filesystemPolicyAllows(root, evidence) {
		return nil, source.ErrOwnershipInvalid
	}
	if info, err := os.Stat(root); !filepath.IsAbs(root) || err != nil || !info.IsDir() {
		return nil, source.ErrOwnershipInvalid
	}
	path := filepath.Join(root, "ownership.db")
	_, err := os.Stat(path)
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, source.ErrOwnershipInvalid
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open ownership database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ledger := &Ledger{db: db}
	if exists {
		if runLedgerIntegrityVerifier(context.Background(), db).outcome != verificationPassed {
			_ = db.Close()
			return nil, source.ErrOwnershipInvalid
		}
		err = ledger.verify(context.Background())
	} else {
		err = ledger.initialize(context.Background())
		if err == nil && runLedgerIntegrityVerifier(context.Background(), db).outcome != verificationPassed {
			err = source.ErrOwnershipInvalid
		}
	}
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return ledger, nil
}

func filesystemPolicyAllows(root string, evidence filesystemEvidence) bool {
	if evidence.Available != proofYes || evidence.Locality != localityLocal || evidence.Owner != proofYes || evidence.Permissions != proofYes || evidence.LinkSafe != proofYes || evidence.ReparseSafe != proofYes {
		return false
	}
	if !filepath.IsAbs(root) || !filepath.IsAbs(evidence.ApplicationDataRoot) {
		return false
	}
	rel, err := filepath.Rel(evidence.ApplicationDataRoot, root)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)
}

func applicationDataRoot() string {
	root, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return root
}
func (l *Ledger) Close() error { return l.db.Close() }
func (l *Ledger) Admit(ctx context.Context, record source.OwnershipRecord) error {
	for attempt := 0; ; attempt++ {
		commitAttempted, err := l.admitAttempt(ctx, record)
		if err == nil {
			return l.requireExactRecord(ctx, record)
		}
		if commitAttempted {
			found, readbackErr := l.readbackExactRecord(ctx, record)
			if readbackErr != nil {
				return readbackErr
			}
			if found {
				return nil
			}
		} else if !isSQLiteBusy(err) {
			return err
		}
		if attempt == len(transactionRetryDelays) {
			return err
		}
		if err := waitForRetry(ctx, transactionRetryDelays[attempt]); err != nil {
			return err
		}
	}
}

func (l *Ledger) admitAttempt(ctx context.Context, record source.OwnershipRecord) (commitAttempted bool, err error) {
	conn, err := l.db.Conn(ctx)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "PRAGMA busy_timeout = 250"); err != nil {
		return false, err
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return false, err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()
	var path, profile, created string
	var version int
	var digest []byte
	err = conn.QueryRowContext(ctx, `SELECT remote_path, version, profile, target_digest, created_at FROM ownership WHERE token = ?`, record.Token).
		Scan(&path, &version, &profile, &digest, &created)
	switch {
	case err == nil:
		if path != record.RemotePath || version != userVersion || profile != record.Profile || !bytes.Equal(digest, record.TargetDigest) || created != record.CreatedAt.UTC().Format(time.RFC3339) {
			return false, source.ErrOwnershipMismatch
		}
	case errors.Is(err, sql.ErrNoRows):
		var count int
		if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM ownership").Scan(&count); err != nil {
			return false, err
		}
		if count >= 64 {
			return false, source.ErrOwnershipCapacity
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO ownership (token, remote_path, version, profile, target_digest, created_at) VALUES (?, ?, ?, ?, ?, ?)`, record.Token, record.RemotePath, userVersion, record.Profile, record.TargetDigest, record.CreatedAt.UTC().Format(time.RFC3339)); err != nil {
			return false, err
		}
	default:
		return false, err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return true, err
	}
	committed = true
	return true, nil
}

func (l *Ledger) requireExactRecord(ctx context.Context, record source.OwnershipRecord) error {
	found, err := l.readbackExactRecord(ctx, record)
	if err != nil {
		return err
	}
	if !found {
		return source.ErrOwnershipInvalid
	}
	return nil
}

func (l *Ledger) readbackExactRecord(ctx context.Context, record source.OwnershipRecord) (bool, error) {
	var path, profile, created string
	var version int
	var digest []byte
	err := l.db.QueryRowContext(ctx, `SELECT remote_path, version, profile, target_digest, created_at FROM ownership WHERE token = ?`, record.Token).
		Scan(&path, &version, &profile, &digest, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if path != record.RemotePath || version != userVersion || profile != record.Profile || !bytes.Equal(digest, record.TargetDigest) || created != record.CreatedAt.UTC().Format(time.RFC3339) {
		return false, source.ErrOwnershipMismatch
	}
	return true, nil
}

func isSQLiteBusy(err error) bool {
	var sqliteErr *sqliteDriver.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code()&0xff == sqliteBusy
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func (l *Ledger) initialize(ctx context.Context) error {
	for _, statement := range []string{"PRAGMA journal_mode = DELETE", "PRAGMA synchronous = EXTRA", "PRAGMA busy_timeout = 250", fmt.Sprintf("PRAGMA application_id = %d", applicationID), fmt.Sprintf("PRAGMA user_version = %d", userVersion), ownershipSchema} {
		if _, err := l.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return l.verify(ctx)
}
func (l *Ledger) verify(ctx context.Context) error {
	var application, version, timeout, synchronous int
	var journal, schema string
	if l.db.QueryRowContext(ctx, "PRAGMA application_id").Scan(&application) != nil || l.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version) != nil || l.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journal) != nil || l.db.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&timeout) != nil || l.db.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&synchronous) != nil || l.db.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'ownership'").Scan(&schema) != nil {
		return source.ErrOwnershipInvalid
	}
	if application != applicationID || version != userVersion || journal != "delete" || timeout != 250 || synchronous != 3 || schema != ownershipSchema {
		return source.ErrOwnershipInvalid
	}
	return nil
}
