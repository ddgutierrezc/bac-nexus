// Package onboardingdiagnostics persists the bounded, secret-free evidence
// emitted by configuration after a failed direct onboarding operation.
package onboardingdiagnostics

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/connectors/ibmi/mapepirestdio"
	"bac-nexus/internal/localstate"
)

const (
	schema        = "onboarding_failure_diagnostic/v1"
	maxAge        = 14 * 24 * time.Hour
	maxRecords    = 20
	recordFileExt = ".json"
)

var errUnavailable = errors.New("onboarding diagnostics unavailable")

type Config struct {
	UserConfigDir func() (string, error)
	Platform      localstate.ManagedFilePlatform
	Now           func() time.Time
	Random        io.Reader
	Write         func(*os.File, []byte) (int, error)
	Sync          func(*os.File) error
}

type Store struct {
	userConfigDir func() (string, error)
	platform      localstate.ManagedFilePlatform
	now           func() time.Time
	random        io.Reader
	write         func(*os.File, []byte) (int, error)
	sync          func(*os.File) error
}

func New(config Config) *Store {
	now := config.Now
	if now == nil {
		now = time.Now
	}
	randomSource := config.Random
	if randomSource == nil {
		randomSource = rand.Reader
	}
	return &Store{userConfigDir: config.UserConfigDir, platform: config.Platform, now: now, random: randomSource, write: config.Write, sync: config.Sync}
}

type diskRecord struct {
	Schema             string `json:"schema"`
	Reference          string `json:"reference"`
	TimestampUTC       string `json:"timestamp_utc"`
	Phase              string `json:"phase"`
	Class              string `json:"class"`
	CleanupRequired    bool   `json:"cleanup_required"`
	CredentialRetained bool   `json:"credential_retained"`
	ArtifactStage      string `json:"artifact_stage,omitempty"`
}

func (s *Store) Record(ctx context.Context, phase configuration.OnboardingFailurePhase, class configuration.OnboardingFailureClass, cleanupRequired, credentialRetained bool, artifactStages ...mapepirestdio.ArtifactStage) (string, error) {
	var artifactStage mapepirestdio.ArtifactStage
	if len(artifactStages) == 1 {
		artifactStage = artifactStages[0]
	}
	if ctx == nil || ctx.Err() != nil || len(artifactStages) > 1 || !validFailure(phase, class) || !validArtifactStage(phase, class, artifactStage) || s == nil || s.userConfigDir == nil || s.platform == nil {
		return "", errUnavailable
	}
	root, err := s.userConfigDir()
	if err != nil || root == "" {
		return "", errUnavailable
	}
	if ctx.Err() != nil {
		return "", errUnavailable
	}
	directory := filepath.Join(root, "BAC Nexus", "onboarding-diagnostics")
	if _, err := s.platform.VerifyManagedDirectory(directory, "BAC Nexus", "onboarding-diagnostics"); err != nil {
		return "", errUnavailable
	}
	now := s.now().UTC()
	records, err := s.readRecords(directory)
	if err != nil {
		return "", errUnavailable
	}
	if err := s.prune(directory, records, now); err != nil {
		return "", errUnavailable
	}
	active := make([]storedRecord, 0, len(records))
	for _, record := range records {
		if !record.timestamp.Before(now.Add(-maxAge)) {
			active = append(active, record)
		}
	}
	if len(active) >= maxRecords {
		sort.Slice(active, func(i, j int) bool { return active[i].timestamp.Before(active[j].timestamp) })
		if err := s.removeRecord(filepath.Join(directory, active[0].name)); err != nil {
			return "", errUnavailable
		}
	}
	reference, err := newReference(s.random)
	if err != nil {
		return "", errUnavailable
	}
	encoded, err := json.Marshal(diskRecord{Schema: schema, Reference: reference, TimestampUTC: now.Format(time.RFC3339Nano), Phase: string(phase), Class: string(class), CleanupRequired: cleanupRequired, CredentialRetained: credentialRetained, ArtifactStage: string(artifactStage)})
	if err != nil {
		return "", errUnavailable
	}
	path := filepath.Join(directory, reference+recordFileExt)
	if _, err := s.platform.CreateManagedFile(path, "BAC Nexus", "onboarding-diagnostics", reference+recordFileExt); err != nil {
		return "", errUnavailable
	}
	if err := s.writeRecord(path, encoded); err != nil {
		return "", errUnavailable
	}
	return reference, nil
}

func (s *Store) writeRecord(path string, data []byte) error {
	file, err := s.platform.OpenManagedFile(path, os.O_WRONLY, "BAC Nexus", "onboarding-diagnostics", filepath.Base(path))
	if err != nil {
		return errUnavailable
	}
	defer file.Close()
	if err := file.Truncate(0); err != nil {
		return errUnavailable
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return errUnavailable
	}
	write := s.write
	if write == nil {
		write = func(file *os.File, data []byte) (int, error) { return file.Write(data) }
	}
	count, err := write(file, data)
	if err != nil || count != len(data) {
		return errUnavailable
	}
	sync := s.sync
	if sync == nil {
		sync = func(file *os.File) error { return file.Sync() }
	}
	if err := sync(file); err != nil {
		return errUnavailable
	}
	return nil
}

type storedRecord struct {
	name      string
	timestamp time.Time
}

