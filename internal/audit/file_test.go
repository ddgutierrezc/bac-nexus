package audit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bac-nexus/internal/localstate"
)

func TestOpenFileRecordsCanonicalBoundedEvent(t *testing.T) {
	root := t.TempDir()
	sink, err := OpenFile(FileConfig{
		Root:          root,
		Components:    []string{"audit"},
		RetentionDays: "30",
		Platform:      testPlatform{root: root},
		Now:           func() time.Time { return time.Date(2026, 9, 2, 10, 0, 0, 0, time.FixedZone("offset", -6*60*60)) },
	})
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	t.Cleanup(func() { _ = sink.Close() })

	event := validAuditEvent()
	if err := sink.Record(context.Background(), event); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "audit", "audit-2026-09-02.jsonl"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' || len(data) > 1024 {
		t.Fatalf("record framing/size = %d bytes, want newline-framed <= 1024", len(data))
	}
	want := "{\"operation_class\":\"catalog_resolve\",\"policy_id\":\"verified_read_only\",\"result_class\":\"allow\",\"requested_lines\":50,\"returned_lines\":7,\"duration_ms\":250,\"lifecycle_outcome\":\"completed\"}\n"
	if got := string(data); got != want {
		t.Fatalf("audit record = %q, want %q", got, want)
	}
	for _, legacy := range []string{"capability", "connector", "target_class", "result", "requested", "returned", "timestamp", "reason"} {
		if strings.Contains(string(data), "\""+legacy+"\"") {
			t.Fatalf("persisted record contains legacy or sensitive field %q", legacy)
		}
	}
}

func TestEncodeEventMapsSourceDenialWithoutLegacyFields(t *testing.T) {
	event := validAuditEvent()
	event.Capability, event.TargetClass, event.Result = CapabilitySourceRead, TargetClassIBMiSource, ResultClassDeny
	event.Requested, event.Returned, event.Duration = 3, 0, time.Millisecond
	data, err := encodeEvent(event)
	if err != nil {
		t.Fatalf("encodeEvent() error = %v", err)
	}
	want := "{\"operation_class\":\"source_read\",\"policy_id\":\"verified_read_only\",\"result_class\":\"deny\",\"requested_lines\":3,\"returned_lines\":0,\"duration_ms\":1,\"lifecycle_outcome\":\"completed\"}\n"
	if got := string(data); got != want {
		t.Fatalf("audit record = %q, want %q", got, want)
	}
}

