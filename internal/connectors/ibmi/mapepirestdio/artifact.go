package mapepirestdio

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

type RemoteFiles interface {
	WorkingDirectory() (string, error)
	MkdirAll(string) error
	Chmod(string, os.FileMode) error
	Stat(string) (os.FileInfo, error)
	OpenRead(string) (io.ReadCloser, error)
	OpenWriteExclusive(string) (io.WriteCloser, error)
	Rename(string, string) error
	Remove(string) error
}

// MapepireArtifactRemote is the authenticated remote-files capability that
// owns a verified Mapepire artifact receipt.
type MapepireArtifactRemote interface {
	RemoteFiles
	MapepireHostIdentity() string
}

// VerifiedMapepireArtifactReceipt is issued only after the fixed artifact is
// active and hashed through the authenticated remote capability.
type VerifiedMapepireArtifactReceipt struct {
	files          MapepireArtifactRemote
	hostIdentity   string
	remotePath     string
	sha256         string
	policyRevision string
}

type artifactHooks struct {
	afterLocalHash func(string, *os.File) error
}

func EnsureServerJAR(files MapepireArtifactRemote, localPath string) (VerifiedMapepireArtifactReceipt, error) {
	receipt, err := ensureServerJARReceiptWith(files, localPath, ServerSHA256, rand.Reader, artifactHooks{})
	if err != nil {
		return VerifiedMapepireArtifactReceipt{}, err
	}
	if _, err := receipt.commandPath(); err != nil {
		return VerifiedMapepireArtifactReceipt{}, err
	}
	return receipt, nil
}

func ensureServerJARReceiptWith(files MapepireArtifactRemote, localPath, expected string, random io.Reader, hooks artifactHooks) (VerifiedMapepireArtifactReceipt, error) {
	remotePath, err := ensureServerJARWith(files, localPath, expected, random, hooks)
	if err != nil {
		return VerifiedMapepireArtifactReceipt{}, err
	}
	return newArtifactReceipt(files, files.MapepireHostIdentity(), remotePath, expected), nil
}

func newArtifactReceipt(files MapepireArtifactRemote, hostIdentity, remotePath, expected string) VerifiedMapepireArtifactReceipt {
	return VerifiedMapepireArtifactReceipt{files: files, hostIdentity: hostIdentity, remotePath: remotePath, sha256: expected, policyRevision: mapepireLaunchPolicyRevision}
}

// Reverify confirms the receipt remains bound to the same authenticated
// remote capability and immediately rehashes the exact receipt path.
func (r VerifiedMapepireArtifactReceipt) Reverify(files MapepireArtifactRemote) error {
	if r.files == nil || files == nil || r.files != files || r.hostIdentity == "" || files.MapepireHostIdentity() != r.hostIdentity || r.policyRevision != mapepireLaunchPolicyRevision || !safeRemoteJARPath(r.remotePath) {
		return errors.New("Mapepire artifact receipt is invalid")
	}
	actual, err := remoteSHA256(files, r.remotePath)
	if err != nil || actual != r.sha256 {
		return errors.New("Mapepire artifact receipt verification failed")
	}
	return nil
}

// AdmitFixedStart rehashes an issued receipt before invoking the only
// injectable fixed-start seam. The callback deliberately receives no command
// or launch inputs, so it cannot become a caller-controlled execution API.
func (r VerifiedMapepireArtifactReceipt) AdmitFixedStart(files MapepireArtifactRemote, start func() error) error {
	if start == nil || r.Reverify(files) != nil {
		return errors.New("Mapepire artifact receipt is invalid")
	}
	if _, err := BuildCommand(r); err != nil {
		return errors.New("Mapepire artifact receipt is invalid")
	}
	return start()
}

func (r VerifiedMapepireArtifactReceipt) commandPath() (string, error) {
	if r.files == nil || r.hostIdentity == "" || r.sha256 != ServerSHA256 || r.policyRevision != mapepireLaunchPolicyRevision || !safeRemoteJARPath(r.remotePath) {
		return "", errors.New("Mapepire artifact receipt is invalid")
	}
	return r.remotePath, nil
}

func ensureServerJAR(files RemoteFiles, localPath, expected string) (string, error) {
	return ensureServerJARWith(files, localPath, expected, rand.Reader, artifactHooks{})
}

