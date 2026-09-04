package onboardingdiagnostics

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/localstate"
)

type platformStub struct{}

func (platformStub) VerifyManagedDirectory(path string, _ ...string) (localstate.Evidence, error) {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return localstate.Evidence{}, err
	}
	return localstate.Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}, nil
}

func (platformStub) CreateManagedFile(path string, _ ...string) (localstate.Evidence, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return localstate.Evidence{}, err
	}
	if err := file.Close(); err != nil {
		return localstate.Evidence{}, err
	}
	return localstate.Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}, nil
}

func (platformStub) OpenManagedFile(path string, flags int, _ ...string) (*os.File, error) {
	return os.OpenFile(path, flags, 0)
}

func (platformStub) RemoveManagedFile(path string, _ ...string) error { return os.Remove(path) }

func diagnosticStore(t *testing.T, now time.Time) (*Store, string) {
	t.Helper()
	root := t.TempDir()
	return New(Config{UserConfigDir: func() (string, error) { return root, nil }, Platform: platformStub{}, Now: func() time.Time { return now }, Random: strings.NewReader(strings.Repeat("a", 64))}), root
}

func TestStoreWritesStrictSecretFreeRecord(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	store, root := diagnosticStore(t, now)
	reference, err := store.Record(context.Background(), configuration.OnboardingPhaseAuthenticatedProof, configuration.OnboardingFailureClass(configuration.ResultAuthenticationFailed), true, false)
	if err != nil || !validReference(reference) {
		t.Fatalf("Record() = %q, %v", reference, err)
	}
	data, err := os.ReadFile(filepath.Join(root, "BAC Nexus", "onboarding-diagnostics", reference+".json"))
	if err != nil || strings.Contains(string(data), "secret") || strings.Contains(string(data), "error") {
		t.Fatalf("record=%s err=%v", data, err)
	}
	if _, _, err := decodeRecord(data, reference); err != nil {
		t.Fatalf("strict record validation: %v", err)
	}
}

func TestStorePersistsFirstUseDiagnosticsThroughNativeManagedPath(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "BAC Nexus", "onboarding-diagnostics")
	if _, err := os.Lstat(directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("managed diagnostics directory exists before first use: %v", err)
	}
	store := New(Config{
		UserConfigDir: func() (string, error) { return root, nil },
		Platform:      localstate.NewPlatform(func() (string, error) { return root, nil }),
		Now:           func() time.Time { return now },
		Random:        strings.NewReader(strings.Repeat("a", 64)),
	})
	reference, err := store.Record(context.Background(), configuration.OnboardingPhaseHostKeyInspection, configuration.OnboardingClassHostKeyTimeout, false, false)
	if err != nil || !validReference(reference) {
		t.Fatalf("Record() = %q, %v", reference, err)
	}
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("managed diagnostics directory = %v, %v", info, err)
	}
	if _, _, err := decodeRecord(mustReadFile(t, filepath.Join(directory, reference+recordFileExt)), reference); err != nil {
		t.Fatalf("strict first-use record validation: %v", err)
	}
}

type windowsModePlatform struct{ approve bool }

func (p windowsModePlatform) VerifyManagedDirectory(path string, _ ...string) (localstate.Evidence, error) {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return localstate.Evidence{}, err
	}
	return localstate.Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}, nil
}

func (windowsModePlatform) CreateManagedFile(path string, _ ...string) (localstate.Evidence, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return localstate.Evidence{}, err
	}
	if err := file.Close(); err != nil || os.Chmod(path, 0o666) != nil {
		return localstate.Evidence{}, localstate.ErrUnsafePath
	}
	return localstate.Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}, nil
}

func (p windowsModePlatform) VerifyManagedFile(path string, _ ...string) (localstate.Evidence, error) {
	if !p.approve {
		return localstate.Evidence{}, localstate.ErrUnsafePath
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return localstate.Evidence{}, localstate.ErrUnsafePath
	}
	return localstate.Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}, nil
}

func (p windowsModePlatform) OpenManagedFile(path string, flags int, _ ...string) (*os.File, error) {
	if _, err := p.VerifyManagedFile(path); err != nil {
		return nil, err
	}
	return os.OpenFile(path, flags, 0)
}

