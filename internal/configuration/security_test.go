package configuration

import (
	"context"
	"errors"
	"strings"
	"testing"

	"bac-nexus/internal/credential"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
)

type lifecycleCredentialStore struct {
	presence credential.Presence
	set      int
	deleted  int
	migrated int
}

func (s *lifecycleCredentialStore) Get(string) ([]byte, error) { return []byte{1}, nil }
func (s *lifecycleCredentialStore) Status(string) (credential.Presence, error) {
	return s.presence, nil
}
func (s *lifecycleCredentialStore) Set(string, []byte) error {
	s.set++
	s.presence = credential.PresencePresent
	return nil
}
func (s *lifecycleCredentialStore) Delete(string) error {
	s.deleted++
	s.presence = credential.PresenceAbsent
	return nil
}
func (s *lifecycleCredentialStore) Migrate(string, credential.LegacyVault) error {
	s.migrated++
	return nil
}

type lifecycleSecretInput struct{ calls int }

func (s *lifecycleSecretInput) WithSecret(_ context.Context, _ string, use func([]byte) error) error {
	s.calls++
	return use([]byte{1, 2, 3})
}

func TestCredentialServiceReturnsOpaqueLifecycleOutcomes(t *testing.T) {
	store := &lifecycleCredentialStore{presence: credential.PresenceAbsent}
	input := &lifecycleSecretInput{}
	service := NewCredentialService(store, input)

	status, err := service.Status(context.Background(), "dev")
	if err != nil || status != credential.PresenceAbsent {
		t.Fatalf("Status() = %q, %v", status, err)
	}
	result, err := service.Set(context.Background(), "dev")
	if err != nil || result != CredentialOutcomeStored || store.set != 1 || input.calls != 1 {
		t.Fatalf("Set() = %q, %v; calls=%d, sets=%d", result, err, input.calls, store.set)
	}
	if strings.Contains(string(result), "123") {
		t.Fatal("credential outcome contains transient credential material")
	}
	result, err = service.Rotate(context.Background(), "dev")
	if err != nil || result != CredentialOutcomeRotated || store.set != 2 {
		t.Fatalf("Rotate() = %q, %v; sets=%d", result, err, store.set)
	}
	result, err = service.Delete(context.Background(), "dev", "delete credential dev")
	if err != nil || result != CredentialOutcomeDeleted || store.deleted != 1 {
		t.Fatalf("Delete() = %q, %v; deletes=%d", result, err, store.deleted)
	}
}

func TestCredentialServiceFailsClosedForInvalidSecretInputAndUnavailableStore(t *testing.T) {
	service := NewCredentialService(&lifecycleCredentialStore{presence: credential.PresenceUnavailable}, nil)
	if _, err := service.Set(context.Background(), "dev"); !errors.Is(err, ErrCredentialUnavailable) {
		t.Fatalf("Set() error = %v, want unavailable", err)
	}
	if _, err := service.Delete(context.Background(), "dev", "delete credential dev"); !errors.Is(err, ErrCredentialUnavailable) {
		t.Fatalf("Delete() error = %v, want unavailable", err)
	}
}

type lifecycleVault struct{ deleted bool }

func (v *lifecycleVault) Get() ([]byte, error) { return []byte{1, 2, 3}, nil }
func (v *lifecycleVault) Delete() error        { v.deleted = true; return nil }

func TestCredentialServiceMigrationUsesExplicitConfirmation(t *testing.T) {
	store := &lifecycleCredentialStore{presence: credential.PresenceAbsent}
	vault := &lifecycleVault{}
	service := NewCredentialService(store, nil)

	if _, err := service.Migrate(context.Background(), "dev", vault, false); !errors.Is(err, ErrConfirmationRequired) {
		t.Fatalf("Migrate() error = %v, want confirmation required", err)
	}
	result, err := service.Migrate(context.Background(), "dev", vault, true)
	if err != nil || result != CredentialOutcomeMigrated || store.migrated != 1 || vault.deleted {
		t.Fatalf("Migrate() = %q, %v; migrated=%d deleted=%t", result, err, store.migrated, vault.deleted)
	}
}

type lifecycleProfileStore struct{ value profile.Profile }

func (s *lifecycleProfileStore) Read(string) (profile.Profile, error) { return s.value, nil }
func (s *lifecycleProfileStore) Update(value profile.Profile, _ string) (profile.ProfileUpdateResult, error) {
	s.value = value
	return profile.ProfileUpdateResult{ReplacementCommitted: true}, nil
}

func TestTrustServiceInspectsOnlyAfterWarningAndPersistsConfirmedTOFU(t *testing.T) {
	fingerprint := "SHA256:" + strings.Repeat("A", 43)
	profiles := &lifecycleProfileStore{value: testSecurityProfile()}
	inspect := func(context.Context, string, int) (remote.HostKeyObservation, error) {
		return remote.HostKeyObservation{Algorithm: "ssh-ed25519", Fingerprint: fingerprint, TrustCandidate: profile.HostKeyTrustTOFU}, nil
	}
	service := NewTrustService(profiles, inspect)

	if _, err := service.InspectAndEnroll(context.Background(), "dev", false, "enroll "+fingerprint); !errors.Is(err, ErrWarningRequired) {
		t.Fatalf("InspectAndEnroll() error = %v, want warning", err)
	}
	result, err := service.InspectAndEnroll(context.Background(), "dev", true, "enroll "+fingerprint)
	if err != nil || result != TrustOutcomeEnrolled || profiles.value.HostKeyFingerprint != fingerprint || profiles.value.HostKeyTrust != profile.HostKeyTrustTOFU {
		t.Fatalf("InspectAndEnroll() = %q, %v; profile=%+v", result, err, profiles.value)
	}
}

func TestTrustServiceManualEnrollmentAndMismatchFailClosed(t *testing.T) {
	fingerprint := "SHA256:" + strings.Repeat("A", 43)
	profiles := &lifecycleProfileStore{value: testSecurityProfile()}
	service := NewTrustService(profiles, nil)
	if _, err := service.EnrollManual(context.Background(), "dev", fingerprint, "verified by operator", "enroll "+fingerprint); err != nil {
		t.Fatalf("EnrollManual() error = %v", err)
	}
	if err := service.Verify(context.Background(), "dev", "SHA256:"+strings.Repeat("B", 43)); !errors.Is(err, remote.ErrHostKeyChanged) {
		t.Fatalf("Verify() error = %v, want host key changed", err)
	}
}

func testSecurityProfile() profile.Profile {
	return profile.Profile{Name: "dev", Host: "example.com", Port: 22, Username: "operator", HostKeyFingerprint: "SHA256:" + strings.Repeat("C", 43), HostKeyTrust: profile.HostKeyTrustVerified, CredentialMode: profile.CredentialModePrompt}
}
