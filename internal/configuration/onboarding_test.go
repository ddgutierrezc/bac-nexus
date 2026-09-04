package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"bac-nexus/internal/connectors/ibmi/mapepirestdio"
	"bac-nexus/internal/credential"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
)

func TestOnboardingCaptureLeaseIsBoundedSingleUseAndExpires(t *testing.T) {
	now := time.Unix(100, 0)
	input, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()

	first, second := []byte("first-secret"), []byte("second-secret")
	read := first
	service := NewOnboardingService(OnboardingDeps{Now: func() time.Time { return now }})
	prompt := remote.SecretPrompt{
		Input: input, Output: output, IsTerminal: func(int) bool { return true },
		Read: func(int) ([]byte, error) { return read, nil },
	}
	request := OnboardingRequest{Name: "test-profile", Host: "ibmi.example.test", Port: 2222, Username: "USER"}

	firstLease, code := service.Capture(context.Background(), request, prompt, input, output, "password: ")
	if code != remote.PromptCaptured || firstLease.ID == "" {
		t.Fatalf("first Capture() = %#v, %q", firstLease, code)
	}
	for i, value := range first {
		if value != 0 {
			t.Fatalf("first capture input[%d] = %d, want zero", i, value)
		}
	}

	read = second
	secondLease, code := service.Capture(context.Background(), request, prompt, input, output, "password: ")
	if code != remote.PromptCaptured || secondLease.ID == firstLease.ID {
		t.Fatalf("replacement Capture() = %#v, %q", secondLease, code)
	}
	if code := service.StartCaptured(context.Background(), request, firstLease); code != OnboardingRejected {
		t.Fatalf("replaced StartCaptured() = %q, want rejected", code)
	}
	for i, value := range second {
		if value != 0 {
			t.Fatalf("replacement capture input[%d] = %d, want zero", i, value)
		}
	}

	now = now.Add(2*time.Minute + time.Nanosecond)
	if code := service.StartCaptured(context.Background(), request, secondLease); code != OnboardingRejected {
		t.Fatalf("expired StartCaptured() = %q, want rejected", code)
	}
}

func TestOnboardingCaptureAcceptsOnlyOneTo1024SecretBytes(t *testing.T) {
	for _, test := range []struct {
		name     string
		secret   []byte
		captured bool
	}{
		{name: "empty", secret: nil},
		{name: "one byte", secret: []byte{'x'}, captured: true},
		{name: "maximum bytes", secret: make([]byte, 1024), captured: true},
		{name: "over maximum bytes", secret: make([]byte, 1025)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if len(test.secret) > 0 {
				test.secret[0] = 'x'
			}
			service := NewOnboardingService(OnboardingDeps{})
			lease, code := captureForTest(t, service, test.secret)
			if test.captured {
				if code != remote.PromptCaptured || lease.ID == "" {
					t.Fatalf("Capture() = %#v, %q; want opaque captured lease", lease, code)
				}
				service.Revoke(lease)
			} else if code == remote.PromptCaptured || lease != (OperationIdentity{}) {
				t.Fatalf("Capture() = %#v, %q; want secret-free rejection", lease, code)
			}
			for i, value := range test.secret {
				if value != 0 {
					t.Fatalf("secret[%d] = %d, want zero", i, value)
				}
			}
		})
	}
}

func TestOnboardingUsesSelectedPortAndAuditsAfterProofBeforeCommit(t *testing.T) {
	var events []string
	store := profile.Store{Root: t.TempDir()}
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(_ context.Context, host string, port int) (remote.HostKeyObservation, error) {
			if host != "ibmi.example.test" || port != 2222 {
				t.Fatalf("Inspect(%q, %d), want selected endpoint", host, port)
			}
			events = append(events, "inspect")
			return remote.HostKeyObservation{Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}, nil
		},
		Proof: func(_ context.Context, p profile.Profile, _ []byte) Step8Result {
			if p.Name != "selected-port" || p.Port != 2222 {
				t.Fatalf("proof profile = %#v", p)
			}
			events = append(events, "proof")
			return successfulProof()
		},
		Save: func(profile.Profile) error { t.Fatal("legacy Save must not run"); return nil },
		Audit: func(_ context.Context, event OnboardingAuditEvent) error {
			if event.Code != "identity_bootstrap_allowed" {
				t.Fatalf("unexpected audit = %#v", event)
			}
			events = append(events, "audit")
			return nil
		},
		Commit: func(_ context.Context, p profile.Profile, _ []byte, _ func(context.Context) error) profile.OnboardingCommitResult {
			events = append(events, "commit")
			if _, err := store.Save(p); err != nil {
				t.Fatalf("Save selected-port profile: %v", err)
			}
			return profile.OnboardingCommitResult{Saved: true}
		},
	})
	request := OnboardingRequest{Name: "selected-port", Host: "ibmi.example.test", Port: 2222, Username: "USER"}
	lease, code := captureRequestForTest(t, service, request, []byte("password"))
	if code != remote.PromptCaptured || service.StartCaptured(context.Background(), request, lease) != OnboardingStarted {
		t.Fatalf("Capture/StartCaptured() = %#v, %q", lease, code)
	}
	if result := service.Wait(context.Background(), lease.ID); result.Code != OnboardingSaved || result.Profile.Port != 2222 {
		t.Fatalf("result = %#v", result)
	}
	if persisted, err := store.Load(request.Name); err != nil || persisted.Port != request.Port {
		t.Fatalf("persisted JSON profile = %#v, %v; want port %d", persisted, err, request.Port)
	}
	want := []string{"inspect", "proof", "audit", "commit"}
	if len(events) != len(want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events = %v, want %v", events, want)
		}
	}
}

