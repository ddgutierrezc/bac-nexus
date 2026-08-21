package source

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"bac-nexus/internal/credential"
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

// RecoveryLedger permits bounded recovery enumeration and exact ownership deletion.
// It is separate from acquisition ownership because recovery never admits new rows.
type RecoveryLedger interface {
	ListRecovery(context.Context) ([]OwnershipRecord, error)
	Delete(context.Context, OwnershipRecord) error
}

type recoveryProfileResolver func(context.Context, string) (profile.Profile, error)
type recoveryCredentialGetter func(context.Context, string) ([]byte, error)

// RecoveryRemote permits only exact temporary cleanup confirmation.
type RecoveryRemote interface {
	io.Closer
	Remove(context.Context, string) error
	Stat(context.Context, string) (os.FileInfo, error)
}

type recoveryCleanupOpener func(context.Context, profile.Profile, []byte) (RecoveryRemote, error)
type recoveryReady func(context.Context, RecoveryRemote, string) error

type recoveryGuards struct {
	resolveProfile recoveryProfileResolver
	getCredential  recoveryCredentialGetter
	openCleanup    recoveryCleanupOpener
	cleanupReady   recoveryReady
}

type recoveryCoordinator struct {
	ledger         RecoveryLedger
	resolveProfile recoveryProfileResolver
	getCredential  recoveryCredentialGetter
	openCleanup    recoveryCleanupOpener
}

func (r recoveryCoordinator) Recover(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.ledger == nil || r.resolveProfile == nil || r.getCredential == nil || r.openCleanup == nil {
		return ErrOwnershipInvalid
	}
	records, err := r.ledger.ListRecovery(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		if err := recoverOwnershipRecord(ctx, record, recoveryGuards{
			resolveProfile: r.resolveProfile,
			getCredential:  r.getCredential,
			openCleanup:    r.openCleanup,
			cleanupReady: func(ctx context.Context, remote RecoveryRemote, path string) error {
				return removeConfirmed(ctx, remote, path)
			},
		}); err != nil {
			return err
		}
		if err := r.ledger.Delete(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

func recoverOwnershipRecord(ctx context.Context, record OwnershipRecord, guards recoveryGuards) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := guardRecoveryRecord(record); err != nil {
		return err
	}
	if guards.resolveProfile == nil || guards.getCredential == nil || guards.openCleanup == nil || guards.cleanupReady == nil {
		return ErrOwnershipInvalid
	}

	fresh, err := guards.resolveProfile(ctx, record.Profile)
	if err != nil || fresh.Name != record.Profile {
		return ErrOwnershipInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	secret, err := guards.getCredential(ctx, record.Profile)
	if err != nil || len(secret) == 0 || len(secret) > 4096 {
		return ErrOwnershipInvalid
	}
	defer credential.Zero(secret)
	if err := ctx.Err(); err != nil {
		return err
	}

	binding := recoveryTargetDigest(fresh)
	if subtle.ConstantTimeCompare(binding[:], record.TargetDigest) != 1 {
		return ErrOwnershipInvalid
	}
	if err := fresh.Validate(); err != nil {
		return ErrOwnershipInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	remote, err := guards.openCleanup(ctx, fresh, secret)
	if err != nil || remote == nil {
		return ErrOwnershipInvalid
	}
	defer func() {
		if closeErr := remote.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	return guards.cleanupReady(ctx, remote, record.RemotePath)
}

func recoveryTargetDigest(fresh profile.Profile) [sha256.Size]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("BAC Nexus/recovery-target-binding/v1\x00"))
	writeRecoveryBindingField(hash, fresh.Host)
	var port [2]byte
	binary.BigEndian.PutUint16(port[:], uint16(fresh.Port))
	_, _ = hash.Write(port[:])
	writeRecoveryBindingField(hash, fresh.Username)
	writeRecoveryBindingField(hash, fresh.HostKeyFingerprint)
	writeRecoveryBindingField(hash, string(fresh.HostKeyTrust))
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return digest
}

func writeRecoveryBindingField(hash io.Writer, value string) {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	_, _ = hash.Write(length[:])
	_, _ = io.WriteString(hash, value)
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
