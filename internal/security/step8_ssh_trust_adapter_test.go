package security

import (
	"context"
	"errors"
	"testing"
	"time"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/profile"
)

const (
	step8SSHObservedFingerprint = "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	step8SSHRotatedFingerprint  = "SHA256:AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE"
)

func TestStep8SSHTrustAdapterVerifiesOnlyIndependentSSHEnrollment(t *testing.T) {
	validProfile := step8SSHTrustProfile()
	validProfile.TLSTrust = profile.TrustEvidence{
		Mode:       profile.TrustModePin,
		Pin:        step8SSHObservedFingerprint,
		Provenance: "tls-looks-like-ssh",
	}

	tests := []struct {
		name     string
		ctx      context.Context
		profile  profile.Profile
		observer observedSSHFingerprintFunc
		want     SSHTrustFailure
	}{
		{"observer receives only host and port", context.Background(), validProfile, func(_ context.Context, host string, port int) (string, error) {
			if host != "ibmi.example.test" || port != 22 {
				t.Fatalf("observer coordinates = %q:%d, want ibmi.example.test:22", host, port)
			}
			return step8SSHObservedFingerprint, nil
		}, ""},
		{"SSH rotation blocks", context.Background(), validProfile, func(context.Context, string, int) (string, error) { return step8SSHRotatedFingerprint, nil }, SSHTrustMismatch},
		{"TLS evidence cannot replace SSH enrollment", context.Background(), func() profile.Profile { p := validProfile; p.SSHTrust = profile.TrustEvidence{}; return p }(), func(context.Context, string, int) (string, error) { return step8SSHObservedFingerprint, nil }, SSHTrustMissing},
		{"missing observed fingerprint blocks", context.Background(), validProfile, func(context.Context, string, int) (string, error) { return "", nil }, SSHTrustMissing},
		{"malformed observed fingerprint blocks", context.Background(), validProfile, func(context.Context, string, int) (string, error) { return "not-a-fingerprint", nil }, SSHTrustUnapproved},
		{"unapproved SSH enrollment blocks", context.Background(), func() profile.Profile { p := validProfile; p.SSHTrust.Mode = profile.TrustModeCA; return p }(), func(context.Context, string, int) (string, error) { return step8SSHObservedFingerprint, nil }, SSHTrustUnapproved},
		{"observer failure blocks without detail", context.Background(), validProfile, func(context.Context, string, int) (string, error) { return "", errors.New("observer host detail") }, SSHTrustUnavailable},
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, cancelDeadline := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelDeadline()
	tests = append(tests,
		struct {
			name     string
			ctx      context.Context
			profile  profile.Profile
			observer observedSSHFingerprintFunc
			want     SSHTrustFailure
		}{"cancellation blocks before observation", cancelled, validProfile, func(context.Context, string, int) (string, error) {
			t.Fatal("observer called after cancellation")
			return "", nil
		}, SSHTrustUnavailable},
		struct {
			name     string
			ctx      context.Context
			profile  profile.Profile
			observer observedSSHFingerprintFunc
			want     SSHTrustFailure
		}{"deadline blocks before observation", expired, validProfile, func(context.Context, string, int) (string, error) {
			t.Fatal("observer called after deadline")
			return "", nil
		}, SSHTrustUnavailable},
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewStep8SSHTrustAdapter(tt.observer).VerifySSH(tt.ctx, tt.profile)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("VerifySSH() error = %v", err)
				}
				return
			}
			var trustErr *SSHTrustError
			if !errors.As(err, &trustErr) || trustErr.Failure != tt.want || err.Error() != "ssh_trust_blocked" {
				t.Fatalf("VerifySSH() error = %#v, want sanitized %q", err, tt.want)
			}
		})
	}
}