func TestOnboardingDirectProfileBindsOnlyVerifiedReadOnlyFallbackPolicy(t *testing.T) {
	var proofProfile, savedProfile profile.Profile
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
			return remote.HostKeyObservation{Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", TrustCandidate: profile.HostKeyTrustTOFU}, nil
		},
		Proof: func(_ context.Context, p profile.Profile, _ []byte) Step8Result {
			proofProfile = p
			return successfulProof()
		},
		Save: func(p profile.Profile) error {
			savedProfile = p
			return nil
		},
		Audit: func(context.Context, OnboardingAuditEvent) error { return nil },
	})

	id, code := captureForTest(t, service, []byte("password"))
	if code != remote.PromptCaptured || service.StartCaptured(context.Background(), onboardingRequest(), id) != OnboardingStarted {
		t.Fatalf("Capture/StartCaptured() = %q", code)
	}
	if result := service.Wait(context.Background(), id.ID); result.Code != OnboardingSaved {
		t.Fatalf("Wait() = %#v", result)
	}
	for _, profileAtBoundary := range []struct {
		name string
		p    profile.Profile
	}{
		{name: "proof", p: proofProfile},
		{name: "saved", p: savedProfile},
	} {
		if !profileAtBoundary.p.FallbackAllowed || profileAtBoundary.p.EndpointPolicyRef != VerifiedReadOnlyEndpointPolicyRef {
			t.Fatalf("%s profile policy = fallback:%t ref:%q, want only approved verified read-only fallback", profileAtBoundary.name, profileAtBoundary.p.FallbackAllowed, profileAtBoundary.p.EndpointPolicyRef)
		}
		wantTrust := profile.TrustEvidence{Mode: profile.TrustModeTOFU, Pin: profileAtBoundary.p.HostKeyFingerprint, Provenance: profileAtBoundary.p.HostKeyProvenance}
		if profileAtBoundary.p.HostKeyTrust != profile.HostKeyTrustTOFU || profileAtBoundary.p.SSHTrust != wantTrust {
			t.Fatalf("%s profile trust = legacy:%q/%q/%q ssh:%#v, want matching canonical TOFU evidence", profileAtBoundary.name, profileAtBoundary.p.HostKeyFingerprint, profileAtBoundary.p.HostKeyTrust, profileAtBoundary.p.HostKeyProvenance, profileAtBoundary.p.SSHTrust)
		}
	}
}

func TestOnboardingBoundsOnlyHostKeyInspection(t *testing.T) {
	var inspectionDeadline time.Time
	inspectionHasDeadline := false
	proofHasDeadline := false
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(ctx context.Context, _ string, _ int) (remote.HostKeyObservation, error) {
			inspectionDeadline, inspectionHasDeadline = ctx.Deadline()
			return remote.HostKeyObservation{Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", TrustCandidate: profile.HostKeyTrustTOFU}, nil
		},
		Proof: func(ctx context.Context, _ profile.Profile, _ []byte) Step8Result {
			_, proofHasDeadline = ctx.Deadline()
			return successfulProof()
		},
		Save:  func(profile.Profile) error { return nil },
		Audit: func(context.Context, OnboardingAuditEvent) error { return nil },
	})
	id, code := captureForTest(t, service, []byte("password"))
	if code != remote.PromptCaptured || service.StartCaptured(context.Background(), onboardingRequest(), id) != OnboardingStarted {
		t.Fatalf("Capture/StartCaptured() = %q", code)
	}
	if result := service.Wait(context.Background(), id.ID); result.Code != OnboardingSaved {
		t.Fatalf("Wait() = %#v", result)
	}
	remaining := time.Until(inspectionDeadline)
	if !inspectionHasDeadline || remaining <= 0 || remaining > hostKeyInspectionTimeout {
		t.Fatalf("inspection deadline = %v, want bounded by %v", inspectionDeadline, hostKeyInspectionTimeout)
	}
	if proofHasDeadline {
		t.Fatal("proof inherited the host-key inspection deadline")
	}
}

