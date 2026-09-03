package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"bac-nexus/internal/localstate"
	"bac-nexus/internal/strictjson"
)

const (
	maxRetentionDays = 3650
	maxRecordBytes   = 1023
	maxSegmentBytes  = 64 << 20
)

// ErrAuditUnavailable is a sanitized failure classification for durable audit
// admission and append. It intentionally does not expose a local path or OS error.
var ErrAuditUnavailable = errors.New("audit: durable evidence unavailable")

// FileConfig admits a single owner-only audit root. RetentionDays is deliberately
// a required string so an omitted environment value cannot silently use a default.
type FileConfig struct {
	Root, RetentionDays string
	Components          []string
	Platform            localstate.SecurePathPlatform
	Now                 func() time.Time
	Write               func([]byte) (int, error) // test seam; production leaves this nil
	Sync                func() error              // test seam; production leaves this nil
	RepairSync          func(*os.File) error      // test seam; production leaves this nil
	DirectorySync       func(*os.File) error      // test seam; production leaves this nil
}

// File is a process-exclusive, newline-framed audit sink.
type File struct {
	mu               sync.Mutex
	root             string
	components       []string
	platform         localstate.SecurePathPlatform
	now              func() time.Time
	retentionDays    int
	write            func([]byte) (int, error)
	sync             func() error
	repairSync       func(*os.File) error
	directorySync    func(*os.File) error
	lock             *os.File
	segment          *os.File
	segmentDate      string
	poisoned, closed bool
}

