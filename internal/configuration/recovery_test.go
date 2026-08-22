package configuration

import (
	"context"
	"errors"
	"testing"

	"bac-nexus/internal/credential"
	"bac-nexus/internal/profile"
)

type recoveryProfiles struct {
	deleteConfirmation profile.DeleteConfirmation
	restored           string
	deleteResult       profile.ProfileDeleteResult
	deleteErr          error
}

func (r *recoveryProfiles) Save(profile.Profile) (string, error) { return "profile", nil }
func (r *recoveryProfiles) List(int) ([]profile.Profile, error)  { return nil, nil }
func (r *recoveryProfiles) Read(string) (profile.Profile, error) {
	return profile.Profile{}, profile.ErrProfileNotFound
}
func (r *recoveryProfiles) Update(profile.Profile, string) (profile.ProfileUpdateResult, error) {
	return profile.ProfileUpdateResult{}, nil
}
func (r *recoveryProfiles) Delete(_ string, confirmation profile.DeleteConfirmation) (profile.ProfileDeleteResult, error) {
	r.deleteConfirmation = confirmation
	if r.deleteErr != nil {
		return profile.ProfileDeleteResult{}, r.deleteErr
	}
	return r.deleteResult, nil
}
func (r *recoveryProfiles) Restore(name string) error {
	r.restored = name
	return nil
}

type recoveryVault struct {
	deleted string
	err     error
}

func (r *recoveryVault) Set(string, []byte, []byte, bool) (credential.SetResult, error) {
	return credential.SetResult{}, nil
}
func (r *recoveryVault) Delete(name string) (bool, error) {
	r.deleted = name
	return r.err == nil, r.err
}

func TestDeleteProfileSeparatesExactDecisionsAndRestoresOnCredentialFailure(t *testing.T) {
	profiles := &recoveryProfiles{deleteResult: profile.ProfileDeleteResult{
		Deleted: true, BackupPath: "retained", CredentialOutcome: profile.CredentialOutcomeUntouched,
	}}
	vault := &recoveryVault{err: errors.New("native store unavailable")}
	deps := newServiceDeps(t)
	deps.Profiles = profiles
	deps.Vaults = vault
	svc := NewService(deps)

	if _, err := svc.DeleteProfile(context.Background(), "dev", "yes", ""); err == nil {
		t.Fatal("accepted an inexact profile confirmation")
	}
	if _, err := svc.DeleteProfile(context.Background(), "dev", profile.DeleteConfirmation("delete dev"), "delete credential other"); err == nil {
		t.Fatal("accepted an inexact credential confirmation")
	}
	result, err := svc.DeleteProfile(context.Background(), "dev", profile.DeleteConfirmation("delete dev"), "")
	if err != nil || result.CredentialOutcome != profile.CredentialOutcomeUntouched || vault.deleted != "" {
		t.Fatalf("profile-only delete = %#v, %v; vault=%q", result, err, vault.deleted)
	}
	result, err = svc.DeleteProfile(context.Background(), "dev", profile.DeleteConfirmation("delete dev"), profile.CredentialDeleteConfirmation("delete credential dev"))
	if err == nil || result.CredentialOutcome != profile.CredentialOutcomeFailed || profiles.restored != "dev" || vault.deleted != "dev" {
		t.Fatalf("failed credential delete = %#v, %v; restored=%q vault=%q", result, err, profiles.restored, vault.deleted)
	}
	if err.Error() != "credential deletion failed; profile restored" {
		t.Fatalf("credential failure was not deterministic/sanitized: %v", err)
	}
}

func TestDeleteProfileCancellationDoesNotReachStores(t *testing.T) {
	profiles := &recoveryProfiles{}
	deps := newServiceDeps(t)
	deps.Profiles = profiles
	svc := NewService(deps)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.DeleteProfile(ctx, "dev", profile.DeleteConfirmation("delete dev"), ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled delete = %v", err)
	}
	if profiles.deleteConfirmation != "" {
		t.Fatal("cancelled delete reached profile store")
	}
}
