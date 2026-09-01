package configuration

import (
	"context"
	"errors"
	"testing"

	"bac-nexus/internal/credential"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
)

type onboardingKeyringStub struct{}

func (onboardingKeyringStub) Get(string) ([]byte, error) { return nil, nil }
func (onboardingKeyringStub) Set(string, []byte) error   { return nil }
func (onboardingKeyringStub) Delete(string) error        { return nil }

func TestOnboardingOwnsAndZeroesSecretAfterAuditedProofBeforeSave(t *testing.T) {
	var events []string
	secret := []byte("password")
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
			events = append(events, "inspect")
			return remote.HostKeyObservation{Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", TrustCandidate: profile.HostKeyTrustTOFU}, nil
		},
		Proof: func(_ context.Context, _ profile.Profile, password []byte) error {
			events = append(events, "proof")
			if string(password) != "password" {
				t.Fatal("password was not delivered to proof")
			}
			return nil
		},
		Save: func(p profile.Profile) error {
			events = append(events, "save")
			if p.CredentialMode != profile.CredentialModePrompt || p.HostKeyProvenance != automaticTOFUProvenance {
				t.Fatalf("saved profile = %#v", p)
			}
			return nil
		},
		Audit: func(_ context.Context, event OnboardingAuditEvent) error {
			events = append(events, string(event.Code))
			return nil
		},
		Capability: func() credential.Capability { return credential.CapabilityUnsupported },
	})
	id, code := service.StartCaptured(context.Background(), OnboardingRequest{Host: "ibmi.example.test", Username: "USER"}, secret)
	if code != OnboardingStarted || id.ID == "" {
		t.Fatalf("StartCaptured() = %#v, %q", id, code)
	}
	result := service.Wait(context.Background(), id.ID)
	if result.Code != OnboardingSaved || result.Profile.CredentialMode != profile.CredentialModePrompt {
		t.Fatalf("Wait() = %#v", result)
	}
	for i, b := range secret {
		if b != 0 {
			t.Fatalf("secret[%d] was not zeroed", i)
		}
	}
	want := []string{"inspect", "identity_bootstrap_allowed", "proof", "save", "identity_pin_committed"}
	if len(events) != len(want) {
		t.Fatalf("events = %v", events)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events = %v, want %v", events, want)
		}
	}
}

func TestOnboardingRejectsInvalidSecretBeforeSideEffects(t *testing.T) {
	calls := 0
	service := NewOnboardingService(OnboardingDeps{Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
		calls++
		return remote.HostKeyObservation{}, nil
	}})
	id, code := service.StartCaptured(context.Background(), OnboardingRequest{Host: "ibmi.example.test", Username: "USER"}, nil)
	if id.ID != "" || code != OnboardingRejected || calls != 0 {
		t.Fatalf("StartCaptured() = %#v, %q, calls=%d", id, code, calls)
	}
}

func TestOnboardingRejectsUnauditedAutomaticTOFUBeforeAnySideEffect(t *testing.T) {
	inspected := 0
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
			inspected++
			return remote.HostKeyObservation{}, nil
		},
		Proof: func(context.Context, profile.Profile, []byte) error { return nil },
		Save:  func(profile.Profile) error { return nil },
	})
	id, code := service.StartCaptured(context.Background(), OnboardingRequest{Host: "ibmi.example.test", Username: "USER"}, []byte("password"))
	if code != OnboardingRejected || id != (OperationIdentity{}) || inspected != 0 {
		t.Fatalf("unaudited start = %#v, %q, inspected=%d", id, code, inspected)
	}
}