func (windowsModePlatform) RemoveManagedFile(path string, _ ...string) error { return os.Remove(path) }

func TestStoreAcceptsWindowsModeOnlyAfterLocalstateApproval(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name    string
		approve bool
		wantOK  bool
	}{
		{"approved normal writable file", true, true},
		{"rejected unsafe ACL or reparse evidence", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			store := New(Config{UserConfigDir: func() (string, error) { return root, nil }, Platform: windowsModePlatform{approve: test.approve}, Now: func() time.Time { return now }, Random: strings.NewReader(strings.Repeat("a", 64))})
			reference, err := store.Record(context.Background(), configuration.OnboardingPhaseHostKeyInspection, configuration.OnboardingClassHostKeyTimeout, false, false)
			if test.wantOK {
				if err != nil || !validReference(reference) {
					t.Fatalf("Record() = %q, %v", reference, err)
				}
				return
			}
			if err == nil || reference != "" {
				t.Fatalf("Record() = %q, %v; want rejected unsafe evidence", reference, err)
			}
		})
	}
}

func TestStoreRecordsInExistingManagedDiagnosticsDirectory(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	store, root := diagnosticStore(t, now)
	directory := filepath.Join(root, "BAC Nexus", "onboarding-diagnostics")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if reference, err := store.Record(context.Background(), configuration.OnboardingPhaseHostKeyInspection, configuration.OnboardingClassHostKeyNoKey, false, false); err != nil || !validReference(reference) {
		t.Fatalf("Record() = %q, %v", reference, err)
	}
}

func TestStoreRejectsManagedDirectoryLinkAndCancellationWithoutWriting(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "BAC Nexus")); err != nil {
		t.Fatal(err)
	}
	store := New(Config{UserConfigDir: func() (string, error) { return root, nil }, Platform: localstate.NewPlatform(func() (string, error) { return root, nil }), Now: func() time.Time { return now }})
	if reference, err := store.Record(context.Background(), configuration.OnboardingPhaseHostKeyInspection, configuration.OnboardingClassHostKeyUnavailable, false, false); err == nil || reference != "" {
		t.Fatalf("Record() with managed link = %q, %v", reference, err)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cleanRoot := t.TempDir()
	cancelledStore := New(Config{UserConfigDir: func() (string, error) { cancel(); return cleanRoot, nil }, Platform: platformStub{}, Now: func() time.Time { return now }})
	if reference, err := cancelledStore.Record(cancelled, configuration.OnboardingPhaseHostKeyInspection, configuration.OnboardingClassHostKeyUnavailable, false, false); err == nil || reference != "" {
		t.Fatalf("Record() after cancellation = %q, %v", reference, err)
	}
	if _, err := os.Lstat(filepath.Join(cleanRoot, "BAC Nexus")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancellation created diagnostics state: %v", err)
	}
}