func TestStep8SSHTrustAdapterDenialPrecedesConsentCredentialsAndRuntime(t *testing.T) {
	credentials := &step8TrustCredentials{}
	runtimeCalls := 0
	gate := configuration.PostObservationGate{
		Policy:      NewStep8SSHPolicy(),
		Trust:       NewStep8SSHTrustAdapter(observedSSHFingerprintFunc(func(context.Context, string, int) (string, error) { return step8SSHRotatedFingerprint, nil })),
		Credentials: credentials,
	}

	result := gate.ApplyWithCredential(context.Background(), configuration.Step8Request{
		RequestID: "request-1",
		Profile:   step8SSHTrustProfile(),
		Consent:   false,
	}, configuration.Observation{Decision: configuration.DecisionSSHEligible, Reason: configuration.ReasonDaemonUnavailable}, func([]byte) configuration.Step8Result {
		runtimeCalls++
		return configuration.Step8Result{}
	})
	if result.Decision != configuration.DecisionTerminal || result.Class != configuration.ResultTrustMismatch {
		t.Fatalf("result = %#v, want terminal trust mismatch", result)
	}
	if credentials.calls != 0 || runtimeCalls != 0 {
		t.Fatalf("credential calls=%d runtime calls=%d, want both zero", credentials.calls, runtimeCalls)
	}
}

func TestStep8SSHTrustAdapterMatchingObservationReachesCredentialsAndRuntime(t *testing.T) {
	credentials := &step8TrustCredentials{}
	runtimeCalls := 0
	gate := configuration.PostObservationGate{
		Policy:      NewStep8SSHPolicy(),
		Trust:       NewStep8SSHTrustAdapter(observedSSHFingerprintFunc(func(context.Context, string, int) (string, error) { return step8SSHObservedFingerprint, nil })),
		Credentials: credentials,
	}

	result := gate.ApplyWithCredential(context.Background(), configuration.Step8Request{
		RequestID: "request-1",
		Profile:   step8SSHTrustProfile(),
		Consent:   true,
	}, configuration.Observation{Decision: configuration.DecisionSSHEligible, Reason: configuration.ReasonDaemonUnavailable}, func([]byte) configuration.Step8Result {
		runtimeCalls++
		return configuration.Step8Result{Decision: configuration.DecisionSSHEligible, Class: configuration.ResultProofSuccess}
	})
	if result.Decision != configuration.DecisionSSHEligible || result.Class != configuration.ResultProofSuccess {
		t.Fatalf("result = %#v, want SSH proof success", result)
	}
	if credentials.calls != 1 || runtimeCalls != 1 {
		t.Fatalf("credential calls=%d runtime calls=%d, want both one", credentials.calls, runtimeCalls)
	}
}

func TestStep8SSHTrustAdapterBoundsFreshObservationByOperationAndParentDeadlines(t *testing.T) {
	parentDeadline := time.Now().Add(time.Second)
	parent, cancelParent := context.WithDeadline(context.Background(), parentDeadline)
	defer cancelParent()
	var observed context.Context
	err := NewStep8SSHTrustAdapter(observedSSHFingerprintFunc(func(ctx context.Context, _ string, _ int) (string, error) {
		observed = ctx
		return step8SSHObservedFingerprint, nil
	})).VerifySSH(parent, step8SSHTrustProfile())
	if err != nil {
		t.Fatalf("VerifySSH() error = %v", err)
	}
	deadline, hasDeadline := observed.Deadline()
	if !hasDeadline || deadline.After(parentDeadline) {
		t.Fatalf("fresh observation deadline = %v, want bounded by parent %v", deadline, parentDeadline)
	}
	select {
	case <-observed.Done():
	default:
		t.Fatal("fresh observation context was not cancelled promptly")
	}
}

type observedSSHFingerprintFunc func(context.Context, string, int) (string, error)

func (f observedSSHFingerprintFunc) ObserveSSHFingerprint(ctx context.Context, host string, port int) (string, error) {
	return f(ctx, host, port)
}

type step8TrustCredentials struct{ calls int }

func (c *step8TrustCredentials) Get(context.Context, string, profile.CredentialMode) ([]byte, error) {
	c.calls++
	return []byte("credential"), nil
}

func step8SSHTrustProfile() profile.Profile {
	return profile.Profile{
		SchemaVersion:     profile.SchemaVersionV3,
		Name:              "saved",
		Host:              "ibmi.example.test",
		Port:              22,
		Username:          "USER",
		CredentialMode:    profile.CredentialModePrompt,
		EndpointPolicyRef: "verified-readonly",
		FallbackAllowed:   true,
		TLSTrust:          profile.TrustEvidence{Mode: profile.TrustModePin, Pin: "sha256/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", Provenance: "tls-enrollment"},
		SSHTrust:          profile.TrustEvidence{Mode: profile.TrustModeTOFU, Pin: step8SSHObservedFingerprint, Provenance: "ssh-enrollment"},
	}
}