func TestOnboardingHostKeyDeadlineExpiryIsClassifiedAsTimeout(t *testing.T) {
	recorder := &onboardingDiagnosticRecorderStub{}
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
			return remote.HostKeyObservation{}, context.DeadlineExceeded
		},
		Diagnostics: recorder,
	})
	result := service.run(context.Background(), onboardingRequest(), []byte("secret"))
	if result.Code != OnboardingFailed || result.Diagnostic.Phase != OnboardingPhaseHostKeyInspection || result.Diagnostic.Class != OnboardingClassHostKeyTimeout || !result.Diagnostic.Written || len(recorder.calls) != 1 {
		t.Fatalf("result=%+v recorder=%+v", result, recorder.calls)
	}
}

type onboardingKeyringStub struct{}

func (onboardingKeyringStub) Get(string) ([]byte, error) { return nil, nil }
func (onboardingKeyringStub) Set(string, []byte) error   { return nil }
func (onboardingKeyringStub) Delete(string) error        { return nil }

func onboardingRequest() OnboardingRequest {
	return OnboardingRequest{Name: "user-ibmi-example-test", Host: "ibmi.example.test", Port: 22, Username: "USER"}
}

func captureForTest(t *testing.T, service *OnboardingService, secret []byte) (OperationIdentity, remote.PromptCode) {
	return captureRequestForTest(t, service, onboardingRequest(), secret)
}

func captureRequestForTest(t *testing.T, service *OnboardingService, request OnboardingRequest, secret []byte) (OperationIdentity, remote.PromptCode) {
	t.Helper()
	input, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { input.Close() })
	output, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { output.Close() })
	prompt := remote.SecretPrompt{Input: input, Output: output, IsTerminal: func(int) bool { return true }, Read: func(int) ([]byte, error) { return secret, nil }}
	return service.Capture(context.Background(), request, prompt, input, output, "password: ")
}

func TestOnboardingOwnsAndZeroesSecretAfterAuditedProofBeforeSave(t *testing.T) {
	var events []string
	secret := []byte("password")
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
			events = append(events, "inspect")
			return remote.HostKeyObservation{Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", TrustCandidate: profile.HostKeyTrustTOFU}, nil
		},
		Proof: func(_ context.Context, _ profile.Profile, password []byte) Step8Result {
			events = append(events, "proof")
			if string(password) != "password" {
				t.Fatal("password was not delivered to proof")
			}
			return successfulProof()
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
	lease, capture := captureForTest(t, service, secret)
	if capture != remote.PromptCaptured {
		t.Fatalf("Capture() = %q", capture)
	}
	if code := service.StartCaptured(context.Background(), onboardingRequest(), lease); code != OnboardingStarted {
		t.Fatalf("StartCaptured() = %q", code)
	}
	id := lease
	if id.ID == "" {
		t.Fatal("Capture returned no lease")
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
	want := []string{"inspect", "proof", "identity_bootstrap_allowed", "save", "identity_pin_committed"}
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
	if id, code := service.Capture(context.Background(), onboardingRequest(), remote.SecretPrompt{}, nil, nil, "password: "); id.ID != "" || code == remote.PromptCaptured || calls != 0 {
		t.Fatalf("Capture() = %#v, %q, calls=%d", id, code, calls)
	}
}

func TestOnboardingRejectsUnauditedAutomaticTOFUBeforeAnySideEffect(t *testing.T) {
	inspected := 0
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
			inspected++
			return remote.HostKeyObservation{}, nil
		},
		Proof: func(context.Context, profile.Profile, []byte) Step8Result { return successfulProof() },
		Save:  func(profile.Profile) error { return nil },
	})
	lease, code := captureForTest(t, service, []byte("password"))
	if code != remote.PromptCaptured || service.StartCaptured(context.Background(), onboardingRequest(), lease) != OnboardingRejected || inspected != 0 {
		t.Fatalf("unaudited start = %#v, %q, inspected=%d", lease, code, inspected)
	}
}