// OpenFile validates retained evidence while holding its lifetime lock.
func OpenFile(config FileConfig) (*File, error) {
	retentionDays, err := parseRetention(config.RetentionDays)
	if err != nil || config.Root == "" || len(config.Components) == 0 || config.Platform == nil {
		return nil, ErrAuditUnavailable
	}
	root := filepath.Join(append([]string{config.Root}, config.Components...)...)
	if _, err := config.Platform.VerifyManagedDirectory(root, config.Components...); err != nil {
		return nil, ErrAuditUnavailable
	}
	lockComponents := append(append([]string{}, config.Components...), "audit.lock")
	lockPath := filepath.Join(root, "audit.lock")
	if err := os.Mkdir(lockPath, 0o700); err != nil {
		return nil, ErrAuditUnavailable
	}
	if _, err := config.Platform.VerifyManagedDirectory(lockPath, lockComponents...); err != nil {
		_ = os.Remove(lockPath)
		return nil, ErrAuditUnavailable
	}
	lock, err := os.Open(lockPath)
	if err != nil {
		_ = os.Remove(lockPath)
		return nil, ErrAuditUnavailable
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	f := &File{root: root, components: append([]string{}, config.Components...), platform: config.Platform, now: now, retentionDays: retentionDays, write: config.Write, sync: config.Sync, repairSync: config.RepairSync, directorySync: config.DirectorySync, lock: lock}
	if err := f.recover(); err != nil {
		_ = lock.Close()
		_ = os.Remove(lockPath)
		return nil, ErrAuditUnavailable
	}
	return f, nil
}

func parseRetention(value string) (int, error) {
	days, err := strconv.Atoi(value)
	if err != nil || days < 1 || days > maxRetentionDays {
		return 0, ErrAuditUnavailable
	}
	return days, nil
}

// LoadOperatorRetention reads the mandatory operator-owned audit retention
// policy. It deliberately has no fallback because serving without durable
// retention evidence is unsafe.
func LoadOperatorRetention(configRoot string, platform localstate.SecurePathPlatform) (int, error) {
	if configRoot == "" || platform == nil {
		return 0, ErrAuditUnavailable
	}
	directory := filepath.Join(configRoot, "BAC Nexus")
	if _, err := platform.VerifyManagedDirectory(directory, "BAC Nexus"); err != nil {
		return 0, ErrAuditUnavailable
	}
	path := filepath.Join(directory, "operator.json")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 {
		return 0, ErrAuditUnavailable
	}
	file, err := os.Open(path)
	if err != nil {
		return 0, ErrAuditUnavailable
	}
	defer file.Close()
	if _, err := platform.CreateManagedFile(path, "BAC Nexus", "operator.json"); err != nil {
		return 0, ErrAuditUnavailable
	}
	handleInfo, err := file.Stat()
	if err != nil {
		return 0, ErrAuditUnavailable
	}
	currentInfo, err := os.Lstat(path)
	if err != nil || !handleInfo.Mode().IsRegular() || !currentInfo.Mode().IsRegular() || currentInfo.Mode()&os.ModeSymlink != 0 || currentInfo.Mode().Perm() != 0o600 || !os.SameFile(handleInfo, currentInfo) {
		return 0, ErrAuditUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil || len(data) > 4096 || strictjson.ValidateObjectKeys(data, "audit_retention_days") != nil {
		return 0, ErrAuditUnavailable
	}
	var config struct {
		RetentionDays int `json:"audit_retention_days"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&config) != nil {
		return 0, ErrAuditUnavailable
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return 0, ErrAuditUnavailable
	}
	if _, err := parseRetention(strconv.Itoa(config.RetentionDays)); err != nil {
		return 0, ErrAuditUnavailable
	}
	return config.RetentionDays, nil
}

type diskEvent struct {
	OperationClass   string `json:"operation_class"`
	PolicyID         string `json:"policy_id"`
	ResultClass      string `json:"result_class"`
	RequestedLines   int    `json:"requested_lines"`
	ReturnedLines    int    `json:"returned_lines"`
	DurationMS       int64  `json:"duration_ms"`
	LifecycleOutcome string `json:"lifecycle_outcome"`
}

func encodeEvent(event Event) ([]byte, error) {
	if err := ValidateEvent(event); err != nil {
		return nil, err
	}
	operationClass, policyID, resultClass := "", "", ""
	switch event.Capability {
	case CapabilityCatalogResolve, CapabilitySourceRead, CapabilityLifecycleCompletion, CapabilityConfigurationDiagnostic:
		operationClass = string(event.Capability)
	}
	if event.PolicyID == PolicyIDVerifiedReadOnly {
		policyID = "verified_read_only"
	}
	switch event.Result {
	case ResultClassAllow, ResultClassDeny, ResultClassError, ResultClassSucceeded, ResultClassCancelled, ResultClassTimedOut, ResultClassFailed:
		resultClass = string(event.Result)
	}
	if operationClass == "" || policyID == "" || resultClass == "" {
		return nil, ErrAuditUnavailable
	}
	encoded, err := json.Marshal(diskEvent{operationClass, policyID, resultClass, event.Requested, event.Returned, event.Duration.Milliseconds(), "completed"})
	if err != nil || len(encoded) > maxRecordBytes {
		return nil, ErrAuditUnavailable
	}
	return append(encoded, '\n'), nil
}

// Record encodes before entering the critical section, then durably appends one
// complete record. Any storage fault poisons this sink for its remaining lifetime.
func (f *File) Record(ctx context.Context, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	record, err := encodeEvent(event)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.poisoned || f.closed {
		return ErrAuditUnavailable
	}
	if err := f.selectSegment(len(record)); err != nil {
		return f.poison()
	}
	count, writeErr := f.writeRecord(record)
	if writeErr != nil || count != len(record) {
		return f.poison()
	}
	if f.syncSegment() != nil {
		return f.poison()
	}
	return nil
}

func (f *File) selectSegment(recordBytes int) error {
	date := f.now().UTC().Format("2006-01-02")
	if f.segment != nil && f.segmentDate == date {
		info, err := f.segment.Stat()
		if err != nil || info.Size() > maxSegmentBytes-int64(recordBytes) {
			return ErrAuditUnavailable
		}
		return nil
	}
	if f.segment != nil {
		if f.syncSegment() != nil || f.segment.Close() != nil {
			return ErrAuditUnavailable
		}
		f.segment = nil
	}
	name := "audit-" + date + ".jsonl"
	components := append(append([]string{}, f.components...), name)
	path := filepath.Join(f.root, name)
	if _, err := f.platform.CreateManagedFile(path, components...); err != nil {
		return ErrAuditUnavailable
	}
	segment, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return ErrAuditUnavailable
	}
	info, err := segment.Stat()
	if err != nil || info.Size() > maxSegmentBytes-int64(recordBytes) {
		_ = segment.Close()
		return ErrAuditUnavailable
	}
	f.segment, f.segmentDate = segment, date
	return nil
}

func (f *File) recover() error {
	entries, err := os.ReadDir(f.root)
	if err != nil {
		return err
	}
	today := f.now().UTC()
	cutoff := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1-f.retentionDays)
	type segment struct {
		name string
		date time.Time
	}
	segments := make([]segment, 0, len(entries))
	for _, entry := range entries {
		if entry.Name() == "audit.lock" && entry.IsDir() {
			continue
		}
		date, ok := parseSegmentName(entry.Name())
		if !ok || !entry.Type().IsRegular() {
			return ErrAuditUnavailable
		}
		if date.After(today.UTC()) {
			return ErrAuditUnavailable
		}
		segments = append(segments, segment{name: entry.Name(), date: date})
	}
	newest := -1
	for index := range segments {
		if newest == -1 || segments[index].date.After(segments[newest].date) {
			newest = index
		}
	}
	for index, segment := range segments {
		path := filepath.Join(f.root, segment.name)
		repaired, err := f.validateSegment(path, index == newest)
		if err != nil {
			return err
		}
		if repaired {
			if err := f.syncDirectory(); err != nil {
				return err
			}
			if err := f.recordRecovery(); err != nil {
				return err
			}
		}
		if segment.date.Before(cutoff) {
			if err := os.Remove(path); err != nil {
				return err
			}
			if err := f.syncDirectory(); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseSegmentName(name string) (time.Time, bool) {
	if len(name) != len("audit-2006-01-02.jsonl") || name[:6] != "audit-" || name[16:] != ".jsonl" {
		return time.Time{}, false
	}
	date, err := time.Parse("2006-01-02", name[6:16])
	return date, err == nil && date.Format("2006-01-02") == name[6:16]
}

func (f *File) validateSegment(path string, mayRepair bool) (bool, error) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxSegmentBytes {
		return false, ErrAuditUnavailable
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	repaired := false
	if len(data) > 0 && data[len(data)-1] != '\n' {
		if !mayRepair {
			return false, ErrAuditUnavailable
		}
		lastNewline := -1
		for index := len(data) - 1; index >= 0; index-- {
			if data[index] == '\n' {
				lastNewline = index
				break
			}
		}
		if lastNewline < 0 {
			return false, ErrAuditUnavailable
		}
		data = data[:lastNewline+1]
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return false, err
		}
		count, writeErr := file.Write(data)
		syncErr, closeErr := f.syncRepair(file), file.Close()
		if writeErr != nil || count != len(data) || syncErr != nil || closeErr != nil {
			return false, ErrAuditUnavailable
		}
		repaired = true
	}
	for len(data) > 0 {
		newline := 0
		for newline < len(data) && data[newline] != '\n' {
			newline++
		}
		if newline == len(data) || newline > maxRecordBytes || !validDiskRecord(data[:newline]) {
			return false, ErrAuditUnavailable
		}
		data = data[newline+1:]
	}
	return repaired, nil
}

func validDiskRecord(data []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return false
	}
	fields := make(map[string]bool, 7)
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil || fields[key.(string)] {
			return false
		}
		fields[key.(string)] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return false
		}
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') || len(fields) != 7 {
		return false
	}
	for _, name := range []string{"operation_class", "policy_id", "result_class", "requested_lines", "returned_lines", "duration_ms", "lifecycle_outcome"} {
		if !fields[name] {
			return false
		}
	}
	var event diskEvent
	if json.Unmarshal(data, &event) != nil || event.RequestedLines < 0 || event.ReturnedLines < 0 || event.DurationMS < 0 {
		return false
	}
	if event.PolicyID != "verified_read_only" || (event.LifecycleOutcome != "completed" && event.LifecycleOutcome != "recovered") {
		return false
	}
	if event.OperationClass == string(CapabilityConfigurationDiagnostic) {
		return event.ResultClass == string(ResultClassSucceeded) || event.ResultClass == string(ResultClassCancelled) || event.ResultClass == string(ResultClassTimedOut) || event.ResultClass == string(ResultClassFailed)
	}
	return (event.OperationClass == string(CapabilityCatalogResolve) || event.OperationClass == string(CapabilitySourceRead) || event.OperationClass == string(CapabilityLifecycleCompletion)) &&
		(event.ResultClass == string(ResultClassAllow) || event.ResultClass == string(ResultClassDeny) || event.ResultClass == string(ResultClassError))
}

func (f *File) recordRecovery() error {
	record, err := json.Marshal(diskEvent{string(CapabilityCatalogResolve), "verified_read_only", string(ResultClassError), 0, 0, 0, "recovered"})
	if err != nil {
		return err
	}
	record = append(record, '\n')
	if err := f.selectSegment(len(record)); err != nil {
		return err
	}
	count, err := f.writeRecord(record)
	if err != nil || count != len(record) || f.syncSegment() != nil {
		return ErrAuditUnavailable
	}
	return nil
}

func (f *File) syncDirectory() error {
	directory, err := os.Open(f.root)
	if err != nil {
		return err
	}
	defer directory.Close()
	if f.directorySync != nil {
		return f.directorySync(directory)
	}
	return directory.Sync()
}

func (f *File) syncRepair(file *os.File) error {
	if f.repairSync != nil {
		return f.repairSync(file)
	}
	return file.Sync()
}

func (f *File) writeRecord(record []byte) (int, error) {
	if f.write != nil {
		return f.write(record)
	}
	return f.segment.Write(record)
}

func (f *File) syncSegment() error {
	if f.sync != nil {
		return f.sync()
	}
	return f.segment.Sync()
}

func (f *File) poison() error {
	f.poisoned = true
	return ErrAuditUnavailable
}

// Close syncs and closes retained resources. A poisoned sink remains a
// deterministic failure even if operating-system cleanup succeeds.
func (f *File) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed || f.poisoned {
		return ErrAuditUnavailable
	}
	f.closed = true
	if f.segment != nil && (f.syncSegment() != nil || f.segment.Close() != nil) {
		return f.poison()
	}
	if f.lock == nil || f.lock.Close() != nil || os.Remove(filepath.Join(f.root, "audit.lock")) != nil {
		return f.poison()
	}
	return nil
}

var _ Auditor = (*File)(nil)
var _ io.Closer = (*File)(nil)
