package profile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bac-nexus/internal/localstate"
)

type approvedEligibilityPlatform struct{}

func (approvedEligibilityPlatform) VerifyManagedDirectory(path string, _ ...string) (localstate.Evidence, error) {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return localstate.Evidence{}, err
	}
	return localstate.Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}, nil
}

func (approvedEligibilityPlatform) CreateManagedFile(string, ...string) (localstate.Evidence, error) {
	return localstate.Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}, nil
}

type rejectedEligibilityPlatform struct{ rejectDirectory, rejectFile bool }

func (p rejectedEligibilityPlatform) VerifyManagedDirectory(path string, _ ...string) (localstate.Evidence, error) {
	if p.rejectDirectory {
		return localstate.Evidence{}, localstate.ErrUnsafePath
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return localstate.Evidence{}, err
	}
	return localstate.Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}, nil
}

func (p rejectedEligibilityPlatform) CreateManagedFile(string, ...string) (localstate.Evidence, error) {
	if p.rejectFile {
		return localstate.Evidence{}, localstate.ErrUnsafePath
	}
	return localstate.Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}, nil
}

func newEligibilityStore(t *testing.T) EligibilityStore {
	t.Helper()
	configRoot := t.TempDir()
	return EligibilityStore{
		Root:          filepath.Join(configRoot, "BAC Nexus", "profiles"),
		UserConfigDir: func() (string, error) { return configRoot, nil },
		Platform:      approvedEligibilityPlatform{},
	}
}

func approvedEligibility() Eligibility {
	return Eligibility{
		SchemaVersion: EligibilitySchemaVersionV1,
		Profile:       "production",
		TargetDigest:  eligibilityDigest('a'),
		PolicyID:      "verified_read_only",
		PinDigest:     eligibilityDigest('b'),
		CredentialRef: "keyring:" + eligibilityDigest('c'),
		ArtifactRef:   eligibilityDigest('d'),
		ProofDigest:   eligibilityDigest('e'),
		ApprovedAt:    time.Unix(100, 0).UTC(),
		ExpiresAt:     time.Unix(200, 0).UTC(),
	}
}

func TestEligibilityStoreRoundTripBindsOnlyCanonicalReferences(t *testing.T) {
	store := newEligibilityStore(t)
	want := approvedEligibility()
	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := store.Load("production")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != want {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}

func TestEligibilityStoreClassifiesMissingStaleMismatchKeyringAndLegacyAsIneligible(t *testing.T) {
	now := time.Unix(150, 0).UTC()
	store := newEligibilityStore(t)
	want := approvedEligibility()
	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	cases := []struct {
		name       string
		profile    Profile
		binding    EligibilityBinding
		keyringOK  bool
		prepare    func(t *testing.T)
		wantReason EligibilityRejection
	}{
		{name: "approved", profile: eligibleProfile(), binding: want.Binding(), keyringOK: true, wantReason: EligibilityApproved},
		{name: "missing", profile: eligibleProfile(), binding: want.Binding(), keyringOK: true, prepare: func(t *testing.T) {
			if err := os.Remove(filepath.Join(store.Root, "production.eligibility.json")); err != nil {
				t.Fatal(err)
			}
		}, wantReason: EligibilityMissing},
		{name: "stale", profile: eligibleProfile(), binding: EligibilityBinding{TargetDigest: want.TargetDigest, PolicyID: want.PolicyID, PinDigest: want.PinDigest, CredentialRef: want.CredentialRef, ArtifactRef: want.ArtifactRef, ProofDigest: want.ProofDigest}, keyringOK: true, prepare: func(t *testing.T) {
			stale := want
			stale.ExpiresAt = now.Add(-time.Second)
			if err := store.Save(stale); err != nil {
				t.Fatal(err)
			}
		}, wantReason: EligibilityStale},
		{name: "mismatch", profile: eligibleProfile(), binding: EligibilityBinding{TargetDigest: eligibilityDigest('f'), PolicyID: want.PolicyID, PinDigest: want.PinDigest, CredentialRef: want.CredentialRef, ArtifactRef: want.ArtifactRef, ProofDigest: want.ProofDigest}, keyringOK: true, wantReason: EligibilityMismatch},
		{name: "keyring unavailable", profile: eligibleProfile(), binding: want.Binding(), keyringOK: false, wantReason: EligibilityKeyringUnavailable},
		{name: "legacy profile", profile: legacyProfile(), binding: want.Binding(), keyringOK: true, wantReason: EligibilityLegacyProfile},
		{name: "unknown profile version", profile: unknownProfile(), binding: want.Binding(), keyringOK: true, wantReason: EligibilityLegacyProfile},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if err := store.Save(want); err != nil {
				t.Fatalf("reset Save() error = %v", err)
			}
			if tt.prepare != nil {
				tt.prepare(t)
			}
			got := store.Check(tt.profile, tt.binding, tt.keyringOK, now)
			if got != tt.wantReason {
				t.Fatalf("Check() = %q, want %q", got, tt.wantReason)
			}
		})
	}
}

