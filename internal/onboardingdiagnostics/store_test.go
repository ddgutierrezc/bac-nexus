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