func TestOnboardingCompensatesSavedProfileWhenCommittedAuditFails(t *testing.T) {
	var events []string
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
			return remote.HostKeyObservation{Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", TrustCandidate: profile.HostKeyTrustTOFU}, nil
		},
		Proof: func(context.Context, profile.Profile, []byte) Step8Result {
			events = append(events, "proof")
			return successfulProof()
		},
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
	id, capture := captureForTest(t, service, []byte("password"))
	if capture != remote.PromptCaptured || service.StartCaptured(context.Background(), onboardingRequest(), id) != OnboardingStarted {
		t.Fatalf("start = %q", capture)
	}
	result := service.Wait(context.Background(), id.ID)
	if result.Code != OnboardingFailed || result.CleanupRequired {
		t.Fatalf("committed audit result = %#v", result)
	}
	want := []string{"proof", "identity_bootstrap_allowed", "save", "identity_pin_committed", "delete"}
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
		Proof: func(context.Context, profile.Profile, []byte) Step8Result {
			t.Fatal("proof must not run for a mismatched pin")
			return Step8Result{}
		},
		Save:       func(profile.Profile) error { t.Fatal("save must not run for a mismatched pin"); return nil },
		Capability: func() credential.Capability { return credential.CapabilityUnsupported },
	})
	id, code := captureForTest(t, service, secret)
	if code != remote.PromptCaptured || service.StartCaptured(context.Background(), onboardingRequest(), id) != OnboardingStarted || id == (OperationIdentity{}) {
		t.Fatalf("Capture/StartCaptured() = %#v, %q", id, code)
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
		Proof: func(context.Context, profile.Profile, []byte) Step8Result {
			events = append(events, "proof")
			return successfulProof()
		},
		Save: func(profile.Profile) error { events = append(events, "save"); return nil },
		Audit: func(_ context.Context, event OnboardingAuditEvent) error {
			events = append(events, event.Code)
			return nil
		},
		Capability: func() credential.Capability { return credential.CapabilityUnsupported },
	})
	id, code := captureForTest(t, service, []byte("password"))
	if code != remote.PromptCaptured || service.StartCaptured(context.Background(), onboardingRequest(), id) != OnboardingStarted {
		t.Fatalf("Capture/StartCaptured() code = %q", code)
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
	recorder := &onboardingDiagnosticRecorderStub{}
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(ctx context.Context, _ string, _ int) (remote.HostKeyObservation, error) {
			close(started)
			<-ctx.Done()
			return remote.HostKeyObservation{}, ctx.Err()
		},
		Proof:       func(context.Context, profile.Profile, []byte) Step8Result { return successfulProof() },
		Save:        func(profile.Profile) error { saved++; return nil },
		Audit:       func(context.Context, OnboardingAuditEvent) error { return nil },
		Diagnostics: recorder,
	})
	id, code := captureForTest(t, service, []byte("password"))
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	if code != remote.PromptCaptured || service.StartCaptured(parent, onboardingRequest(), id) != OnboardingStarted {
		t.Fatalf("Capture/StartCaptured() = %q", code)
	}
	<-started
	cancel()
	if result := service.Wait(context.Background(), id.ID); result.Code != OnboardingCancelled {
		t.Fatalf("Wait() = %#v", result)
	}
	if saved != 0 || len(recorder.calls) != 0 {
		t.Fatalf("Save() calls=%d diagnostics=%d after cancellation", saved, len(recorder.calls))
	}
}