func TestOnboardingCompensatesSavedProfileWhenCommittedAuditFails(t *testing.T) {
	var events []string
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
			return remote.HostKeyObservation{Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", TrustCandidate: profile.HostKeyTrustTOFU}, nil
		},
		Proof:  func(context.Context, profile.Profile, []byte) error { events = append(events, "proof"); return nil },
		Save:   func(profile.Profile) error { events = append(events, "save"); return nil },
		Delete: func(string) error { events = append(events, "delete"); return nil },
		Audit: func(_ context.Context, event OnboardingAuditEvent) error {
			events = append(events, event.Code)
			if event.Code == "identity_pin_committed" {
				return errors.New("audit unavailable")
			}
			return nil
		},
		Capability: func() credential.Capability { return credential.CapabilityUnsupported },
	})
	id, code := service.StartCaptured(context.Background(), OnboardingRequest{Host: "ibmi.example.test", Username: "USER"}, []byte("password"))
	if code != OnboardingStarted {
		t.Fatalf("start = %q", code)
	}
	result := service.Wait(context.Background(), id.ID)
	if result.Code != OnboardingFailed || result.CleanupRequired {
		t.Fatalf("committed audit result = %#v", result)
	}
	want := []string{"identity_bootstrap_allowed", "proof", "save", "identity_pin_committed", "delete"}
	if len(events) != len(want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events = %v, want %v", events, want)
		}
	}
}

func TestOnboardingRejectsExistingPinMismatchAndAuditsIdentityChange(t *testing.T) {
	var events []string
	secret := []byte("password")
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
			return remote.HostKeyObservation{Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", TrustCandidate: profile.HostKeyTrustTOFU}, nil
		},
		Existing: func(context.Context, string) (*profile.Profile, error) {
			p := profile.Profile{Name: "user-ibmi-example-test", Host: "ibmi.example.test", Port: 22, Username: "USER", HostKeyFingerprint: "SHA256:CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC", HostKeyTrust: profile.HostKeyTrustTOFU}
			return &p, nil
		},
		Audit: func(_ context.Context, event OnboardingAuditEvent) error {
			events = append(events, event.Code)
			return nil
		},
		Proof: func(context.Context, profile.Profile, []byte) error {
			t.Fatal("proof must not run for a mismatched pin")
			return nil
		},
		Save:       func(profile.Profile) error { t.Fatal("save must not run for a mismatched pin"); return nil },
		Capability: func() credential.Capability { return credential.CapabilityUnsupported },
	})
	id, code := service.StartCaptured(context.Background(), OnboardingRequest{Host: "ibmi.example.test", Username: "USER"}, secret)
	if code != OnboardingStarted || id == (OperationIdentity{}) {
		t.Fatalf("StartCaptured() = %#v, %q", id, code)
	}
	if result := service.Wait(context.Background(), id.ID); result.Code != OnboardingFailed {
		t.Fatalf("Wait() = %#v", result)
	}
	if len(events) != 1 || events[0] != "identity_changed" {
		t.Fatalf("events = %v, want [identity_changed]", events)
	}
	for index, value := range secret {
		if value != 0 {
			t.Fatalf("secret[%d] = %d, want zero", index, value)
		}
	}
}

func TestOnboardingAcceptsExactExistingPinWithoutBootstrapAudit(t *testing.T) {
	var events []string
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
			return remote.HostKeyObservation{Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", TrustCandidate: profile.HostKeyTrustTOFU}, nil
		},
		Existing: func(context.Context, string) (*profile.Profile, error) {
			p := profile.Profile{SchemaVersion: profile.SchemaVersionV3, Name: "user-ibmi-example-test", Host: "ibmi.example.test", Port: 22, Username: "USER", HostKeyFingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", HostKeyTrust: profile.HostKeyTrustTOFU, CredentialMode: profile.CredentialModePrompt}
			return &p, nil
		},
		Proof: func(context.Context, profile.Profile, []byte) error { events = append(events, "proof"); return nil },
		Save:  func(profile.Profile) error { events = append(events, "save"); return nil },
		Audit: func(_ context.Context, event OnboardingAuditEvent) error {
			events = append(events, event.Code)
			return nil
		},
		Capability: func() credential.Capability { return credential.CapabilityUnsupported },
	})
	id, code := service.StartCaptured(context.Background(), OnboardingRequest{Host: "ibmi.example.test", Username: "USER"}, []byte("password"))
	if code != OnboardingStarted {
		t.Fatalf("StartCaptured() code = %q", code)
	}
	if result := service.Wait(context.Background(), id.ID); result.Code != OnboardingSaved {
		t.Fatalf("Wait() = %#v", result)
	}
	want := []string{"proof", "save", "identity_pin_committed"}
	if len(events) != len(want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	for index := range want {
		if events[index] != want[index] {
			t.Fatalf("events = %v, want %v", events, want)
		}
	}
}

