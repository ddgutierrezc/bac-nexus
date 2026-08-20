package source

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"bac-nexus/internal/catalog"
)

const cleanupTimeout = 15 * time.Second

var ErrRemoteNotFound = errors.New("remote file not found")

// AcquisitionRemote permits only the fixed copy and operations on its temporary result.
type AcquisitionRemote interface {
	CopyToUTF8(context.Context, string, string) error
	Stat(context.Context, string) (os.FileInfo, error)
	Download(context.Context, string) (io.ReadCloser, error)
	Remove(context.Context, string) error
}

// RemoteOpener supplies a separately owned remote lifecycle for request and cleanup work.
type RemoteOpener func(context.Context) (AcquisitionRemote, io.Closer, error)

// Acquirer builds a complete in-memory snapshot and confirms its remote temporary is gone.
type Acquirer struct {
	Open   RemoteOpener
	Random io.Reader
}

func (a Acquirer) Acquire(ctx context.Context, candidate catalog.Candidate) (snap *Snapshot, err error) {
	if a.Open == nil {
		return nil, errors.New("source acquisition dependencies are required")
	}
	remote, closeRemote, err := a.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("open source acquisition connection: %w", err)
	}
	defer func() {
		if closeErr := closeRemote.Close(); closeErr != nil {
			snap = nil
			err = errors.Join(err, fmt.Errorf("close source acquisition connection: %w", closeErr))
		}
	}()
	path, err := temporaryName(a.Random)
	if err != nil {
		return nil, err
	}
	qsysPath, err := candidate.QSYSPath()
	if err != nil {
		return nil, err
	}
	owned := false
	defer func() {
		if owned {
			if cleanupErr := a.cleanup(ctx, path); cleanupErr != nil {
				snap = nil
				err = errors.Join(err, cleanupErr)
			}
		}
	}()
	owned = true // CPYTOSTMF may create the fixed Nexus path even when it reports failure.
	if err = remote.CopyToUTF8(ctx, qsysPath, path); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	info, err := remote.Stat(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("stat remote source temporary file: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() < 0 {
		return nil, errors.New("remote source temporary path is not a regular file")
	}
	if info.Size() > AbsoluteMaxBytes {
		return nil, errors.New("source exceeds 4 MiB ceiling")
	}
	file, err := remote.Download(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("download remote source temporary file: %w", err)
	}
	data, readErr := io.ReadAll(io.LimitReader(file, AbsoluteMaxBytes+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if int64(len(data)) != info.Size() {
		return nil, errors.New("remote source temporary size changed during download")
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	snap, err = NewSnapshot(data)
	if err != nil {
		snap = nil
	}
	return snap, err
}

func (a Acquirer) cleanup(ctx context.Context, path string) (err error) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	remote, closeRemote, err := a.Open(cleanupCtx)
	if err != nil {
		return fmt.Errorf("open cleanup connection: %w", err)
	}
	defer func() {
		if closeErr := closeRemote.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close cleanup connection: %w", closeErr))
		}
	}()
	return removeConfirmed(cleanupCtx, remote, path)
}

func removeConfirmed(ctx context.Context, remote AcquisitionRemote, path string) error {
	removeErr := remote.Remove(ctx, path)
	_, statErr := remote.Stat(ctx, path)
	if errors.Is(statErr, ErrRemoteNotFound) {
		if removeErr == nil || errors.Is(removeErr, ErrRemoteNotFound) {
			return nil
		}
		return fmt.Errorf("remove remote temporary: %w", removeErr)
	}
	if statErr != nil {
		return errors.Join(removeErr, fmt.Errorf("confirm remote temporary cleanup: %w", statErr))
	}
	return errors.Join(removeErr, errors.New("remote temporary cleanup was not confirmed"))
}

func temporaryName(random io.Reader) (string, error) {
	if random == nil {
		random = rand.Reader
	}
	token := make([]byte, 16)
	if _, err := io.ReadFull(random, token); err != nil {
		return "", fmt.Errorf("generate remote temporary name: %w", err)
	}
	return "/tmp/bac-nexus-catalog-" + hex.EncodeToString(token) + ".utf8", nil
}

func privateDirectory(home string) (string, error) {
	if !strings.HasPrefix(home, "/") || strings.Contains(home, "..") {
		return "", errors.New("authenticated remote home is not an absolute safe path")
	}
	return path.Join(home, ".bac-nexus", "tmp"), nil
}