func TestDeriveEligibilityBindingIsDeterministicAndBindsEveryIdentityDimension(t *testing.T) {
	base := eligibleProfile()
	base.HostKeyTrust = HostKeyTrustVerified
	first, err := DeriveEligibilityBinding(base)
	if err != nil {
		t.Fatalf("DeriveEligibilityBinding() error = %v", err)
	}
	second, err := DeriveEligibilityBinding(base)
	if err != nil || first != second {
		t.Fatalf("DeriveEligibilityBinding() = %#v, %v; want deterministic result", second, err)
	}
	for _, mutate := range []func(*Profile){
		func(p *Profile) { p.Host = "other.example" },
		func(p *Profile) { p.Port++ },
		func(p *Profile) { p.Username = "other" },
		func(p *Profile) { p.HostKeyTrust = HostKeyTrustTOFU },
		func(p *Profile) { p.Name = "other" },
	} {
		candidate := base
		mutate(&candidate)
		got, err := DeriveEligibilityBinding(candidate)
		if err != nil || got == first {
			t.Fatalf("mutated binding = %#v, %v; want changed valid binding", got, err)
		}
	}
	approvedAt := time.Unix(100, 0).UTC()
	eligibility, err := NewEligibility(base, approvedAt)
	if err != nil || eligibility.ApprovedAt != approvedAt || eligibility.ExpiresAt != approvedAt.Add(EligibilityLifetime) || eligibility.Binding() != first {
		t.Fatalf("NewEligibility() = %#v, %v", eligibility, err)
	}
	for _, reference := range []string{first.TargetDigest, first.PinDigest, first.CredentialRef, first.ArtifactRef, first.ProofDigest} {
		if strings.Contains(reference, "secret") || strings.Contains(reference, base.Name) {
			t.Fatalf("reference leaks controlled identity: %q", reference)
		}
	}
	if eligibilityReference("vector", "a", "bc") == eligibilityReference("vector", "ab", "c") {
		t.Fatal("length-prefixed encoding collided across field boundaries")
	}
}

func TestEligibilityStoreRejectsUnsafeOrNonCanonicalRecordsAndPreservesPriorRecordOnReplaceFailure(t *testing.T) {
	store := newEligibilityStore(t)
	want := approvedEligibility()
	if err := store.Save(want); err != nil {
		t.Fatalf("initial Save() error = %v", err)
	}
	invalid := want
	invalid.TargetDigest = "ibmi.example.internal"
	if err := store.Save(invalid); !errors.Is(err, ErrEligibilityInvalid) {
		t.Fatalf("Save(invalid) error = %v, want ErrEligibilityInvalid", err)
	}
	if got, err := store.Load(want.Profile); err != nil || got != want {
		t.Fatalf("Load() after invalid Save = %#v, %v; want original", got, err)
	}
	broken := EligibilityStore{Root: store.Root, UserConfigDir: store.UserConfigDir, Platform: store.Platform, replace: func(string, string) error { return errors.New("replace failed") }}
	if err := broken.Save(want); !errors.Is(err, ErrEligibilityUnavailable) {
		t.Fatalf("Save(replace failure) error = %v, want ErrEligibilityUnavailable", err)
	}
	if got, err := store.Load(want.Profile); err != nil || got != want {
		t.Fatalf("Load() after replace failure = %#v, %v; want original", got, err)
	}
}

func TestEligibilityStoreRejectsUnavailableDirectoryAndFileEvidence(t *testing.T) {
	store := newEligibilityStore(t)
	want := approvedEligibility()

	store.Platform = rejectedEligibilityPlatform{rejectDirectory: true}
	if err := store.Save(want); !errors.Is(err, ErrEligibilityInvalid) {
		t.Fatalf("Save() with rejected directory evidence = %v, want ErrEligibilityInvalid", err)
	}

	store = newEligibilityStore(t)
	store.Platform = rejectedEligibilityPlatform{rejectFile: true}
	if err := store.Save(want); !errors.Is(err, ErrEligibilityUnavailable) {
		t.Fatalf("Save() with rejected file evidence = %v, want ErrEligibilityUnavailable", err)
	}
}

func eligibleProfile() Profile {
	return Profile{SchemaVersion: SchemaVersionV3, Name: "production", Host: "127.0.0.1", Port: 22, Username: "operator", HostKeyFingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", HostKeyTrust: HostKeyTrustTOFU, CredentialMode: CredentialModeKeyring}
}

func legacyProfile() Profile {
	p := eligibleProfile()
	p.SchemaVersion = 0
	p.CredentialMode = CredentialModeVault
	return p
}
func unknownProfile() Profile { p := eligibleProfile(); p.SchemaVersion = 99; return p }
func eligibilityDigest(character rune) string {
	return "sha256:" + strings.Repeat(string(character), 64)
}