func ensureServerJARWith(files RemoteFiles, localPath, expected string, random io.Reader, hooks artifactHooks) (remoteJAR string, err error) {
	input, localInfo, err := openVerifiedLocalJAR(localPath, expected)
	if err != nil {
		return "", err
	}
	defer input.Close()
	if hooks.afterLocalHash != nil {
		if err := hooks.afterLocalHash(localPath, input); err != nil {
			return "", err
		}
	}
	if err := verifyLocalPathIdentity(localPath, input, localInfo); err != nil {
		return "", err
	}

	home, err := files.WorkingDirectory()
	if err != nil {
		return "", fmt.Errorf("resolve remote home: %w", err)
	}
	if !strings.HasPrefix(home, "/") || strings.Contains(home, "..") {
		return "", errors.New("remote home is not an absolute safe path")
	}
	directory := path.Join(home, path.Dir(RemoteJar))
	remotePath := path.Join(home, RemoteJar)
	if err := files.MkdirAll(directory); err != nil {
		return "", fmt.Errorf("create Nexus component directory: %w", err)
	}
	if err := files.Chmod(directory, 0o700); err != nil {
		return "", fmt.Errorf("secure Nexus component directory: %w", err)
	}

	priorHash, priorExists, err := inspectRemoteJAR(files, remotePath)
	if err != nil {
		return "", err
	}
	if priorExists && priorHash == expected {
		if err := files.Chmod(remotePath, 0o600); err != nil {
			return "", err
		}
		return remotePath, nil
	}

	temporary, err := randomRemotePath(remotePath+".upload-", random)
	if err != nil {
		return "", err
	}
	output, err := files.OpenWriteExclusive(temporary)
	if err != nil {
		return "", fmt.Errorf("create exclusive remote JAR upload: %w", err)
	}
	temporaryOwned := true
	defer func() {
		if temporaryOwned {
			if cleanupErr := files.Remove(temporary); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf("remove temporary remote JAR upload: %w", cleanupErr))
			}
		}
	}()
	if err := copyExact(output, input, localInfo.Size()); err != nil {
		_ = output.Close()
		return "", fmt.Errorf("upload remote JAR: %w", err)
	}
	if err := output.Close(); err != nil {
		return "", fmt.Errorf("close remote JAR upload: %w", err)
	}
	if err := verifyLocalIdentity(localPath, input, localInfo); err != nil {
		return "", err
	}
	if err := files.Chmod(temporary, 0o600); err != nil {
		return "", err
	}
	actual, err := remoteSHA256(files, temporary)
	if err != nil {
		return "", err
	}
	if actual != expected {
		return "", errors.New("uploaded Mapepire JAR checksum mismatch")
	}

	backup := ""
	backupOwned := false
	defer func() {
		if backupOwned {
			if cleanupErr := files.Remove(backup); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf("remove remote JAR rollback: %w", cleanupErr))
			}
		}
	}()
	if priorExists {
		backup, err = randomRemotePath(remotePath+".rollback-", random)
		if err != nil {
			return "", err
		}
		if err := copyRemoteExclusive(files, remotePath, backup, priorHash); err != nil {
			return "", fmt.Errorf("prepare remote JAR rollback: %w", err)
		}
		backupOwned = true
	}

	if priorExists {
		if err := files.Remove(remotePath); err != nil {
			return "", fmt.Errorf("prepare remote JAR activation: %w", err)
		}
	}
	if err := files.Rename(temporary, remotePath); err != nil {
		rollbackErr := restoreRemoteJAR(files, backup, remotePath, priorExists)
		if priorExists {
			backupOwned = false
		}
		return "", errors.Join(fmt.Errorf("activate remote JAR: %w", err), rollbackErr)
	}
	temporaryOwned = false
	actual, verifyErr := remoteSHA256(files, remotePath)
	if verifyErr != nil || actual != expected {
		removeErr := files.Remove(remotePath)
		rollbackErr := restoreRemoteJAR(files, backup, remotePath, priorExists)
		if priorExists {
			backupOwned = false
		}
		return "", errors.Join(errors.New("activated Mapepire JAR checksum verification failed"), removeErr, rollbackErr)
	}
	if backupOwned {
		if cleanupErr := files.Remove(backup); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
			return remotePath, fmt.Errorf("remote JAR activated; rollback cleanup pending: %w", cleanupErr)
		}
		backupOwned = false
	}
	return remotePath, nil
}

