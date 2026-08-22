package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxListLimit = MaxListLimit

var (
	ErrInvalidRoot         = errors.New("invalid profile store root")
	ErrInvalidUpdateTarget = errors.New("invalid profile update target")
	ErrProfileNotFound     = errors.New("profile not found")
)

// DeleteConfirmation is the exact, case-sensitive confirmation required
// before a profile is moved to its retained backup.
type DeleteConfirmation string

// CredentialDeleteConfirmation is deliberately separate from
// DeleteConfirmation so profile deletion never implies credential deletion.
type CredentialDeleteConfirmation string

type CredentialOutcome string

const (
	CredentialOutcomeUntouched CredentialOutcome = "untouched"
	CredentialOutcomeDeleted   CredentialOutcome = "deleted"
	CredentialOutcomeFailed    CredentialOutcome = "failed"
)

// ProfileDeleteResult contains only sanitized durable outcomes.
type ProfileDeleteResult struct {
	Deleted           bool
	BackupPath        string
	CredentialOutcome CredentialOutcome
}

// ProfileUpdateResult describes only the durable outcome of an update.
// It intentionally contains no filesystem path or profile field.
type ProfileUpdateResult struct {
	ReplacementCommitted bool
	PreviousBackup       string
	FileReplaced         bool
}

// List returns valid profiles in deterministic name order. Invalid entries
// are ignored so one damaged file cannot expose paths or block the list.
func (s Store) List(limit int) ([]Profile, error) {
	if limit < 1 || limit > maxListLimit {
		return nil, fmt.Errorf("list profiles: %w", ErrInvalidUpdateTarget)
	}
	if err := s.verifyRoot(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.Root)
	if err != nil {
		return nil, errors.New("list profiles: store unavailable")
	}
	profiles := make([]Profile, 0, limit)
	for _, entry := range entries {
		if len(profiles) == limit {
			break
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		base := strings.TrimSuffix(name, ".json")
		if !namePattern.MatchString(base) {
			continue
		}
		full := filepath.Join(s.Root, name)
		info, err := os.Lstat(full)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&fs.ModeSymlink != 0 {
			continue
		}
		data, err := readBounded(full)
		if err != nil {
			continue
		}
		var p Profile
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&p); err != nil || p.Name != base || p.Validate() != nil {
			continue
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			continue
		}
		profiles = append(profiles, p)
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
	return profiles, nil
}

// Update validates before changing any file, creates a restorable backup,
// and atomically publishes a 0600 replacement. A failed publication rolls
// the prior live file back from the backup.
func (s Store) Update(p Profile, previousName string) (ProfileUpdateResult, error) {
	if err := p.Validate(); err != nil {
		return ProfileUpdateResult{}, fmt.Errorf("update profile: %w", err)
	}
	if previousName == "" || !namePattern.MatchString(previousName) || p.Name != previousName {
		return ProfileUpdateResult{}, fmt.Errorf("update profile: %w", ErrInvalidUpdateTarget)
	}
	if err := s.verifyRoot(); err != nil {
		return ProfileUpdateResult{}, err
	}
	live := filepath.Join(s.Root, previousName+".json")
	backup := filepath.Join(s.Root, previousName+".bak")
	liveInfo, err := os.Lstat(live)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ProfileUpdateResult{}, ErrProfileNotFound
		}
		return ProfileUpdateResult{}, fmt.Errorf("update profile: %w", ErrInvalidUpdateTarget)
	}
	if !liveInfo.Mode().IsRegular() || liveInfo.Mode()&fs.ModeSymlink != 0 {
		return ProfileUpdateResult{}, fmt.Errorf("update profile: %w", ErrInvalidUpdateTarget)
	}
	if _, err := s.Load(previousName); err != nil {
		return ProfileUpdateResult{}, fmt.Errorf("update profile: %w", ErrInvalidUpdateTarget)
	}
	if info, err := os.Lstat(backup); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&fs.ModeSymlink != 0 {
			return ProfileUpdateResult{}, fmt.Errorf("update profile: %w", ErrInvalidUpdateTarget)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return ProfileUpdateResult{}, fmt.Errorf("update profile: %w", ErrInvalidUpdateTarget)
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return ProfileUpdateResult{}, errors.New("update profile: serialization failed")
	}
	data = append(data, '\n')
	if err := s.atomicReplace(live, backup); err != nil {
		return ProfileUpdateResult{}, errors.New("update profile: backup failed")
	}
	restored := false
	defer func() {
		if !restored {
			return
		}
		_, _ = os.Stat(live)
	}()
	restore := func() {
		_ = s.atomicReplace(backup, live)
		restored = true
	}
	temp, err := s.writeTemp(data)
	if err != nil {
		restore()
		return ProfileUpdateResult{}, errors.New("update profile: temporary write failed")
	}
	if err := s.atomicReplace(temp, live); err != nil {
		_ = os.Remove(temp)
		restore()
		return ProfileUpdateResult{}, errors.New("update profile: replacement failed")
	}
	if err := os.Chmod(live, 0o600); err != nil {
		restore()
		return ProfileUpdateResult{}, errors.New("update profile: permission update failed")
	}
	if err := os.Chmod(backup, 0o600); err != nil {
		return ProfileUpdateResult{}, errors.New("update profile: backup permission failed")
	}
	return ProfileUpdateResult{ReplacementCommitted: true, PreviousBackup: previousName + ".bak", FileReplaced: true}, nil
}