func TestOpenFileReopensAfterLifecycleCompletionRecord(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	config := FileConfig{
		Root:          root,
		Components:    []string{"audit"},
		RetentionDays: "30",
		Platform:      testPlatform{root: root},
		Now:           func() time.Time { return now },
	}
	sink, err := OpenFile(config)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	event := validAuditEvent()
	event.Capability = CapabilityLifecycleCompletion
	event.TargetClass = TargetClassLifecycle
	if err := sink.Record(context.Background(), event); err != nil {
		t.Fatalf("Record(lifecycle completion) error = %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if reopened, err := OpenFile(config); err != nil {
		t.Fatalf("OpenFile() after lifecycle completion error = %v", err)
	} else if err := reopened.Close(); err != nil {
		t.Fatalf("reopened Close() error = %v", err)
	}
}

func TestOpenFileRejectsMissingOrInvalidRetention(t *testing.T) {
	for _, retention := range []string{"", "0", "3651", "thirty"} {
		t.Run(retention, func(t *testing.T) {
			_, err := OpenFile(FileConfig{Root: t.TempDir(), Components: []string{"audit"}, RetentionDays: retention, Platform: testPlatform{root: t.TempDir()}})
			if !errors.Is(err, ErrAuditUnavailable) {
				t.Fatalf("OpenFile(%q) error = %v, want ErrAuditUnavailable", retention, err)
			}
		})
	}
}

func TestLoadOperatorRetentionAcceptsOnlySecureStrictConfiguration(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "BAC Nexus", "operator.json")
	if _, err := LoadOperatorRetention(root, testPlatform{root: root}); !errors.Is(err, ErrAuditUnavailable) {
		t.Fatalf("LoadOperatorRetention() missing file error = %v, want ErrAuditUnavailable", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	for _, tt := range []struct {
		name string
		data string
		want int
		ok   bool
	}{
		{"valid", `{"audit_retention_days":30}`, 30, true},
		{"missing field", `{}`, 0, false},
		{"unknown field", `{"audit_retention_days":30,"unexpected":1}`, 0, false},
		{"duplicate field", `{"audit_retention_days":30,"audit_retention_days":31}`, 0, false},
		{"trailing value", `{"audit_retention_days":30}{}`, 0, false},
		{"out of range", `{"audit_retention_days":0}`, 0, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tt.data), 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			got, err := LoadOperatorRetention(root, testPlatform{root: root})
			if tt.ok {
				if err != nil || got != tt.want {
					t.Fatalf("LoadOperatorRetention() = %d, %v; want %d, nil", got, err, tt.want)
				}
				return
			}
			if !errors.Is(err, ErrAuditUnavailable) {
				t.Fatalf("LoadOperatorRetention() error = %v, want ErrAuditUnavailable", err)
			}
		})
	}
	if err := os.WriteFile(path, []byte(`{"audit_retention_days":30}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}
	if _, err := LoadOperatorRetention(root, testPlatform{root: root}); !errors.Is(err, ErrAuditUnavailable) {
		t.Fatalf("LoadOperatorRetention() unsafe mode error = %v, want ErrAuditUnavailable", err)
	}
}

func TestLoadOperatorRetentionRejectsReplacementAfterPlatformEvidence(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "BAC Nexus", "operator.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"audit_retention_days":30}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	replacement := filepath.Join(root, "replacement.json")
	if err := os.WriteFile(replacement, []byte(`{"audit_retention_days":31}`), 0o600); err != nil {
		t.Fatalf("WriteFile() replacement error = %v", err)
	}
	platform := swapAfterEvidencePlatform{testPlatform: testPlatform{root: root}, swap: func() {
		if err := os.Rename(replacement, path); err != nil {
			t.Fatalf("Rename() error = %v", err)
		}
	}}
	if got, err := LoadOperatorRetention(root, platform); !errors.Is(err, ErrAuditUnavailable) || got != 0 {
		t.Fatalf("LoadOperatorRetention() = %d, %v; want 0, ErrAuditUnavailable", got, err)
	}
}

func TestFilePoisonedAfterWriteFailure(t *testing.T) {
	root := t.TempDir()
	sink, err := OpenFile(FileConfig{Root: root, Components: []string{"audit"}, RetentionDays: "1", Platform: testPlatform{root: root}, Write: func([]byte) (int, error) { return 0, nil }})
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	if err := sink.Record(context.Background(), validAuditEvent()); !errors.Is(err, ErrAuditUnavailable) {
		t.Fatalf("first Record() error = %v, want ErrAuditUnavailable", err)
	}
	if err := sink.Record(context.Background(), validAuditEvent()); !errors.Is(err, ErrAuditUnavailable) {
		t.Fatalf("poisoned Record() error = %v, want ErrAuditUnavailable", err)
	}
	if err := sink.Close(); !errors.Is(err, ErrAuditUnavailable) {
		t.Fatalf("poisoned Close() error = %v, want ErrAuditUnavailable", err)
	}
}

func TestFilePoisonsAfterPositiveShortWrite(t *testing.T) {
	root := t.TempDir()
	writes := 0
	sink, err := OpenFile(FileConfig{
		Root:          root,
		Components:    []string{"audit"},
		RetentionDays: "1",
		Platform:      testPlatform{root: root},
		Write: func(record []byte) (int, error) {
			writes++
			return len(record) - 1, nil
		},
	})
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	if err := sink.Record(context.Background(), validAuditEvent()); !errors.Is(err, ErrAuditUnavailable) {
		t.Fatalf("first Record() error = %v, want ErrAuditUnavailable", err)
	}
	if writes != 1 {
		t.Fatalf("writes after positive short write = %d, want 1", writes)
	}
	if err := sink.Record(context.Background(), validAuditEvent()); !errors.Is(err, ErrAuditUnavailable) {
		t.Fatalf("poisoned Record() error = %v, want ErrAuditUnavailable", err)
	}
	if writes != 1 {
		t.Fatalf("writes after poisoned Record() = %d, want 1", writes)
	}
	if err := sink.Close(); !errors.Is(err, ErrAuditUnavailable) {
		t.Fatalf("poisoned Close() error = %v, want ErrAuditUnavailable", err)
	}
}

func TestFileDoesNotLeakSensitiveValues(t *testing.T) {
	root := t.TempDir()
	sink, err := OpenFile(FileConfig{Root: root, Components: []string{"audit"}, RetentionDays: "1", Platform: testPlatform{root: root}})
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer sink.Close()
	event := validAuditEvent()
	event.Reason = "path /sensitive"
	err = sink.Record(context.Background(), event)
	if !errors.Is(err, ErrReasonRejected) || strings.Contains(err.Error(), "/sensitive") {
		t.Fatalf("Record() error = %v, want sanitized ErrReasonRejected", err)
	}
}

func TestOpenFileKeepsExclusiveLockAndRotatesAtUTCBoundary(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 2, 23, 59, 0, 0, time.FixedZone("west", -2*60*60))
	config := FileConfig{Root: root, Components: []string{"audit"}, RetentionDays: "1", Platform: testPlatform{root: root}, Now: func() time.Time { return now }}
	sink, err := OpenFile(config)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	if _, err := OpenFile(config); !errors.Is(err, ErrAuditUnavailable) {
		t.Fatalf("second OpenFile() error = %v, want exclusive ErrAuditUnavailable", err)
	}
	if err := sink.Record(context.Background(), validAuditEvent()); err != nil {
		t.Fatalf("first Record() error = %v", err)
	}
	now = now.Add(23 * time.Hour)
	if err := sink.Record(context.Background(), validAuditEvent()); err != nil {
		t.Fatalf("rotated Record() error = %v", err)
	}
	for _, name := range []string{"audit-2026-09-03.jsonl", "audit-2026-09-04.jsonl"} {
		if _, err := os.Stat(filepath.Join(root, "audit", name)); err != nil {
			t.Fatalf("UTC segment %q missing: %v", name, err)
		}
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestFilePoisonsAtSegmentCap(t *testing.T) {
	root := t.TempDir()
	sink, err := OpenFile(FileConfig{Root: root, Components: []string{"audit"}, RetentionDays: "1", Platform: testPlatform{root: root}})
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	path := filepath.Join(root, "audit", "audit-"+time.Now().UTC().Format("2006-01-02")+".jsonl")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Truncate(path, maxSegmentBytes); err != nil {
		t.Fatalf("Truncate() error = %v", err)
	}
	if err := sink.Record(context.Background(), validAuditEvent()); !errors.Is(err, ErrAuditUnavailable) {
		t.Fatalf("Record() error = %v, want cap failure", err)
	}
}

func TestFileDoesNotWritePastSegmentCap(t *testing.T) {
	root := t.TempDir()
	sink, err := OpenFile(FileConfig{Root: root, Components: []string{"audit"}, RetentionDays: "1", Platform: testPlatform{root: root}})
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	record, err := encodeEvent(validAuditEvent())
	if err != nil {
		t.Fatalf("encodeEvent() error = %v", err)
	}
	path := filepath.Join(root, "audit", "audit-"+time.Now().UTC().Format("2006-01-02")+".jsonl")
	before := int64(maxSegmentBytes-len(record)) + 1
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Truncate(path, before); err != nil {
		t.Fatalf("Truncate() error = %v", err)
	}
	if err := sink.Record(context.Background(), validAuditEvent()); !errors.Is(err, ErrAuditUnavailable) {
		t.Fatalf("Record() error = %v, want cap failure", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Size() != before {
		t.Fatalf("segment size after rejected append = %d, want %d", info.Size(), before)
	}
}

func TestFilePoisonsAfterSyncFailure(t *testing.T) {
	root := t.TempDir()
	sink, err := OpenFile(FileConfig{Root: root, Components: []string{"audit"}, RetentionDays: "1", Platform: testPlatform{root: root}, Sync: func() error { return errors.New("disk failure") }})
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	if err := sink.Record(context.Background(), validAuditEvent()); !errors.Is(err, ErrAuditUnavailable) {
		t.Fatalf("Record() error = %v, want sync failure", err)
	}
	if err := sink.Record(context.Background(), validAuditEvent()); !errors.Is(err, ErrAuditUnavailable) {
		t.Fatalf("poisoned Record() error = %v, want deterministic failure", err)
	}
}

func TestOpenFileRepairsOnlyNewestTornTailAndRecordsRecovery(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	path := filepath.Join(root, "audit", "audit-2026-09-02.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	valid, err := encodeEvent(validAuditEvent())
	if err != nil {
		t.Fatalf("encodeEvent() error = %v", err)
	}
	if err := os.WriteFile(path, append(valid, []byte(`{"operation_class":"catalog_resolve"}`)...), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	sink, err := OpenFile(FileConfig{Root: root, Components: []string{"audit"}, RetentionDays: "2", Platform: testPlatform{root: root}, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	t.Cleanup(func() { _ = sink.Close() })

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.HasSuffix(string(data), "\n") || !strings.Contains(string(data), `"lifecycle_outcome":"recovered"`) {
		t.Fatalf("repaired segment = %q, want framed durable recovery record", data)
	}
}

func TestOpenFileRejectsOldTornTailAndUnknownOrCorruptSegments(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	valid, err := encodeEvent(validAuditEvent())
	if err != nil {
		t.Fatalf("encodeEvent() error = %v", err)
	}
	for name, data := range map[string][]byte{
		"audit-2026-09-01.jsonl": append(valid, []byte(`{"broken"`)...),
		"audit-2026-09-02.jsonl": []byte("not-json\n"),
		"unexpected.jsonl":       valid,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(root, t.Name(), "audit", name)
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatalf("MkdirAll() error = %v", err)
			}
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			if name == "audit-2026-09-01.jsonl" {
				if err := os.WriteFile(filepath.Join(filepath.Dir(path), "audit-2026-09-02.jsonl"), valid, 0o600); err != nil {
					t.Fatalf("WriteFile(newest) error = %v", err)
				}
			}
			_, err := OpenFile(FileConfig{Root: filepath.Join(root, t.Name()), Components: []string{"audit"}, RetentionDays: "2", Platform: testPlatform{root: filepath.Join(root, t.Name())}, Now: func() time.Time { return now }})
			if !errors.Is(err, ErrAuditUnavailable) {
				t.Fatalf("OpenFile() error = %v, want ErrAuditUnavailable", err)
			}
		})
	}
}

func TestOpenFileDeletesExpiredSegmentsUsingValidatedUTCDates(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	dir := filepath.Join(root, "audit")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	valid, err := encodeEvent(validAuditEvent())
	if err != nil {
		t.Fatalf("encodeEvent() error = %v", err)
	}
	for _, name := range []string{"audit-2026-08-30.jsonl", "audit-2026-09-01.jsonl"} {
		if err := os.WriteFile(filepath.Join(dir, name), valid, 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", name, err)
		}
	}
	sink, err := OpenFile(FileConfig{Root: root, Components: []string{"audit"}, RetentionDays: "2", Platform: testPlatform{root: root}, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	t.Cleanup(func() { _ = sink.Close() })
	if _, err := os.Stat(filepath.Join(dir, "audit-2026-08-30.jsonl")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired segment stat error = %v, want not exist", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "audit-2026-09-01.jsonl")); err != nil {
		t.Fatalf("retained segment stat error = %v", err)
	}
}

func TestOpenFileRejectsDuplicateSchemaField(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "audit")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	record := []byte(`{"operation_class":"catalog_resolve","operation_class":"source_read","policy_id":"verified_read_only","result_class":"allow","requested_lines":0,"returned_lines":0,"duration_ms":0,"lifecycle_outcome":"completed"}` + "\n")
	if err := os.WriteFile(filepath.Join(dir, "audit-2026-09-02.jsonl"), record, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	_, err := OpenFile(FileConfig{Root: root, Components: []string{"audit"}, RetentionDays: "2", Platform: testPlatform{root: root}, Now: func() time.Time { return time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC) }})
	if !errors.Is(err, ErrAuditUnavailable) {
		t.Fatalf("OpenFile() error = %v, want ErrAuditUnavailable", err)
	}
}

func TestOpenFileRejectsRepairAndRecoveryDurabilityFailures(t *testing.T) {
	valid, err := encodeEvent(validAuditEvent())
	if err != nil {
		t.Fatalf("encodeEvent() error = %v", err)
	}
	for name, configure := range map[string]func(*FileConfig){
		"repair file sync": func(config *FileConfig) {
			config.RepairSync = func(*os.File) error { return errors.New("repair sync") }
		},
		"directory sync": func(config *FileConfig) {
			config.DirectorySync = func(*os.File) error { return errors.New("directory sync") }
		},
		"recovery record": func(config *FileConfig) { config.Sync = func() error { return errors.New("recovery sync") } },
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "audit")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatalf("MkdirAll() error = %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, "audit-2026-09-02.jsonl"), append(valid, []byte(`{"operation_class":"catalog_resolve"}`)...), 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			config := FileConfig{Root: root, Components: []string{"audit"}, RetentionDays: "2", Platform: testPlatform{root: root}, Now: func() time.Time { return time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC) }}
			configure(&config)
			if _, err := OpenFile(config); !errors.Is(err, ErrAuditUnavailable) {
				t.Fatalf("OpenFile() error = %v, want ErrAuditUnavailable", err)
			}
		})
	}
}

func TestOpenFileRejectsDirectorySyncFailureAfterRetentionDeletion(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "audit")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	valid, err := encodeEvent(validAuditEvent())
	if err != nil {
		t.Fatalf("encodeEvent() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "audit-2026-08-30.jsonl"), valid, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	config := FileConfig{Root: root, Components: []string{"audit"}, RetentionDays: "2", Platform: testPlatform{root: root}, Now: func() time.Time { return time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC) }, DirectorySync: func(*os.File) error { return errors.New("directory sync") }}
	if _, err := OpenFile(config); !errors.Is(err, ErrAuditUnavailable) {
		t.Fatalf("OpenFile() error = %v, want ErrAuditUnavailable", err)
	}
}

func TestOpenFileRejectsInvalidHistoricalRecords(t *testing.T) {
	valid := `{"operation_class":"catalog_resolve","policy_id":"verified_read_only","result_class":"allow","requested_lines":0,"returned_lines":0,"duration_ms":0,"lifecycle_outcome":"completed"}`
	for name, record := range map[string]string{
		"unknown field":            valid[:len(valid)-1] + `,"unknown":true}`,
		"missing field":            `{"operation_class":"catalog_resolve","policy_id":"verified_read_only","result_class":"allow","requested_lines":0,"returned_lines":0,"duration_ms":0}`,
		"invalid operation":        strings.Replace(valid, "catalog_resolve", "unknown", 1),
		"invalid policy":           strings.Replace(valid, "verified_read_only", "unknown", 1),
		"invalid result":           strings.Replace(valid, `"allow"`, `"unknown"`, 1),
		"invalid lifecycle":        strings.Replace(valid, "completed", "unknown", 1),
		"negative requested":       strings.Replace(valid, `"requested_lines":0`, `"requested_lines":-1`, 1),
		"negative returned":        strings.Replace(valid, `"returned_lines":0`, `"returned_lines":-1`, 1),
		"negative duration":        strings.Replace(valid, `"duration_ms":0`, `"duration_ms":-1`, 1),
		"non integer requested":    strings.Replace(valid, `"requested_lines":0`, `"requested_lines":1.5`, 1),
		"non integer returned":     strings.Replace(valid, `"returned_lines":0`, `"returned_lines":1.5`, 1),
		"non integer duration":     strings.Replace(valid, `"duration_ms":0`, `"duration_ms":1.5`, 1),
		"incomplete newline frame": valid,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "audit")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatalf("MkdirAll() error = %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, "audit-2026-09-02.jsonl"), []byte(record+"\n"), 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			if name == "incomplete newline frame" {
				if err := os.WriteFile(filepath.Join(dir, "audit-2026-09-02.jsonl"), []byte(record), 0o600); err != nil {
					t.Fatalf("WriteFile(incomplete) error = %v", err)
				}
			}
			_, err := OpenFile(FileConfig{Root: root, Components: []string{"audit"}, RetentionDays: "2", Platform: testPlatform{root: root}, Now: func() time.Time { return time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC) }})
			if !errors.Is(err, ErrAuditUnavailable) {
				t.Fatalf("OpenFile() error = %v, want ErrAuditUnavailable", err)
			}
		})
	}
}

func TestOpenFileScansWhileLifetimeLockExcludesCompetingOpener(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "audit")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	valid, err := encodeEvent(validAuditEvent())
	if err != nil {
		t.Fatalf("encodeEvent() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "audit-2026-08-30.jsonl"), valid, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	config := FileConfig{Root: root, Components: []string{"audit"}, RetentionDays: "2", Platform: testPlatform{root: root}, Now: func() time.Time { return time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC) }, DirectorySync: func(*os.File) error { close(entered); <-release; return nil }}
	opened := make(chan *File, 1)
	go func() { sink, _ := OpenFile(config); opened <- sink }()
	<-entered
	if _, err := OpenFile(config); !errors.Is(err, ErrAuditUnavailable) {
		t.Fatalf("competing OpenFile() error = %v, want ErrAuditUnavailable", err)
	}
	close(release)
	sink := <-opened
	if sink == nil {
		t.Fatal("primary OpenFile() returned nil after validated retention scan")
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

type testPlatform struct{ root string }

type swapAfterEvidencePlatform struct {
	testPlatform
	swap func()
}

func (p swapAfterEvidencePlatform) CreateManagedFile(path string, components ...string) (localstate.Evidence, error) {
	evidence, err := p.testPlatform.CreateManagedFile(path, components...)
	if err == nil && p.swap != nil {
		p.swap()
	}
	return evidence, err
}

func (p testPlatform) VerifyManagedDirectory(path string, components ...string) (localstate.Evidence, error) {
	if path != filepath.Join(append([]string{p.root}, components...)...) {
		return localstate.Evidence{}, localstate.ErrUnsafePath
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return localstate.Evidence{}, err
	}
	return localstate.Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}, nil
}

func (p testPlatform) CreateManagedFile(path string, components ...string) (localstate.Evidence, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return localstate.Evidence{}, err
	}
	if err := file.Close(); err != nil {
		return localstate.Evidence{}, err
	}
	return localstate.Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}, nil
}
