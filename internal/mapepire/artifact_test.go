package mapepire

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type memoryFileInfo struct {
	size int64
	mode os.FileMode
}

func (f memoryFileInfo) Name() string       { return "jar" }
func (f memoryFileInfo) Size() int64        { return f.size }
func (f memoryFileInfo) Mode() os.FileMode  { return f.mode }
func (f memoryFileInfo) ModTime() time.Time { return time.Time{} }
func (f memoryFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f memoryFileInfo) Sys() any           { return nil }

type memoryWriteCloser struct {
	bytes.Buffer
	onClose func([]byte)
}

func (w *memoryWriteCloser) Close() error {
	w.onClose(append([]byte(nil), w.Bytes()...))
	return nil
}

type memoryRemote struct {
	files           map[string][]byte
	modes           map[string]os.FileMode
	removes         []string
	chmodErr        error
	removeErrorPath string
	renameErrAt     int
	renameErrs      map[int]error
	renames         int
	corruptFinal    bool
}

func newMemoryRemote() *memoryRemote {
	return &memoryRemote{files: map[string][]byte{}, modes: map[string]os.FileMode{}}
}

func (m *memoryRemote) WorkingDirectory() (string, error) { return "/home/NEXUS", nil }
func (m *memoryRemote) MkdirAll(string) error             { return nil }
func (m *memoryRemote) Chmod(remotePath string, _ os.FileMode) error {
	if m.chmodErr != nil && strings.Contains(remotePath, ".upload-") {
		return m.chmodErr
	}
	return nil
}
func (m *memoryRemote) Stat(remotePath string) (os.FileInfo, error) {
	mode, modeExists := m.modes[remotePath]
	data, dataExists := m.files[remotePath]
	if !modeExists && !dataExists {
		return nil, os.ErrNotExist
	}
	if !modeExists {
		mode = 0o600
	}
	return memoryFileInfo{size: int64(len(data)), mode: mode}, nil
}
func (m *memoryRemote) OpenRead(remotePath string) (io.ReadCloser, error) {
	data, ok := m.files[remotePath]
	if !ok || !m.mode(remotePath).IsRegular() {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
func (m *memoryRemote) OpenWriteExclusive(remotePath string) (io.WriteCloser, error) {
	if _, ok := m.files[remotePath]; ok {
		return nil, os.ErrExist
	}
	if _, ok := m.modes[remotePath]; ok {
		return nil, os.ErrExist
	}
	m.modes[remotePath] = 0o600
	return &memoryWriteCloser{onClose: func(data []byte) { m.files[remotePath] = data }}, nil
}
func (m *memoryRemote) Rename(oldPath, newPath string) error {
	m.renames++
	if err := m.renameErrs[m.renames]; err != nil {
		return err
	}
	if m.renameErrAt == m.renames {
		return errors.New("injected rename failure")
	}
	m.files[newPath] = m.files[oldPath]
	m.modes[newPath] = m.mode(oldPath)
	delete(m.files, oldPath)
	delete(m.modes, oldPath)
	if m.corruptFinal && strings.HasSuffix(newPath, ".jar") {
		m.files[newPath] = []byte("corrupt")
		m.corruptFinal = false
	}
	return nil
}

func TestEnsureServerJARPreservesRollbackCopyWhenRecoveryFails(t *testing.T) {
	newData := []byte("new JAR")
	oldData := []byte("prior valid JAR")
	local, expected := localJAR(t, newData)
	remote := newMemoryRemote()
	remotePath := "/home/NEXUS/" + RemoteJar
	backup := remotePath + ".rollback-" + strings.Repeat("07", 16)
	remote.files[remotePath] = append([]byte(nil), oldData...)
	remote.renameErrs = map[int]error{1: errors.New("activation failed"), 2: errors.New("recovery failed")}
	_, err := ensureServerJARWith(remote, local, expected, fixedRandom(), artifactHooks{})
	if err == nil || !strings.Contains(err.Error(), backup) {
		t.Fatalf("recoverable error = %v", err)
	}
	if !bytes.Equal(remote.files[backup], oldData) {
		t.Fatal("rollback copy was removed after recovery failure")
	}
}
func (m *memoryRemote) Remove(remotePath string) error {
	m.removes = append(m.removes, remotePath)
	if remotePath == m.removeErrorPath {
		return errors.New("injected cleanup failure")
	}
	if _, exists := m.files[remotePath]; !exists {
		if _, modeExists := m.modes[remotePath]; !modeExists {
			return os.ErrNotExist
		}
	}
	delete(m.files, remotePath)
	delete(m.modes, remotePath)
	return nil
}
func (m *memoryRemote) mode(remotePath string) os.FileMode {
	if mode, ok := m.modes[remotePath]; ok {
		return mode
	}
	return 0o600
}

func localJAR(t *testing.T, data []byte) (string, string) {
	t.Helper()
	hash := sha256.Sum256(data)
	local := filepath.Join(t.TempDir(), "mapepire.jar")
	if err := os.WriteFile(local, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return local, hex.EncodeToString(hash[:])
}

func fixedRandom() io.Reader { return bytes.NewReader(bytes.Repeat([]byte{7}, 64)) }

func TestEnsureServerJARUsesExclusiveRandomUploadAndBoundedBytes(t *testing.T) {
	data := []byte("test JAR bytes")
	local, expected := localJAR(t, data)
	remote := newMemoryRemote()
	remotePath, err := ensureServerJARWith(remote, local, expected, fixedRandom(), artifactHooks{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(remote.files[remotePath], data) {
		t.Fatal("remote bytes differ")
	}
	for remoteName := range remote.files {
		if strings.HasSuffix(remoteName, ".upload") {
			t.Fatalf("fixed upload path used: %s", remoteName)
		}
	}
}

func TestEnsureServerJARDoesNotFollowOrOverwriteExclusiveTempCollision(t *testing.T) {
	data := []byte("test JAR bytes")
	local, expected := localJAR(t, data)
	remote := newMemoryRemote()
	temporary := "/home/NEXUS/" + RemoteJar + ".upload-" + strings.Repeat("07", 16)
	remote.files[temporary] = []byte("other-account-data")
	remote.modes[temporary] = os.ModeSymlink
	_, err := ensureServerJARWith(remote, local, expected, fixedRandom(), artifactHooks{})
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("collision error = %v", err)
	}
	if string(remote.files[temporary]) != "other-account-data" {
		t.Fatal("pre-existing temporary path was overwritten or removed")
	}
}

func TestEnsureServerJARRejectsLocalAndRemoteLinkLikeArtifacts(t *testing.T) {
	t.Run("local symlink", func(t *testing.T) {
		data := []byte("test JAR bytes")
		local, expected := localJAR(t, data)
		link := local + ".link"
		if err := os.Symlink(local, link); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if _, err := ensureServerJARWith(newMemoryRemote(), link, expected, fixedRandom(), artifactHooks{}); err == nil {
			t.Fatal("local symlink was accepted")
		}
	})
	t.Run("remote link-like final", func(t *testing.T) {
		data := []byte("test JAR bytes")
		local, expected := localJAR(t, data)
		remote := newMemoryRemote()
		remotePath := "/home/NEXUS/" + RemoteJar
		remote.files[remotePath] = []byte("link target")
		remote.modes[remotePath] = os.ModeSymlink
		if _, err := ensureServerJARWith(remote, local, expected, fixedRandom(), artifactHooks{}); err == nil {
			t.Fatal("remote link-like final path was accepted")
		}
		if string(remote.files[remotePath]) != "link target" {
			t.Fatal("remote link-like path was modified")
		}
	})
}

func TestEnsureServerJARRejectsLocalReplacementAndGrowth(t *testing.T) {
	data := []byte("test JAR bytes")
	tests := []struct {
		name string
		hook func(string, *os.File) error
	}{
		{"replacement", func(local string, _ *os.File) error {
			replacement := local + ".replacement"
			if err := os.WriteFile(replacement, data, 0o600); err != nil {
				return err
			}
			if err := os.Remove(local); err != nil {
				return err
			}
			return os.Rename(replacement, local)
		}},
		{"growth", func(local string, _ *os.File) error {
			file, err := os.OpenFile(local, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = file.Write([]byte("growth"))
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			local, expected := localJAR(t, data)
			remote := newMemoryRemote()
			_, err := ensureServerJARWith(remote, local, expected, fixedRandom(), artifactHooks{afterLocalHash: tt.hook})
			if err == nil {
				t.Fatal("local race was accepted")
			}
			if len(remote.files) != 0 {
				t.Fatalf("remote artifact remains after local race: %#v", remote.files)
			}
		})
	}
}

func TestEnsureServerJARRestoresPriorArtifactOnActivationFailures(t *testing.T) {
	newData := []byte("new JAR")
	oldData := []byte("prior valid JAR")
	for _, tt := range []struct {
		name         string
		renameErrAt  int
		corruptFinal bool
	}{
		{"rename", 1, false},
		{"final verification", 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			local, expected := localJAR(t, newData)
			remote := newMemoryRemote()
			remotePath := "/home/NEXUS/" + RemoteJar
			remote.files[remotePath] = append([]byte(nil), oldData...)
			remote.renameErrAt = tt.renameErrAt
			remote.corruptFinal = tt.corruptFinal
			if _, err := ensureServerJARWith(remote, local, expected, fixedRandom(), artifactHooks{}); err == nil {
				t.Fatal("expected activation failure")
			}
			if !bytes.Equal(remote.files[remotePath], oldData) {
				t.Fatalf("prior artifact was not restored: %q", remote.files[remotePath])
			}
		})
	}
}

func TestEnsureServerJARPropagatesCleanupAfterSuccessfulActivation(t *testing.T) {
	newData := []byte("new JAR")
	local, expected := localJAR(t, newData)
	remote := newMemoryRemote()
	remotePath := "/home/NEXUS/" + RemoteJar
	remote.files[remotePath] = []byte("prior JAR")
	backup := remotePath + ".rollback-" + strings.Repeat("07", 16)
	remote.removeErrorPath = backup
	gotPath, err := ensureServerJARWith(remote, local, expected, fixedRandom(), artifactHooks{})
	if err == nil || gotPath != remotePath {
		t.Fatalf("path/error = %q/%v", gotPath, err)
	}
	if !bytes.Equal(remote.files[remotePath], newData) {
		t.Fatal("verified final artifact was not preserved")
	}
}

func TestEnsureServerJARJoinsPrimaryAndTemporaryCleanupFailures(t *testing.T) {
	data := []byte("test JAR bytes")
	local, expected := localJAR(t, data)
	remote := newMemoryRemote()
	remote.chmodErr = errors.New("injected chmod failure")
	remote.removeErrorPath = "/home/NEXUS/" + RemoteJar + ".upload-" + strings.Repeat("07", 16)
	_, err := ensureServerJARWith(remote, local, expected, fixedRandom(), artifactHooks{})
	if err == nil || !strings.Contains(err.Error(), "chmod") || !strings.Contains(err.Error(), "cleanup") {
		t.Fatalf("joined error = %v", err)
	}
}

func TestEnsureServerJARRejectsOversizedLocalFileBeforeRemoteWork(t *testing.T) {
	local := filepath.Join(t.TempDir(), "oversized.jar")
	file, err := os.Create(local)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(MaxServerJARBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	remote := newMemoryRemote()
	if _, err := ensureServerJARWith(remote, local, strings.Repeat("0", 64), fixedRandom(), artifactHooks{}); err == nil {
		t.Fatal("oversized JAR was accepted")
	}
	if len(remote.files) != 0 {
		t.Fatal("remote work occurred for oversized local JAR")
	}
}