func (s *Store) readRecords(directory string) ([]storedRecord, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	records := make([]storedRecord, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !validFileName(name) || entry.Type()&os.ModeSymlink != 0 {
			return nil, errUnavailable
		}
		path := filepath.Join(directory, name)
		file, err := s.platform.OpenManagedFile(path, os.O_RDONLY, "BAC Nexus", "onboarding-diagnostics", name)
		if err != nil {
			return nil, errUnavailable
		}
		data, readErr := io.ReadAll(file)
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			return nil, errUnavailable
		}
		record, timestamp, err := decodeRecord(data, strings.TrimSuffix(name, recordFileExt))
		if err != nil || record.Reference+recordFileExt != name {
			return nil, errUnavailable
		}
		records = append(records, storedRecord{name: name, timestamp: timestamp})
	}
	return records, nil
}

func decodeRecord(data []byte, expectedReference string) (diskRecord, time.Time, error) {
	if !strictRecordKeys(data) {
		return diskRecord{}, time.Time{}, errUnavailable
	}
	var raw map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	if decoder.Decode(&raw) != nil || (len(raw) != 7 && len(raw) != 8) {
		return diskRecord{}, time.Time{}, errUnavailable
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return diskRecord{}, time.Time{}, errUnavailable
	}
	for _, key := range []string{"schema", "reference", "timestamp_utc", "phase", "class", "cleanup_required", "credential_retained"} {
		if _, ok := raw[key]; !ok {
			return diskRecord{}, time.Time{}, errUnavailable
		}
	}
	var record diskRecord
	if json.Unmarshal(data, &record) != nil || record.Schema != schema || record.Reference != expectedReference || !validReference(record.Reference) || !validFailure(configuration.OnboardingFailurePhase(record.Phase), configuration.OnboardingFailureClass(record.Class)) || !validArtifactStage(configuration.OnboardingFailurePhase(record.Phase), configuration.OnboardingFailureClass(record.Class), mapepirestdio.ArtifactStage(record.ArtifactStage)) {
		return diskRecord{}, time.Time{}, errUnavailable
	}
	timestamp, err := time.Parse(time.RFC3339Nano, record.TimestampUTC)
	if err != nil || timestamp.Location() != time.UTC {
		return diskRecord{}, time.Time{}, errUnavailable
	}
	return record, timestamp, nil
}

func strictRecordKeys(data []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return false
	}
	seen := make(map[string]bool, 7)
	for decoder.More() {
		key, err := decoder.Token()
		name, ok := key.(string)
		if err != nil || !ok || seen[name] || !recordKey(name) {
			return false
		}
		seen[name] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return false
		}
	}
	end, err := decoder.Token()
	return err == nil && end == json.Delim('}') && (len(seen) == 7 || len(seen) == 8)
}

func recordKey(name string) bool {
	switch name {
	case "schema", "reference", "timestamp_utc", "phase", "class", "cleanup_required", "credential_retained", "artifact_stage":
		return true
	}
	return false
}

func (s *Store) prune(directory string, records []storedRecord, now time.Time) error {
	cutoff := now.Add(-maxAge)
	for _, record := range records {
		if record.timestamp.Before(cutoff) {
			if err := s.removeRecord(filepath.Join(directory, record.name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) removeRecord(path string) error {
	return s.platform.RemoveManagedFile(path, "BAC Nexus", "onboarding-diagnostics", filepath.Base(path))
}

func newReference(source io.Reader) (string, error) {
	var value [16]byte
	if _, err := io.ReadFull(source, value[:]); err != nil {
		return "", err
	}
	return "ONB-" + hex.EncodeToString(value[:]), nil
}

func validFileName(name string) bool {
	return strings.HasSuffix(name, recordFileExt) && validReference(strings.TrimSuffix(name, recordFileExt))
}

func validReference(reference string) bool {
	if len(reference) != 36 || !strings.HasPrefix(reference, "ONB-") {
		return false
	}
	for _, value := range reference[4:] {
		if !(value >= '0' && value <= '9' || value >= 'a' && value <= 'f') {
			return false
		}
	}
	return true
}

func validFailure(phase configuration.OnboardingFailurePhase, class configuration.OnboardingFailureClass) bool {
	switch phase {
	case configuration.OnboardingPhaseHostKeyInspection:
		switch class {
		case configuration.OnboardingClassHostKeyFailure,
			configuration.OnboardingClassHostKeyTimeout,
			configuration.OnboardingClassHostKeyNegotiation,
			configuration.OnboardingClassHostKeyNoKey,
			configuration.OnboardingClassHostKeyUnavailable,
			configuration.OnboardingClassHostKeyInvalidCandidate:
			return true
		default:
			return false
		}
	case configuration.OnboardingPhaseExistingIdentity:
		return class == configuration.OnboardingClassIdentityFailure
	case configuration.OnboardingPhaseBootstrapAudit:
		return class == configuration.OnboardingClassBootstrapAuditFailure
	case configuration.OnboardingPhaseKeyringPrecondition:
		return class == configuration.OnboardingClassKeyringUnavailable
	case configuration.OnboardingPhaseCommit:
		return class == configuration.OnboardingClassCommitFailure
	case configuration.OnboardingPhaseSave:
		return class == configuration.OnboardingClassSaveFailure
	case configuration.OnboardingPhaseAuthenticatedProof:
		if class == configuration.OnboardingClassProofFailure {
			return true
		}
		return configuration.IsTerminalResult(configuration.ResultClass(class)) && configuration.ResultClass(class) != configuration.ResultCancelled
	default:
		return false
	}
}

func validArtifactStage(phase configuration.OnboardingFailurePhase, class configuration.OnboardingFailureClass, stage mapepirestdio.ArtifactStage) bool {
	return stage == "" || (phase == configuration.OnboardingPhaseAuthenticatedProof && class == configuration.OnboardingFailureClass(configuration.ResultUploadFailure) && mapepirestdio.ValidArtifactStage(stage))
}

var _ configuration.OnboardingFailureRecorder = (*Store)(nil)
