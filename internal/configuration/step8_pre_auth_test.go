package configuration

import (
	"context"
	"errors"
	"testing"
	"time"

	"bac-nexus/internal/profile"
)

func TestManagedStep8PreAuthMapsDaemonOutcomes(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		err      error
		decision Decision
		reason   Step8Reason
	}{
		{name: "supported version selects WSS", version: daemonVersion, decision: DecisionWSSSelected, reason: ReasonWSSSelected},
		{name: "unsupported version is SSH eligible", version: "2.3.4", decision: DecisionSSHEligible, reason: ReasonUnsupportedVersion},
		{name: "daemon unavailable is SSH eligible", err: &ResolveError{Class: FailureAvailability}, decision: DecisionSSHEligible, reason: ReasonDaemonUnavailable},
		{name: "daemon policy disabled is SSH eligible", err: &ResolveError{Class: FailurePolicy}, decision: DecisionSSHEligible, reason: ReasonDaemonPolicyDisabled},
		{name: "identity failure is terminal", err: &ResolveError{Class: FailureIdentity}, decision: DecisionTerminal, reason: ReasonIdentityFailure},
		{name: "TLS trust failure is terminal", err: &ResolveError{Class: FailureTrust}, decision: DecisionTerminal, reason: ReasonIdentityFailure},
		{name: "protocol failure is terminal", err: &ResolveError{Class: FailureProtocol}, decision: DecisionTerminal, reason: ReasonProtocolFailure},
		{name: "pre-auth credential failure is terminal", err: &ResolveError{Class: FailureCredentials}, decision: DecisionTerminal, reason: ReasonCredentialsUnavailable},
		{name: "unknown failure blocks downgrade", err: errors.New("unknown"), decision: DecisionTerminal, reason: ReasonDowngradeBlocked},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := serviceSavedProfile()
			p.TLSTrust = profile.TrustEvidence{Mode: profile.TrustModeCA}
			var gotHost string
			var gotPort int
			var gotTrust *profile.TrustEvidence
			adapter := ManagedStep8PreAuth{newProbe: func(host string, port int, trust *profile.TrustEvidence) (DaemonProbe, error) {
				gotHost, gotPort, gotTrust = host, port, trust
				return step8DaemonProbe{version: tt.version, err: tt.err}, nil
			}}

			got := adapter.Observe(context.Background(), p)
			if got != (Observation{Decision: tt.decision, Reason: tt.reason}) {
				t.Fatalf("observation = %#v, want decision %q and reason %q", got, tt.decision, tt.reason)
			}
			if gotHost != p.Host || gotPort != managedDaemonPort || gotTrust == nil || *gotTrust != p.TLSTrust {
				t.Fatalf("probe inputs = (%q, %d, %#v), want (%q, %d, profile TLS trust)", gotHost, gotPort, gotTrust, p.Host, managedDaemonPort)
			}
		})
	}
}

func TestManagedStep8PreAuthFailsClosedBeforeProbe(t *testing.T) {
	tests := []struct {
		name   string
		ctx    context.Context
		p      profile.Profile
		reason Step8Reason
	}{
		{name: "cancelled context", ctx: cancelledStep8Context(), p: serviceSavedProfile(), reason: ReasonCancelled},
		{name: "expired context", ctx: expiredStep8Context(), p: serviceSavedProfile(), reason: ReasonOperationTimeout},
		{name: "invalid saved profile", ctx: context.Background(), reason: ReasonDowngradeBlocked},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			adapter := ManagedStep8PreAuth{newProbe: func(string, int, *profile.TrustEvidence) (DaemonProbe, error) {
				calls++
				return step8DaemonProbe{version: daemonVersion}, nil
			}}

			got := adapter.Observe(tt.ctx, tt.p)
			want := Observation{Decision: DecisionTerminal, Reason: tt.reason}
			if got != want || calls != 0 {
				t.Fatalf("observation = %#v, probe calls = %d; want %#v and zero calls", got, calls, want)
			}
		})
	}
}

type step8DaemonProbe struct {
	version string
	err     error
}

func (p step8DaemonProbe) Probe(context.Context) (string, error) { return p.version, p.err }

func cancelledStep8Context() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func expiredStep8Context() context.Context {
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancel()
	return ctx
}