func TestOnboardingDelegatesPersistenceToPreparedCommitAfterProof(t *testing.T) {
	events := []string{}
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
			return remote.HostKeyObservation{Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", TrustCandidate: profile.HostKeyTrustTOFU}, nil
		},
		Proof: func(context.Context, profile.Profile, []byte) Step8Result {
			events = append(events, "proof")
			return successfulProof()
		},
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
	id, code := captureForTest(t, service, []byte("password"))
	if code != remote.PromptCaptured || service.StartCaptured(context.Background(), onboardingRequest(), id) != OnboardingStarted || service.Wait(context.Background(), id.ID).Code != OnboardingSaved {
		t.Fatal("prepared transaction did not save")
	}
	want := []string{"proof", "identity_bootstrap_allowed", "prepared-commit", "identity_pin_committed"}
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
		Proof:      func(context.Context, profile.Profile, []byte) Step8Result { return successfulProof() },
		Save:       func(profile.Profile) error { t.Fatal("legacy save bypassed prepared commit"); return nil },
		Audit:      func(context.Context, OnboardingAuditEvent) error { return nil },
		Capability: func() credential.Capability { return credential.CapabilitySupported },
		Keyring:    onboardingKeyringStub{},
		Commit: func(context.Context, profile.Profile, []byte, func(context.Context) error) profile.OnboardingCommitResult {
			return profile.OnboardingCommitResult{CleanupRequired: true, Err: errors.New("profile persistence failed")}
		},
	})
	id, code := captureForTest(t, service, []byte("password"))
	if code != remote.PromptCaptured || service.StartCaptured(context.Background(), onboardingRequest(), id) != OnboardingStarted {
		t.Fatalf("Capture/StartCaptured() = %q", code)
	}
	result := service.Wait(context.Background(), id.ID)
	if result.Code != OnboardingFailed || !result.CleanupRequired || !result.CredentialRetained {
		t.Fatalf("save failure result = %#v; want not saved, retained credential, cleanup guidance classification", result)
	}
}

func TestOnboardingLeaseRevocationZeroesReplacementExpiryStaleAndFailedCapture(t *testing.T) {
	now := time.Unix(100, 0)
	service := NewOnboardingService(OnboardingDeps{Now: func() time.Time { return now }})
	request := onboardingRequest()
	first, second, third := []byte("first-secret"), []byte("second-secret"), []byte("third-secret")
	firstID, code := captureRequestForTest(t, service, request, first)
	if code != remote.PromptCaptured {
		t.Fatalf("first Capture() = %q", code)
	}
	firstLease := service.leases[firstID.ID].secret
	secondID, code := captureRequestForTest(t, service, request, second)
	if code != remote.PromptCaptured || !zeroed(firstLease) {
		t.Fatalf("replacement Capture() = %q, first lease zeroed=%t", code, zeroed(firstLease))
	}
	secondLease := service.leases[secondID.ID].secret
	service.Revoke(OperationIdentity{ID: secondID.ID, Generation: secondID.Generation + 1})
	if !zeroed(secondLease) {
		t.Fatal("stale revoke did not zero the active lease")
	}
	thirdID, code := captureRequestForTest(t, service, request, third)
	if code != remote.PromptCaptured {
		t.Fatalf("third Capture() = %q", code)
	}
	thirdLease := service.leases[thirdID.ID].secret
	now = now.Add(2*time.Minute + time.Nanosecond)
	if got := service.StartCaptured(context.Background(), request, thirdID); got != OnboardingRejected || !zeroed(thirdLease) {
		t.Fatalf("expired StartCaptured() = %q, lease zeroed=%t", got, zeroed(thirdLease))
	}

	failed := []byte("failed-secret")
	input, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	prompt := remote.SecretPrompt{Input: input, Output: output, IsTerminal: func(int) bool { return true }, Read: func(int) ([]byte, error) { return failed, errors.New("capture failed") }}
	if id, got := service.Capture(context.Background(), request, prompt, input, output, "password: "); id != (OperationIdentity{}) || got == remote.PromptCaptured || !zeroed(failed) {
		t.Fatalf("failed Capture() = %#v, %q, input zeroed=%t", id, got, zeroed(failed))
	}
}

func TestOnboardingShutdownWaitsForWorkerAndZeroesSecret(t *testing.T) {
	proofStarted := make(chan struct{})
	var proofSecret []byte
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
			return remote.HostKeyObservation{Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", TrustCandidate: profile.HostKeyTrustTOFU}, nil
		},
		Proof: func(ctx context.Context, _ profile.Profile, secret []byte) Step8Result {
			proofSecret = secret
			close(proofStarted)
			<-ctx.Done()
			return Step8Result{Decision: DecisionTerminal, Class: ResultCancelled}
		},
		Save:  func(profile.Profile) error { t.Fatal("Save must not run after shutdown"); return nil },
		Audit: func(context.Context, OnboardingAuditEvent) error { return nil },
	})
	id, code := captureForTest(t, service, []byte("worker-secret"))
	if code != remote.PromptCaptured || service.StartCaptured(context.Background(), onboardingRequest(), id) != OnboardingStarted {
		t.Fatalf("Capture/StartCaptured() = %q", code)
	}
	<-proofStarted
	service.Shutdown()
	if result := service.Wait(context.Background(), id.ID); result.Code != OnboardingCancelled || !zeroed(proofSecret) {
		t.Fatalf("Wait() = %#v, proof secret zeroed=%t", result, zeroed(proofSecret))
	}
	unused, code := captureForTest(t, service, []byte("unused-secret"))
	if code != remote.PromptCaptured {
		t.Fatalf("unused Capture() = %q", code)
	}
	unusedLease := service.leases[unused.ID].secret
	service.Shutdown()
	if !zeroed(unusedLease) {
		t.Fatal("Shutdown did not zero an unconsumed lease")
	}
}