func TestOnboardingCancelReturnsCancelledWithoutPersistence(t *testing.T) {
	started := make(chan struct{})
	saved := 0
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(ctx context.Context, _ string, _ int) (remote.HostKeyObservation, error) {
			close(started)
			<-ctx.Done()
			return remote.HostKeyObservation{}, ctx.Err()
		},
		Proof: func(context.Context, profile.Profile, []byte) error { return nil },
		Save:  func(profile.Profile) error { saved++; return nil },
		Audit: func(context.Context, OnboardingAuditEvent) error { return nil },
	})
	id, code := service.StartCaptured(context.Background(), OnboardingRequest{Host: "ibmi.example.test", Username: "USER"}, []byte("password"))
	if code != OnboardingStarted {
		t.Fatalf("StartCaptured() = %q", code)
	}
	<-started
	service.Cancel(id.ID)
	if result := service.Wait(context.Background(), id.ID); result.Code != OnboardingCancelled {
		t.Fatalf("Wait() = %#v", result)
	}
	if saved != 0 {
		t.Fatalf("Save() called %d times after cancellation", saved)
	}
}

func TestOnboardingDelegatesPersistenceToPreparedCommitAfterProof(t *testing.T) {
	events := []string{}
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
			return remote.HostKeyObservation{Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", TrustCandidate: profile.HostKeyTrustTOFU}, nil
		},
		Proof: func(context.Context, profile.Profile, []byte) error { events = append(events, "proof"); return nil },
		Save: func(profile.Profile) error {
			t.Fatal("legacy save bypassed prepared commit")
			return nil
		},
		Audit: func(_ context.Context, event OnboardingAuditEvent) error {
			events = append(events, event.Code)
			return nil
		},
		Capability: func() credential.Capability { return credential.CapabilityUnsupported },
		Commit: func(ctx context.Context, p profile.Profile, _ []byte, committed func(context.Context) error) profile.OnboardingCommitResult {
			events = append(events, "prepared-commit")
			if err := committed(ctx); err != nil {
				return profile.OnboardingCommitResult{Err: err}
			}
			return profile.OnboardingCommitResult{Saved: true}
		},
	})
	id, code := service.StartCaptured(context.Background(), OnboardingRequest{Host: "ibmi.example.test", Username: "USER"}, []byte("password"))
	if code != OnboardingStarted || service.Wait(context.Background(), id.ID).Code != OnboardingSaved {
		t.Fatal("prepared transaction did not save")
	}
	want := []string{"identity_bootstrap_allowed", "proof", "prepared-commit", "identity_pin_committed"}
	for i := range want {
		if i >= len(events) || events[i] != want[i] {
			t.Fatalf("events = %v, want %v", events, want)
		}
	}
}

func TestOnboardingSaveFailureClassifiesRetainedCredentialAndRequiresCleanup(t *testing.T) {
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
			return remote.HostKeyObservation{Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", TrustCandidate: profile.HostKeyTrustTOFU}, nil
		},
		Proof:      func(context.Context, profile.Profile, []byte) error { return nil },
		Save:       func(profile.Profile) error { t.Fatal("legacy save bypassed prepared commit"); return nil },
		Audit:      func(context.Context, OnboardingAuditEvent) error { return nil },
		Capability: func() credential.Capability { return credential.CapabilitySupported },
		Keyring:    onboardingKeyringStub{},
		Commit: func(context.Context, profile.Profile, []byte, func(context.Context) error) profile.OnboardingCommitResult {
			return profile.OnboardingCommitResult{CleanupRequired: true, Err: errors.New("profile persistence failed")}
		},
	})
	id, code := service.StartCaptured(context.Background(), OnboardingRequest{Host: "ibmi.example.test", Username: "USER"}, []byte("password"))
	if code != OnboardingStarted {
		t.Fatalf("StartCaptured() = %q", code)
	}
	result := service.Wait(context.Background(), id.ID)
	if result.Code != OnboardingFailed || !result.CleanupRequired || !result.CredentialRetained {
		t.Fatalf("save failure result = %#v; want not saved, retained credential, cleanup guidance classification", result)
	}
}
