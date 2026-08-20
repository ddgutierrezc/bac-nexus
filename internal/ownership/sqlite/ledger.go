package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"bac-nexus/internal/source"

	_ "modernc.org/sqlite"
)

const (
	applicationID   = 1111573326
	userVersion     = 1
	ownershipSchema = `CREATE TABLE ownership (token BLOB PRIMARY KEY CHECK(length(token) = 16), remote_path TEXT UNIQUE CHECK(length(CAST(remote_path AS BLOB)) BETWEEN 1 AND 1024), version INTEGER NOT NULL CHECK(version = 1), profile TEXT NOT NULL CHECK(length(profile) BETWEEN 1 AND 64), target_digest BLOB NOT NULL CHECK(length(target_digest) = 32), created_at TEXT NOT NULL CHECK(length(created_at) = 20))`
)

type Ledger struct{ db *sql.DB }

type filesystemEvidence struct {
	Available              bool
	ApplicationDataRoot    string
	LocalKnown             bool
	Local                  bool
	Shared                 bool
	OwnerKnown             bool
	OwnerVerified          bool
	PermissionsKnown       bool
	PermissionsRestrictive bool
	LinkKnown              bool
	Symlink                bool
	WindowsReparsePoint    bool
}

var queryFilesystemEvidence = func(string) filesystemEvidence { return filesystemEvidence{} }

func Open(root string) (*Ledger, error) {
	return open(root, queryFilesystemEvidence(root))
}

func open(root string, evidence filesystemEvidence) (*Ledger, error) {
	_ = evidence
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
		err = ledger.verify(context.Background())
	} else {
		err = ledger.initialize(context.Background())
	}
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return ledger, nil
}
func (l *Ledger) Close() error { return l.db.Close() }
func (l *Ledger) Admit(ctx context.Context, record source.OwnershipRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	conn, err := l.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
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
			return source.ErrOwnershipMismatch
		}
	case errors.Is(err, sql.ErrNoRows):
		var count int
		if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM ownership").Scan(&count); err != nil {
			return err
		}
		if count >= 64 {
			return source.ErrOwnershipCapacity
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO ownership (token, remote_path, version, profile, target_digest, created_at) VALUES (?, ?, ?, ?, ?, ?)`, record.Token, record.RemotePath, userVersion, record.Profile, record.TargetDigest, record.CreatedAt.UTC().Format(time.RFC3339)); err != nil {
			return err
		}
	default:
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
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