func TestOnboardingCanaryNeverEscapesSecretOwningBoundary(t *testing.T) {
	const canary = "canary-secret-never-serialize"
	var auditJSON, profileJSON []byte
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
			return remote.HostKeyObservation{Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", TrustCandidate: profile.HostKeyTrustTOFU}, nil
		},
		Proof: func(context.Context, profile.Profile, []byte) Step8Result { return successfulProof() },
		Audit: func(_ context.Context, event OnboardingAuditEvent) error {
			var err error
			auditJSON, err = json.Marshal(event)
			return err
		},
		Save: func(p profile.Profile) error {
			var err error
			profileJSON, err = json.Marshal(p)
			return err
		},
		Capability: func() credential.Capability { return credential.CapabilityUnsupported },
	})
	id, code := captureForTest(t, service, []byte(canary))
	if code != remote.PromptCaptured || service.StartCaptured(context.Background(), onboardingRequest(), id) != OnboardingStarted {
		t.Fatalf("Capture/StartCaptured() = %q", code)
	}
	result := service.Wait(context.Background(), id.ID)
	resultJSON, err := json.Marshal(result)
	if err != nil || result.Code != OnboardingSaved {
		t.Fatalf("result = %#v, marshal error = %v", result, err)
	}
	store := profile.Store{Root: t.TempDir()}
	if err := store.WritePreparedCreate(profile.PreparedCreateJournal{Profile: onboardingRequest().Name, TransactionID: "sanitized-recovery", Phase: profile.PreparedCreateSaving}); err != nil {
		t.Fatal(err)
	}
	recoveryJSON, err := os.ReadFile(filepath.Join(store.Root, ".prepared-create-"+onboardingRequest().Name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, surface := range []string{string(auditJSON), string(profileJSON), string(recoveryJSON), string(resultJSON), fmt.Sprintf("%+v", result), fmt.Sprint(errors.New("onboarding failed")), reflect.TypeOf(result).String(), reflect.TypeOf(id).String()} {
		if strings.Contains(surface, canary) {
			t.Fatalf("secret canary escaped into outward surface %q", surface)
		}
	}
}

func TestOnboardingRequiredAuditFailurePreventsCommitAndZeroesWorkerSecret(t *testing.T) {
	var proofSecret []byte
	commits := 0
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
			return remote.HostKeyObservation{Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", TrustCandidate: profile.HostKeyTrustTOFU}, nil
		},
		Proof: func(_ context.Context, _ profile.Profile, secret []byte) Step8Result {
			proofSecret = secret
			return successfulProof()
		},
		Save: func(profile.Profile) error { t.Fatal("Save must not run after required audit failure"); return nil },
		Audit: func(context.Context, OnboardingAuditEvent) error {
			return errors.New("required audit unavailable")
		},
		Commit: func(context.Context, profile.Profile, []byte, func(context.Context) error) profile.OnboardingCommitResult {
			commits++
			return profile.OnboardingCommitResult{Saved: true}
		},
	})
	id, code := captureForTest(t, service, []byte("audit-failure-secret"))
	if code != remote.PromptCaptured || service.StartCaptured(context.Background(), onboardingRequest(), id) != OnboardingStarted {
		t.Fatalf("Capture/StartCaptured() = %q", code)
	}
	if result := service.Wait(context.Background(), id.ID); result.Code != OnboardingFailed || commits != 0 || !zeroed(proofSecret) {
		t.Fatalf("Wait() = %#v, commits=%d, proof secret zeroed=%t", result, commits, zeroed(proofSecret))
	}
}

func zeroed(value []byte) bool {
	for _, b := range value {
		if b != 0 {
			return false
		}
	}
	return true
}

func successfulProof() Step8Result {
	return Step8Result{Decision: DecisionWSSSelected, Class: ResultProofSuccess, ProofRevision: ProofRevision, Cleanup: true}
}

type onboardingDiagnosticRecorderStub struct {
	calls []OnboardingDiagnostic
	err   error
}