func openVerifiedLocalJAR(localPath, expected string) (*os.File, os.FileInfo, error) {
	pathInfo, err := os.Lstat(localPath)
	if err != nil {
		return nil, nil, err
	}
	if !pathInfo.Mode().IsRegular() || pathInfo.Mode()&os.ModeSymlink != 0 {
		return nil, nil, errors.New("Mapepire Server JAR is not a regular non-link file")
	}
	file, err := os.Open(localPath)
	if err != nil {
		return nil, nil, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = file.Close()
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > MaxServerJARBytes || !os.SameFile(pathInfo, info) {
		return nil, nil, errors.New("Mapepire Server JAR is not a bounded stable regular file")
	}
	hash := sha256.New()
	written, err := io.Copy(hash, io.LimitReader(file, MaxServerJARBytes+1))
	if err != nil {
		return nil, nil, err
	}
	if written != info.Size() || hex.EncodeToString(hash.Sum(nil)) != expected {
		return nil, nil, errors.New("Mapepire Server JAR checksum mismatch")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, nil, err
	}
	if err := verifyLocalPathIdentity(localPath, file, info); err != nil {
		return nil, nil, err
	}
	keep = true
	return file, info, nil
}

func verifyLocalIdentity(localPath string, file *os.File, original os.FileInfo) error {
	var extra [1]byte
	if n, err := file.Read(extra[:]); n != 0 || (err != nil && !errors.Is(err, io.EOF)) {
		return errors.New("Mapepire Server JAR grew while uploading")
	}
	return verifyLocalPathIdentity(localPath, file, original)
}

func verifyLocalPathIdentity(localPath string, file *os.File, original os.FileInfo) error {
	current, err := file.Stat()
	if err != nil {
		return err
	}
	pathInfo, err := os.Lstat(localPath)
	if err != nil {
		return errors.New("Mapepire Server JAR path changed while uploading")
	}
	if current.Size() != original.Size() || !os.SameFile(original, current) || !os.SameFile(original, pathInfo) || !pathInfo.Mode().IsRegular() {
		return errors.New("Mapepire Server JAR identity changed while uploading")
	}
	return nil
}

func copyExact(output io.Writer, input io.Reader, size int64) error {
	written, err := io.CopyN(output, input, size)
	if err != nil || written != size {
		return errors.New("Mapepire Server JAR changed or ended during bounded copy")
	}
	return nil
}

func randomRemotePath(prefix string, random io.Reader) (string, error) {
	value := make([]byte, 16)
	if _, err := io.ReadFull(random, value); err != nil {
		return "", errors.New("generate remote JAR temporary path")
	}
	return prefix + hex.EncodeToString(value), nil
}

func inspectRemoteJAR(files RemoteFiles, remotePath string) (string, bool, error) {
	info, err := files.Stat(remotePath)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > MaxServerJARBytes {
		return "", false, errors.New("remote Mapepire JAR path is not a bounded regular file")
	}
	hash, err := remoteSHA256(files, remotePath)
	return hash, true, err
}

func copyRemoteExclusive(files RemoteFiles, source, destination, expectedHash string) (err error) {
	info, err := files.Stat(source)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > MaxServerJARBytes {
		return errors.New("remote Mapepire JAR changed before rollback copy")
	}
	input, err := files.OpenRead(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := files.OpenWriteExclusive(destination)
	if err != nil {
		return err
	}
	owned := true
	defer func() {
		if owned {
			if cleanupErr := files.Remove(destination); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf("remove incomplete remote JAR rollback: %w", cleanupErr))
			}
		}
	}()
	if err := copyExact(output, input, info.Size()); err != nil {
		_ = output.Close()
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	if err := files.Chmod(destination, 0o600); err != nil {
		return err
	}
	actual, err := remoteSHA256(files, destination)
	if err != nil || actual != expectedHash {
		return errors.New("remote Mapepire JAR rollback verification failed")
	}
	owned = false
	return nil
}

func restoreRemoteJAR(files RemoteFiles, backup, remotePath string, priorExists bool) error {
	if !priorExists {
		return nil
	}
	if err := files.Rename(backup, remotePath); err != nil {
		return fmt.Errorf("restore previous remote JAR preserved at %s: %w", backup, err)
	}
	return nil
}

func remoteSHA256(files RemoteFiles, remotePath string) (string, error) {
	info, err := files.Stat(remotePath)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > MaxServerJARBytes {
		return "", errors.New("remote Mapepire JAR is not a bounded regular file")
	}
	file, err := files.OpenRead(remotePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	written, err := io.Copy(hash, io.LimitReader(file, MaxServerJARBytes+1))
	if err != nil {
		return "", err
	}
	if written != info.Size() {
		return "", errors.New("remote Mapepire JAR size changed while hashing")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
