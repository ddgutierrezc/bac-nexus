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

func onboardingDeps(events *[]string) OnboardingDeps {
	return OnboardingDeps{
		Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
			*events = append(*events, "inspect")
			return remote.HostKeyObservation{Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", TrustCandidate: profile.HostKeyTrustTOFU}, nil
		},
		Proof: func(context.Context, profile.Profile, []byte) error { *events = append(*events, "proof"); return nil },
		Save:  func(profile.Profile) error { *events = append(*events, "save"); return nil },
		Audit: func(_ context.Context, event OnboardingAuditEvent) error {
			*events = append(*events, event.Code)
			return nil
		},
		Capability: func() credential.Capability { return credential.CapabilityUnsupported },
	}
}

func TestOnboardingOwnsSecretAndAuditsBeforePersistence(t *testing.T) {
	events := []string{}
	secret := []byte("password")
	service := NewOnboardingService(onboardingDeps(&events))
	id, code := service.StartCaptured(context.Background(), OnboardingRequest{Host: "ibmi.example.test", Username: "USER"}, secret)
	if code != OnboardingStarted || service.Wait(context.Background(), id.ID).Code != OnboardingSaved {
		t.Fatal("onboarding did not save")
	}
	for _, value := range secret {
		if value != 0 {
			t.Fatal("caller secret was not zeroed")
		}
	}
	want := []string{"inspect", "identity_bootstrap_allowed", "proof", "save", "identity_pin_committed"}
	for i := range want {
		if i >= len(events) || events[i] != want[i] {
			t.Fatalf("events = %v, want %v", events, want)
		}
	}
}

func TestOnboardingFailsClosedForMismatchedExistingPin(t *testing.T) {
	events := []string{}
	deps := onboardingDeps(&events)
	deps.Existing = func(context.Context, string) (*profile.Profile, error) {
		return &profile.Profile{Name: "user-ibmi-example-test", Host: "ibmi.example.test", Port: 22, Username: "USER", HostKeyFingerprint: "SHA256:CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC", HostKeyTrust: profile.HostKeyTrustTOFU}, nil
	}
	deps.Proof = func(context.Context, profile.Profile, []byte) error { t.Fatal("proof must not run"); return nil }
	service := NewOnboardingService(deps)
	id, _ := service.StartCaptured(context.Background(), OnboardingRequest{Host: "ibmi.example.test", Username: "USER"}, []byte("password"))
	if result := service.Wait(context.Background(), id.ID); result.Code != OnboardingFailed {
		t.Fatalf("result = %#v", result)
	}
	if len(events) != 2 || events[1] != "identity_changed" {
		t.Fatalf("events = %v", events)
	}
}

func TestOnboardingCancellationPreventsPersistence(t *testing.T) {
	started := make(chan struct{})
	saved := 0
	deps := onboardingDeps(&[]string{})
	deps.Inspect = func(ctx context.Context, _ string, _ int) (remote.HostKeyObservation, error) {
		close(started)
		<-ctx.Done()
		return remote.HostKeyObservation{}, ctx.Err()
	}
	deps.Save = func(profile.Profile) error { saved++; return nil }
	service := NewOnboardingService(deps)
	id, _ := service.StartCaptured(context.Background(), OnboardingRequest{Host: "ibmi.example.test", Username: "USER"}, []byte("password"))
	<-started
	service.Cancel(id.ID)
	if result := service.Wait(context.Background(), id.ID); result.Code != OnboardingCancelled || saved != 0 {
		t.Fatalf("result = %#v, saved = %d", result, saved)
	}
}

func TestOnboardingClassifiesIncompleteKeyringCompensation(t *testing.T) {
	events := []string{}
	deps := onboardingDeps(&events)
	deps.Capability = func() credential.Capability { return credential.CapabilitySupported }
	deps.Keyring = onboardingKeyringStub{}
	deps.Commit = func(context.Context, profile.Profile, []byte, func(context.Context) error) profile.OnboardingCommitResult {
		return profile.OnboardingCommitResult{CleanupRequired: true, Err: errors.New("persistence failed")}
	}
	service := NewOnboardingService(deps)
	id, _ := service.StartCaptured(context.Background(), OnboardingRequest{Host: "ibmi.example.test", Username: "USER"}, []byte("password"))
	result := service.Wait(context.Background(), id.ID)
	if result.Code != OnboardingFailed || !result.CleanupRequired || !result.CredentialRetained {
		t.Fatalf("result = %#v", result)
	}
}