// Read loads a profile while mapping all not-found and invalid-name cases to
// the sanitized ErrProfileNotFound classification.
func (s Store) Read(name string) (Profile, error) {
	if !namePattern.MatchString(name) {
		return Profile{}, ErrProfileNotFound
	}
	if err := s.verifyRoot(); err != nil {
		return Profile{}, err
	}
	p, err := s.Load(name)
	if errors.Is(err, os.ErrNotExist) {
		return Profile{}, ErrProfileNotFound
	}
	if err != nil {
		return Profile{}, fmt.Errorf("read profile: %w", ErrProfileNotFound)
	}
	return p, nil
}

// Delete requires an exact confirmation, validates the existing live and
// backup entries, then moves the live profile to its in-root backup. The
// native credential is intentionally not touched here.
func (s Store) Delete(name string, confirmation DeleteConfirmation) (ProfileDeleteResult, error) {
	if !namePattern.MatchString(name) || confirmation != DeleteConfirmation("delete "+name) {
		return ProfileDeleteResult{}, ErrProfileNotFound
	}
	if err := s.verifyRoot(); err != nil {
		return ProfileDeleteResult{}, err
	}
	live := filepath.Join(s.Root, name+".json")
	backup := filepath.Join(s.Root, name+".bak")
	if err := s.requireRegular(live); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ProfileDeleteResult{}, ErrProfileNotFound
		}
		return ProfileDeleteResult{}, fmt.Errorf("delete profile: %w", ErrInvalidUpdateTarget)
	}
	if err := s.requireOptionalRegular(backup); err != nil {
		return ProfileDeleteResult{}, fmt.Errorf("delete profile: %w", ErrInvalidUpdateTarget)
	}
	if err := s.atomicReplace(live, backup); err != nil {
		return ProfileDeleteResult{}, errors.New("delete profile: backup unavailable")
	}
	return ProfileDeleteResult{Deleted: true, BackupPath: name + ".bak", CredentialOutcome: CredentialOutcomeUntouched}, nil
}

// Restore moves the retained backup back to the live profile after a
// downstream operation fails. Both paths must remain safe in the store root.
func (s Store) Restore(name string) error {
	if !namePattern.MatchString(name) {
		return ErrProfileNotFound
	}
	if err := s.verifyRoot(); err != nil {
		return err
	}
	live := filepath.Join(s.Root, name+".json")
	backup := filepath.Join(s.Root, name+".bak")
	if err := s.requireRegular(backup); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrProfileNotFound
		}
		return fmt.Errorf("restore profile: %w", ErrInvalidUpdateTarget)
	}
	if err := s.requireOptionalRegular(live); err != nil {
		return fmt.Errorf("restore profile: %w", ErrInvalidUpdateTarget)
	}
	if err := s.atomicReplace(backup, live); err != nil {
		return errors.New("restore profile failed")
	}
	return nil
}

func (s Store) requireRegular(path string) error {
	if !s.inRoot(path) {
		return ErrInvalidUpdateTarget
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&fs.ModeSymlink != 0 {
		return ErrInvalidUpdateTarget
	}
	return nil
}

func (s Store) requireOptionalRegular(path string) error {
	err := s.requireRegular(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s Store) verifyRoot() error {
	if s.Root == "" {
		return ErrInvalidRoot
	}
	info, err := os.Lstat(s.Root)
	if err != nil || info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsDir() {
		return ErrInvalidRoot
	}
	return nil
}

func readBounded(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxProfileBytes+1))
	if err != nil || len(data) > maxProfileBytes {
		return nil, errors.New("profile entry unavailable")
	}
	return data, nil
}

func (s Store) writeTemp(data []byte) (string, error) {
	temp, err := os.CreateTemp(s.Root, ".profile-*.tmp")
	if err != nil {
		return "", err
	}
	path := temp.Name()
	cleanup := func() { _ = temp.Close(); _ = os.Remove(path) }
	if err := temp.Chmod(0o600); err != nil {
		cleanup()
		return "", err
	}
	if _, err := temp.Write(data); err != nil {
		cleanup()
		return "", err
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return "", err
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func (s Store) atomicReplace(source, destination string) error {
	if !s.inRoot(source) || !s.inRoot(destination) {
		return ErrInvalidUpdateTarget
	}
	if err := s.requireRegular(source); err != nil {
		return err
	}
	if err := os.Rename(source, destination); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrExist) {
		return err
	}
	sidecar := destination + ".swap"
	if err := s.requireRegular(destination); err != nil {
		return ErrInvalidUpdateTarget
	}
	if err := s.requireOptionalRegular(sidecar); err != nil {
		return ErrInvalidUpdateTarget
	}
	if err := os.Rename(destination, sidecar); err != nil {
		return err
	}
	if err := os.Rename(source, destination); err != nil {
		_ = os.Rename(sidecar, destination)
		return err
	}
	if err := os.Remove(sidecar); err != nil {
		return errors.New("atomic replacement durability is ambiguous")
	}
	return nil
}

func (s Store) inRoot(path string) bool {
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return false
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