func TestStorePersistsOnlyAllowlistedHostKeyClasses(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	for _, class := range []configuration.OnboardingFailureClass{
		configuration.OnboardingClassHostKeyTimeout,
		configuration.OnboardingClassHostKeyNegotiation,
		configuration.OnboardingClassHostKeyNoKey,
		configuration.OnboardingClassHostKeyUnavailable,
		configuration.OnboardingClassHostKeyInvalidCandidate,
	} {
		t.Run(string(class), func(t *testing.T) {
			store, root := diagnosticStore(t, now)
			reference, err := store.Record(context.Background(), configuration.OnboardingPhaseHostKeyInspection, class, false, false)
			if err != nil {
				t.Fatal(err)
			}
			data := mustReadFile(t, filepath.Join(root, "BAC Nexus", "onboarding-diagnostics", reference+recordFileExt))
			if !strings.Contains(string(data), `"class":"`+string(class)+`"`) || strings.Contains(string(data), "host.example.test") || strings.Contains(string(data), "raw-error-canary") {
				t.Fatalf("unsafe or incomplete record: %s", data)
			}
			if _, _, err := decodeRecord(data, reference); err != nil {
				t.Fatalf("strict record validation: %v", err)
			}
		})
	}
	store, _ := diagnosticStore(t, now)
	if reference, err := store.Record(context.Background(), configuration.OnboardingPhaseHostKeyInspection, "raw-error-canary", false, false); err == nil || reference != "" {
		t.Fatalf("Record() with unknown class = %q, %v", reference, err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestStoreRejectsMalformedAndLinkInputsWithoutWriting(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name  string
		setup func(string) error
	}{
		{"malformed", func(directory string) error {
			return os.WriteFile(filepath.Join(directory, "ONB-0123456789abcdef0123456789abcdef.json"), []byte("{}"), 0o600)
		}},
		{"unknown", func(directory string) error {
			return os.WriteFile(filepath.Join(directory, "unknown.json"), []byte("{}"), 0o600)
		}},
		{"link", func(directory string) error {
			return os.Symlink(os.DevNull, filepath.Join(directory, "ONB-0123456789abcdef0123456789abcdef.json"))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, root := diagnosticStore(t, now)
			directory := filepath.Join(root, "BAC Nexus", "onboarding-diagnostics")
			if err := os.MkdirAll(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := test.setup(directory); err != nil {
				t.Fatal(err)
			}
			if reference, err := store.Record(context.Background(), configuration.OnboardingPhaseHostKeyInspection, configuration.OnboardingClassHostKeyFailure, false, false); err == nil || reference != "" {
				t.Fatalf("Record() = %q, %v", reference, err)
			}
		})
	}
}

func TestStorePrunesExpiredAndBoundsCanonicalRecords(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	store, root := diagnosticStore(t, now)
	directory := filepath.Join(root, "BAC Nexus", "onboarding-diagnostics")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 20; index++ {
		reference := fmt.Sprintf("ONB-%032x", index)
		timestamp := now.Add(-time.Duration(20-index) * time.Hour)
		data := []byte(`{"schema":"onboarding_failure_diagnostic/v1","reference":"` + reference + `","timestamp_utc":"` + timestamp.Format(time.RFC3339Nano) + `","phase":"host_key_inspection","class":"host_key_failure","cleanup_required":false,"credential_retained":false}`)
		if err := os.WriteFile(filepath.Join(directory, reference+".json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Record(context.Background(), configuration.OnboardingPhaseHostKeyInspection, configuration.OnboardingClassHostKeyFailure, false, false); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != maxRecords {
		t.Fatalf("entries=%d err=%v", len(entries), err)
	}
}

func TestStoreRemovesExpiredCanonicalRecords(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	store, root := diagnosticStore(t, now)
	directory := filepath.Join(root, "BAC Nexus", "onboarding-diagnostics")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	reference := "ONB-0123456789abcdef0123456789abcdef"
	expired := []byte(`{"schema":"onboarding_failure_diagnostic/v1","reference":"` + reference + `","timestamp_utc":"` + now.Add(-maxAge-time.Nanosecond).Format(time.RFC3339Nano) + `","phase":"host_key_inspection","class":"host_key_failure","cleanup_required":false,"credential_retained":false}`)
	path := filepath.Join(directory, reference+recordFileExt)
	if err := os.WriteFile(path, expired, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Record(context.Background(), configuration.OnboardingPhaseHostKeyInspection, configuration.OnboardingClassHostKeyFailure, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired record was not removed: %v", err)
	}
}

func TestStoreWriteAndSyncFailuresReturnNoReference(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name   string
		mutate func(*Config)
	}{
		{"write", func(config *Config) {
			config.Write = func(*os.File, []byte) (int, error) { return 0, errors.New("write failure") }
		}},
		{"sync", func(config *Config) { config.Sync = func(*os.File) error { return errors.New("sync failure") } }},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			config := Config{UserConfigDir: func() (string, error) { return root, nil }, Platform: platformStub{}, Now: func() time.Time { return now }, Random: strings.NewReader(strings.Repeat("a", 64))}
			test.mutate(&config)
			if reference, err := New(config).Record(context.Background(), configuration.OnboardingPhaseHostKeyInspection, configuration.OnboardingClassHostKeyFailure, false, false); err == nil || reference != "" {
				t.Fatalf("Record()=%q,%v", reference, err)
			}
		})
	}
}