func (r *onboardingDiagnosticRecorderStub) Record(_ context.Context, phase OnboardingFailurePhase, class OnboardingFailureClass, cleanup, retained bool) (string, error) {
	r.calls = append(r.calls, OnboardingDiagnostic{Phase: phase, Class: class})
	if r.err != nil {
		return "", r.err
	}
	return "ONB-0123456789abcdef0123456789abcdef", nil
}

func TestOnboardingFailureClassificationRecordsEveryFailureSite(t *testing.T) {
	for _, test := range []struct {
		name, phase string
		class       OnboardingFailureClass
		modify      func(*OnboardingDeps)
	}{
		{"host key", string(OnboardingPhaseHostKeyInspection), OnboardingClassHostKeyUnavailable, func(d *OnboardingDeps) {
			d.Inspect = func(context.Context, string, int) (remote.HostKeyObservation, error) {
				return remote.HostKeyObservation{}, errors.New("raw host failure")
			}
		}},
		{"existing identity", string(OnboardingPhaseExistingIdentity), OnboardingClassIdentityFailure, func(d *OnboardingDeps) {
			d.Existing = func(context.Context, string) (*profile.Profile, error) {
				return nil, errors.New("raw identity failure")
			}
		}},
		{"proof terminal class", string(OnboardingPhaseAuthenticatedProof), OnboardingFailureClass(ResultAuthenticationFailed), func(d *OnboardingDeps) {
			d.Proof = func(context.Context, profile.Profile, []byte) Step8Result {
				return Step8Result{Decision: DecisionTerminal, Class: ResultAuthenticationFailed}
			}
		}},
		{"bootstrap audit", string(OnboardingPhaseBootstrapAudit), OnboardingClassBootstrapAuditFailure, func(d *OnboardingDeps) {
			d.Audit = func(context.Context, OnboardingAuditEvent) error { return errors.New("raw audit failure") }
		}},
		{"keyring precondition", string(OnboardingPhaseKeyringPrecondition), OnboardingClassKeyringUnavailable, func(d *OnboardingDeps) {
			d.Capability = func() credential.Capability { return credential.CapabilitySupported }
			d.Keyring = nil
		}},
		{"commit", string(OnboardingPhaseCommit), OnboardingClassCommitFailure, func(d *OnboardingDeps) {
			d.Commit = func(context.Context, profile.Profile, []byte, func(context.Context) error) profile.OnboardingCommitResult {
				return profile.OnboardingCommitResult{Err: errors.New("raw commit failure")}
			}
		}},
		{"legacy save", string(OnboardingPhaseSave), OnboardingClassSaveFailure, func(d *OnboardingDeps) {
			d.Save = func(profile.Profile) error { return errors.New("raw save failure") }
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := &onboardingDiagnosticRecorderStub{}
			deps := OnboardingDeps{
				Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
					return remote.HostKeyObservation{Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}, nil
				},
				Proof: successfulProofFunc, Save: func(profile.Profile) error { return nil }, Audit: func(context.Context, OnboardingAuditEvent) error { return nil },
				Capability: func() credential.Capability { return credential.CapabilityUnsupported }, Diagnostics: recorder,
			}
			test.modify(&deps)
			result := NewOnboardingService(deps).run(context.Background(), onboardingRequest(), []byte("secret"))
			if result.Code != OnboardingFailed || string(result.Diagnostic.Phase) != test.phase || result.Diagnostic.Class != test.class || !result.Diagnostic.Written || len(recorder.calls) != 1 {
				t.Fatalf("result=%+v calls=%+v", result, recorder.calls)
			}
		})
	}
}

func TestOnboardingRetainsSafeUploadStageWhenDiagnosticsAreUnavailable(t *testing.T) {
	service := NewOnboardingService(OnboardingDeps{
		Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
			return remote.HostKeyObservation{Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}, nil
		},
		Proof: func(context.Context, profile.Profile, []byte) Step8Result {
			return Step8Result{Decision: DecisionTerminal, Class: ResultUploadFailure, ArtifactStage: mapepirestdio.ArtifactStageDirectoryPrepare}
		},
		Save:        func(profile.Profile) error { return nil },
		Audit:       func(context.Context, OnboardingAuditEvent) error { return nil },
		Diagnostics: &onboardingDiagnosticRecorderStub{err: errors.New("diagnostics unavailable")},
	})
	result := service.run(context.Background(), onboardingRequest(), []byte("secret"))
	if result.Code != OnboardingFailed || result.Diagnostic.Written || result.Diagnostic.ArtifactStage != mapepirestdio.ArtifactStageDirectoryPrepare {
		t.Fatalf("result=%+v", result)
	}
}

