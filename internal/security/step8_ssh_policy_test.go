package security

import (
	"context"
	"testing"
	"time"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/profile"
)

func TestStep8SSHPolicyAllowsOnlyApprovedSavedFallbackProfile(t *testing.T) {
	approved := step8PolicyProfile()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, cancelExpired := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancelExpired()

	tests := []struct {
		name    string
		ctx     context.Context
		profile profile.Profile
		allow   bool
	}{
		{name: "approved saved profile", ctx: context.Background(), profile: approved, allow: true},
		{name: "missing policy reference", ctx: context.Background(), profile: profileWithoutPolicyRef(approved)},
		{name: "blank policy reference", ctx: context.Background(), profile: withPolicyRef(approved, "")},
		{name: "malformed policy reference", ctx: context.Background(), profile: withPolicyRef(approved, "verified-readonly\n")},
		{name: "unapproved policy reference", ctx: context.Background(), profile: withPolicyRef(approved, "other-policy")},
		{name: "fallback disabled", ctx: context.Background(), profile: withoutFallback(approved)},
		{name: "invalid profile", ctx: context.Background(), profile: profile.Profile{}},
		{name: "cancelled context", ctx: cancelled, profile: approved},
		{name: "expired context", ctx: expired, profile: approved},
	}

	policy := NewStep8SSHPolicy()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := policy.AllowSSH(tt.ctx, tt.profile)
			if tt.allow {
				if err != nil {
					t.Fatalf("AllowSSH() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("AllowSSH() error = nil, want fail-closed denial")
			}
			if got, want := err.Error(), "ssh_fallback_policy_denied"; got != want {
				t.Fatalf("AllowSSH() error = %q, want sanitized %q", got, want)
			}
		})
	}
}

func TestStep8SSHPolicyDenialPrecedesTrustCredentialAndRuntime(t *testing.T) {
	trust := &step8CountingTrust{}
	credentials := &step8CountingCredentials{}
	runCalls := 0
	gate := configuration.PostObservationGate{
		Policy:      NewStep8SSHPolicy(),
		Trust:       trust,
		Credentials: credentials,
	}
	denied := withoutFallback(step8PolicyProfile())

	result := gate.ApplyWithCredential(context.Background(), configuration.Step8Request{
		RequestID: "request-1",
		Profile:   denied,
		Consent:   true,
	}, configuration.Observation{
		Decision: configuration.DecisionSSHEligible,
		Reason:   configuration.ReasonDaemonUnavailable,
	}, func([]byte) configuration.Step8Result {
		runCalls++
		return configuration.Step8Result{}
	})

	if result.Decision != configuration.DecisionTerminal || result.Class != configuration.ResultAuthorizationDenied {
		t.Fatalf("gate result = %+v, want authorization denial", result)
	}
	if trust.calls != 0 || credentials.calls != 0 || runCalls != 0 {
		t.Fatalf("denial calls: trust=%d credentials=%d runtime=%d, want all zero", trust.calls, credentials.calls, runCalls)
	}
}

func TestStep8SSHPolicyAllowsApprovedProfileToReachCredentialAndRuntime(t *testing.T) {
	trust := &step8CountingTrust{}
	credentials := &step8CountingCredentials{}
	runCalls := 0
	gate := configuration.PostObservationGate{
		Policy:      NewStep8SSHPolicy(),
		Trust:       trust,
		Credentials: credentials,
	}

	result := gate.ApplyWithCredential(context.Background(), configuration.Step8Request{
		RequestID: "request-1",
		Profile:   step8PolicyProfile(),
		Consent:   true,
	}, configuration.Observation{
		Decision: configuration.DecisionSSHEligible,
		Reason:   configuration.ReasonDaemonUnavailable,
	}, func([]byte) configuration.Step8Result {
		runCalls++
		return configuration.Step8Result{Decision: configuration.DecisionWSSSelected, Class: configuration.ResultProofSuccess}
	})

	if result.Decision != configuration.DecisionWSSSelected || result.Class != configuration.ResultProofSuccess {
		t.Fatalf("gate result = %+v, want runtime proof success", result)
	}
	if trust.calls != 1 || credentials.calls != 1 || runCalls != 1 {
		t.Fatalf("approved calls: trust=%d credentials=%d runtime=%d, want one each", trust.calls, credentials.calls, runCalls)
	}
}

type step8CountingTrust struct{ calls int }

func (f *step8CountingTrust) VerifySSH(context.Context, profile.Profile) error {
	f.calls++
	return nil
}

type step8CountingCredentials struct{ calls int }

func (f *step8CountingCredentials) Get(context.Context, string, profile.CredentialMode) ([]byte, error) {
	f.calls++
	return []byte("opaque"), nil
}

func step8PolicyProfile() profile.Profile {
	return profile.Profile{
		SchemaVersion:     profile.SchemaVersionV3,
		Name:              "saved",
		Host:              "ibmi.example.test",
		Port:              22,
		Username:          "USER",
		CredentialMode:    profile.CredentialModePrompt,
		EndpointPolicyRef: configuration.VerifiedReadOnlyEndpointPolicyRef,
		FallbackAllowed:   true,
	}
}

func profileWithoutPolicyRef(p profile.Profile) profile.Profile {
	p.EndpointPolicyRef = ""
	return p
}

func withPolicyRef(p profile.Profile, ref string) profile.Profile {
	p.EndpointPolicyRef = ref
	return p
}

func withoutFallback(p profile.Profile) profile.Profile {
	p.FallbackAllowed = false
	return p
}
