package remote

import (
	"context"
	"errors"
	"testing"

	"bac-nexus/internal/hostidentity"
	"bac-nexus/internal/profile"
)

func TestHostIdentityInspectorIsConnectorNeutral(t *testing.T) {
	var _ hostidentity.Inspector = HostIdentityInspector{}
	// The adapter's public method accepts only context, host, and port; credentials,
	// profiles, sessions, and persistence cannot cross this boundary.
	_, _ = HostIdentityInspector{}.InspectHostKey(context.Background(), "bad host", 22)
}

func TestMapHostIdentityObservationAcceptsOnlyCompleteUnverifiedTOFU(t *testing.T) {
	valid := HostKeyObservation{Algorithm: "ssh-ed25519", Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", TrustCandidate: profile.HostKeyTrustTOFU}
	got, err := mapHostIdentityObservation(valid)
	if err != nil || got.Algorithm != valid.Algorithm || got.Fingerprint != valid.Fingerprint {
		t.Fatalf("candidate = %#v, %v", got, err)
	}
	for _, observation := range []HostKeyObservation{{}, {Algorithm: "ssh-ed25519", Fingerprint: valid.Fingerprint, Verified: true, TrustCandidate: profile.HostKeyTrustTOFU}, {Algorithm: "ssh-ed25519", Fingerprint: valid.Fingerprint, TrustCandidate: profile.HostKeyTrustVerified}} {
		if _, err := mapHostIdentityObservation(observation); hostidentity.SafeFailure(err) != hostidentity.FailureInvalidCandidate {
			t.Fatalf("error = %v", err)
		}
	}
}

func TestMapHostIdentityFailureProducesOnlySafeCategories(t *testing.T) {
	for _, tt := range []struct {
		err  error
		want hostidentity.Failure
	}{
		{context.Canceled, hostidentity.FailureCancelled},
		{&HostKeyProbeError{Kind: HostKeyProbeTimeout}, hostidentity.FailureTimeout},
		{&HostKeyProbeError{Kind: HostKeyProbeNegotiation, AlgorithmClass: "peer secret"}, hostidentity.FailureNegotiation},
		{&HostKeyProbeError{Kind: HostKeyProbeNoKey}, hostidentity.FailureNoKey},
		{errors.New("host banner and peer algorithm"), hostidentity.FailureUnavailable},
	} {
		if got := hostidentity.SafeFailure(mapHostIdentityFailure(tt.err)); got != tt.want {
			t.Fatalf("failure=%s want=%s", got, tt.want)
		}
	}
}
