package security

import (
	"context"
	"errors"
	"testing"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/profile"
)

const testSSHFingerprint = "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

func TestSSHTrustVerifiesOnlyExactIndependentSSHEnrollment(t *testing.T) {
	p := profile.Profile{
		TLSTrust: profile.TrustEvidence{Mode: profile.TrustModePin, Pin: "sha256/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", Provenance: "tls"},
		SSHTrust: profile.TrustEvidence{Mode: profile.TrustModeTOFU, Pin: testSSHFingerprint, Provenance: "ssh"},
	}
	if err := (SSHTrust{ObservedFingerprint: testSSHFingerprint}).VerifySSH(context.Background(), p); err != nil {
		t.Fatalf("VerifySSH error = %v", err)
	}
}

func TestSSHTrustFailsClosedWithoutTLSReuse(t *testing.T) {
	tests := []struct {
		name     string
		sshTrust profile.TrustEvidence
		observed string
		class    SSHTrustFailure
	}{
		{name: "missing SSH enrollment despite TLS trust", observed: testSSHFingerprint, class: SSHTrustMissing},
		{name: "mismatch", sshTrust: profile.TrustEvidence{Mode: profile.TrustModeTOFU, Pin: testSSHFingerprint, Provenance: "ssh"}, observed: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAE", class: SSHTrustMismatch},
		{name: "unapproved CA mode", sshTrust: profile.TrustEvidence{Mode: profile.TrustModeCA, Provenance: "ssh"}, observed: testSSHFingerprint, class: SSHTrustUnapproved},
		{name: "unknown mode", sshTrust: profile.TrustEvidence{Mode: profile.TrustMode("unknown"), Pin: testSSHFingerprint, Provenance: "ssh"}, observed: testSSHFingerprint, class: SSHTrustUnapproved},
		{name: "missing observed fingerprint", sshTrust: profile.TrustEvidence{Mode: profile.TrustModePin, Pin: testSSHFingerprint, Provenance: "ssh"}, class: SSHTrustMissing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := profile.Profile{TLSTrust: profile.TrustEvidence{Mode: profile.TrustModePin, Pin: "sha256/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", Provenance: "tls"}, SSHTrust: tt.sshTrust}
			err := (SSHTrust{ObservedFingerprint: tt.observed}).VerifySSH(context.Background(), p)
			var trustErr *SSHTrustError
			if !errors.As(err, &trustErr) || trustErr.Failure != tt.class {
				t.Fatalf("error = %v, want SSH trust failure %q", err, tt.class)
			}
			if err.Error() != "ssh_trust_blocked" {
				t.Fatalf("error = %q, want sanitized terminal classification", err)
			}
		})
	}
}

func TestSSHTrustCancellationFailsClosed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := (SSHTrust{ObservedFingerprint: testSSHFingerprint}).VerifySSH(ctx, profile.Profile{SSHTrust: profile.TrustEvidence{Mode: profile.TrustModeTOFU, Pin: testSSHFingerprint, Provenance: "ssh"}})
	var trustErr *SSHTrustError
	if !errors.As(err, &trustErr) || trustErr.Failure != SSHTrustUnavailable {
		t.Fatalf("error = %v, want unavailable SSH trust failure", err)
	}
}

func TestSSHTrustBlocksGateBeforeCredentialOrRuntime(t *testing.T) {
	credentialCalls, runtimeCalls := 0, 0
	gate := configuration.PostObservationGate{
		Policy: allowSSHPolicy{},
		Trust:  SSHTrust{ObservedFingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAE"},
		Credentials: credentialProviderFunc(func(context.Context, string, profile.CredentialMode) ([]byte, error) {
			credentialCalls++
			return []byte("prompt-secret"), nil
		}),
	}
	result := gate.ApplyWithCredential(context.Background(), configuration.Step8Request{RequestID: "request-1", Profile: validSSHTrustProfile(), Consent: true}, configuration.Observation{Decision: configuration.DecisionSSHEligible, Reason: configuration.ReasonDaemonUnavailable}, func([]byte) configuration.Step8Result {
		runtimeCalls++
		return configuration.Step8Result{}
	})
	if result.Class != configuration.ResultTrustMismatch || result.Decision != configuration.DecisionTerminal {
		t.Fatalf("result = %#v, want terminal trust mismatch", result)
	}
	if credentialCalls != 0 || runtimeCalls != 0 {
		t.Fatalf("credential calls = %d, runtime calls = %d; want zero", credentialCalls, runtimeCalls)
	}
}

type allowSSHPolicy struct{}

func (allowSSHPolicy) AllowSSH(context.Context, profile.Profile) error { return nil }

type credentialProviderFunc func(context.Context, string, profile.CredentialMode) ([]byte, error)

func (f credentialProviderFunc) Get(ctx context.Context, key string, mode profile.CredentialMode) ([]byte, error) {
	return f(ctx, key, mode)
}

func validSSHTrustProfile() profile.Profile {
	return profile.Profile{
		SchemaVersion:      profile.SchemaVersionV3,
		Name:               "dev",
		Host:               "ibmi.example.test",
		Port:               22,
		Username:           "NEXUS$USER",
		HostKeyFingerprint: testSSHFingerprint,
		HostKeyTrust:       profile.HostKeyTrustVerified,
		CredentialMode:     profile.CredentialModePrompt,
		FallbackAllowed:    true,
		SSHTrust:           profile.TrustEvidence{Mode: profile.TrustModeTOFU, Pin: testSSHFingerprint, Provenance: "ssh"},
	}
}