func TestOnboardingPreservesOnlySafeHostKeyFailureClasses(t *testing.T) {
	for _, test := range []struct {
		name        string
		observation remote.HostKeyObservation
		err         error
		want        OnboardingFailureClass
	}{
		{"timeout", remote.HostKeyObservation{}, &remote.HostKeyProbeError{Kind: remote.HostKeyProbeTimeout}, OnboardingClassHostKeyTimeout},
		{"negotiation", remote.HostKeyObservation{}, &remote.HostKeyProbeError{Kind: remote.HostKeyProbeNegotiation}, OnboardingClassHostKeyNegotiation},
		{"no key", remote.HostKeyObservation{}, &remote.HostKeyProbeError{Kind: remote.HostKeyProbeNoKey}, OnboardingClassHostKeyNoKey},
		{"unavailable", remote.HostKeyObservation{}, errors.New("host.example.test raw-error-canary"), OnboardingClassHostKeyUnavailable},
		{"invalid candidate", remote.HostKeyObservation{}, nil, OnboardingClassHostKeyInvalidCandidate},
		{"unknown probe kind", remote.HostKeyObservation{}, &remote.HostKeyProbeError{Kind: remote.HostKeyProbeFailure("raw-error-canary")}, OnboardingClassHostKeyFailure},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := &onboardingDiagnosticRecorderStub{}
			service := NewOnboardingService(OnboardingDeps{
				Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
					return test.observation, test.err
				},
				Diagnostics: recorder,
			})
			result := service.run(context.Background(), onboardingRequest(), []byte("secret"))
			if result.Code != OnboardingFailed || result.Diagnostic.Phase != OnboardingPhaseHostKeyInspection || result.Diagnostic.Class != test.want || !result.Diagnostic.Written || len(recorder.calls) != 1 || recorder.calls[0].Class != test.want {
				t.Fatalf("result=%+v recorder=%+v", result, recorder.calls)
			}
			if strings.Contains(fmt.Sprintf("%+v", result), "raw-error-canary") {
				t.Fatalf("result leaked unsafe diagnostic data: %+v", result)
			}
		})
	}
}

func successfulProofFunc(context.Context, profile.Profile, []byte) Step8Result {
	return successfulProof()
}

func TestOnboardingProofMalformedAndCancellationDoNotPersistUnsafeDiagnostics(t *testing.T) {
	recorder := &onboardingDiagnosticRecorderStub{}
	deps := OnboardingDeps{Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
		return remote.HostKeyObservation{Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}, nil
	}, Proof: func(context.Context, profile.Profile, []byte) Step8Result {
		return Step8Result{Decision: DecisionSSHEligible, Class: ResultAuthenticationFailed}
	}, Save: func(profile.Profile) error { return nil }, Audit: func(context.Context, OnboardingAuditEvent) error { return nil }, Diagnostics: recorder}
	result := NewOnboardingService(deps).run(context.Background(), onboardingRequest(), []byte("secret"))
	if result.Diagnostic.Class != OnboardingClassProofFailure || !result.Diagnostic.Written {
		t.Fatalf("malformed proof result=%+v", result)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	result = NewOnboardingService(deps).run(cancelled, onboardingRequest(), []byte("secret"))
	if result.Code != OnboardingCancelled || len(recorder.calls) != 1 {
		t.Fatalf("cancelled result=%+v calls=%d", result, len(recorder.calls))
	}
}

func TestOnboardingDiagnosticRecorderFailureDoesNotChangePrimaryFailureOrLeakCanaries(t *testing.T) {
	const canary = "raw-error-and-secret-canary"
	recorder := &onboardingDiagnosticRecorderStub{err: errors.New(canary)}
	deps := OnboardingDeps{Inspect: func(context.Context, string, int) (remote.HostKeyObservation, error) {
		return remote.HostKeyObservation{}, errors.New(canary)
	}, Proof: successfulProofFunc, Save: func(profile.Profile) error { return nil }, Audit: func(context.Context, OnboardingAuditEvent) error { return nil }, Diagnostics: recorder}
	result := NewOnboardingService(deps).run(context.Background(), onboardingRequest(), []byte(canary))
	encoded, err := json.Marshal(result)
	if err != nil || result.Code != OnboardingFailed || result.Diagnostic.Written || result.Diagnostic.Reference != "" || strings.Contains(string(encoded), canary) || strings.Contains(fmt.Sprintf("%+v", result), canary) {
		t.Fatalf("result=%+v json=%s err=%v", result, encoded, err)
	}
}
