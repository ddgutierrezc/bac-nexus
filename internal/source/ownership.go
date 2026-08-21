package source

import (
	"context"
	"errors"
	"io"
	"path"
	"regexp"
	"strings"
	"time"

	"bac-nexus/internal/profile"
)

var ErrOwnershipInvalid = errors.New("ownership record is invalid")
var ErrOwnershipMismatch = errors.New("ownership record does not match existing token")
var ErrOwnershipCapacity = errors.New("ownership ledger capacity exceeded")

var ownershipProfilePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type OwnershipRecord struct {
	Token        []byte
	RemotePath   string
	Profile      string
	TargetDigest []byte
	CreatedAt    time.Time
}
type OwnershipLedger interface {
	Admit(context.Context, OwnershipRecord) error
	Delete(context.Context, OwnershipRecord) error
	Close() error
}

type recoveryProfileResolver func(context.Context, string) (profile.Profile, error)
type recoveryCredentialGetter func(context.Context, string) ([]byte, error)
type recoveryCleanupRemote interface{ io.Closer }
type recoveryCleanupOpener func(context.Context, profile.Profile, []byte) (recoveryCleanupRemote, error)
type recoveryReady func(context.Context, recoveryCleanupRemote, string) error

type recoveryGuards struct {
	resolveProfile recoveryProfileResolver
	getCredential  recoveryCredentialGetter
	openCleanup    recoveryCleanupOpener
	cleanupReady   recoveryReady
}

// recoverOwnershipRecord is the package-private Slice B seam for fresh recovery guards.
// It intentionally fails closed until the guard chain is implemented.
func recoverOwnershipRecord(context.Context, OwnershipRecord, recoveryGuards) error {
	return ErrOwnershipInvalid
}

func guardRecoveryRecord(record OwnershipRecord) error {
	if len(record.Token) != 16 || !privateRecoveryPath(record.RemotePath) || !ownershipProfilePattern.MatchString(record.Profile) || len(record.TargetDigest) != 32 || !canonicalOwnershipTime(record.CreatedAt) {
		return ErrOwnershipInvalid
	}
	return nil
}

func privateRecoveryPath(remotePath string) bool {
	if len(remotePath) == 0 || len([]byte(remotePath)) > 1024 || strings.ContainsAny(remotePath, "\x00\\\r\n") || !path.IsAbs(remotePath) || path.Clean(remotePath) != remotePath {
		return false
	}
	directory := path.Dir(remotePath)
	namespace := path.Dir(directory)
	home := path.Dir(namespace)
	return path.Base(remotePath) != "." && path.Base(directory) == "tmp" && path.Base(namespace) == ".bac-nexus" && home != "/" && path.IsAbs(home) && path.Clean(home) == home
}

func canonicalOwnershipTime(createdAt time.Time) bool {
	if createdAt.IsZero() || createdAt.Location() != time.UTC || createdAt.Nanosecond() != 0 {
		return false
	}
	canonical, err := time.Parse(time.RFC3339, createdAt.Format(time.RFC3339))
	return err == nil && canonical == createdAt
}
