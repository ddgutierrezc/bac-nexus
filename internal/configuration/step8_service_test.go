package configuration

import (
	"context"
	"errors"
	"testing"
	"time"

	"bac-nexus/internal/connectors/ibmi/mapepirestdio"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
)

func TestStep8ServiceRunsWSSProofAfterPreAuthAndCleansUp(t *testing.T) {
	secret := []byte("opaque")
	trace := []string{}
	markers := &step8Markers{trace: &trace}
	service := Step8Service{
		Observe: step8ObserveFunc(func(context.Context, profile.Profile) Observation {
			trace = append(trace, "observe")
			return Observation{Decision: DecisionWSSSelected, Reason: ReasonWSSSelected}
		}),
		Credentials: step8CredentialsFunc(func(context.Context, string, profile.CredentialMode) ([]byte, error) {
			trace = append(trace, "credential")
			return secret, nil
		}),
		WSS: step8WSSFunc(func(context.Context, profile.Profile) (Step8WSSSession, error) {
			trace = append(trace, "open")
			return &step8Session{trace: &trace}, nil
		}),
		Markers: markers,
		Audit: step8AuditFunc(func(context.Context, Step8AuditEvent) error {
			for _, b := range secret {
				if b != 0 {
					t.Fatal("audit observed credential before release")
				}
			}
			trace = append(trace, "audit")
			return nil
		}),
	}

	result := service.Run(context.Background(), Step8Request{RequestID: "request-1", Profile: serviceSavedProfile(), WSSConsent: true})
	if result.Decision != DecisionWSSSelected || result.Class != ResultProofSuccess || !result.Cleanup || result.ProofRevision != ProofRevision {
		t.Fatalf("unexpected result: %#v", result)
	}
	if got, want := joinTrace(trace), "clear,observe,credential,open,prove,close,audit,marker"; got != want {
		t.Fatalf("order = %q, want %q", got, want)
	}
	for _, b := range secret {
		if b != 0 {
			t.Fatal("credential was not zeroed after cleanup")
		}
	}
	if err := ValidateMarker(markers.marker); err != nil {
		t.Fatalf("marker is invalid: %v", err)
	}
}

func TestStep8ServiceTerminalAndHistoricalMarkerNeverRetrieveCredential(t *testing.T) {
	credentials := 0
	trace := []string{}
	service := Step8Service{
		Observe: step8ObserveFunc(func(context.Context, profile.Profile) Observation {
			trace = append(trace, "observe")
			return Observation{Decision: DecisionTerminal, Reason: ReasonIdentityFailure}
		}),
		Credentials: step8CredentialsFunc(func(context.Context, string, profile.CredentialMode) ([]byte, error) {
			credentials++
			return []byte("secret"), nil
		}),
		Markers: &step8Markers{trace: &trace},
	}
	result := service.Run(context.Background(), Step8Request{RequestID: "request-2", Profile: serviceSavedProfile(), WSSConsent: true})
	if result.Decision != DecisionTerminal || result.Class != ResultIdentityFailure || credentials != 0 {
		t.Fatalf("terminal result = %#v, credentials = %d", result, credentials)
	}
	if got, want := joinTrace(trace), "clear,observe"; got != want {
		t.Fatalf("terminal order = %q, want %q", got, want)
	}
}

func TestStep8ServiceProofFailureClosesAndDoesNotWriteMarker(t *testing.T) {
	secret := []byte("opaque")
	trace := []string{}
	service := Step8Service{
		Observe: step8ObserveFunc(func(context.Context, profile.Profile) Observation {
			return Observation{Decision: DecisionWSSSelected, Reason: ReasonWSSSelected}
		}),
		Credentials: step8CredentialsFunc(func(context.Context, string, profile.CredentialMode) ([]byte, error) { return secret, nil }),
		WSS: step8WSSFunc(func(context.Context, profile.Profile) (Step8WSSSession, error) {
			return &step8Session{trace: &trace, err: errors.New("proof failed")}, nil
		}),
		Markers: &step8Markers{trace: &trace},
		Audit:   step8AuditFunc(func(context.Context, Step8AuditEvent) error { trace = append(trace, "audit"); return nil }),
	}
	result := service.Run(context.Background(), Step8Request{RequestID: "request-4", Profile: serviceSavedProfile(), WSSConsent: true})
	if result.Decision != DecisionTerminal || result.Class != ResultProofFailure {
		t.Fatalf("proof failure = %#v", result)
	}
	if got, want := joinTrace(trace), "clear,prove,close,audit"; got != want {
		t.Fatalf("failure order = %q, want %q", got, want)
	}
	for _, b := range secret {
		if b != 0 {
			t.Fatal("credential was not zeroed")
		}
	}
}

func TestStep8ServiceRejectsInvalidProfileWithoutObservation(t *testing.T) {
	called := false
	service := Step8Service{Observe: step8ObserveFunc(func(context.Context, profile.Profile) Observation { called = true; return Observation{} })}
	result := service.Run(context.Background(), Step8Request{RequestID: "request-3"})
	if result.Class != ResultDowngradeBlocked || called {
		t.Fatalf("invalid result = %#v, observe called = %t", result, called)
	}
}

func TestStep8ServiceUnknownObservationFailsClosedBeforeCredential(t *testing.T) {
	credentials := 0
	service := Step8Service{
		Observe: step8ObserveFunc(func(context.Context, profile.Profile) Observation {
			return Observation{Decision: Decision("unknown"), Reason: Step8Reason("unknown")}
		}),
		Credentials: step8CredentialsFunc(func(context.Context, string, profile.CredentialMode) ([]byte, error) {
			credentials++
			return []byte("secret"), nil
		}),
	}
	result := service.Run(context.Background(), Step8Request{RequestID: "request-5", Profile: serviceSavedProfile(), WSSConsent: true})
	if result.Decision != DecisionTerminal || result.Class != ResultDowngradeBlocked || credentials != 0 {
		t.Fatalf("unknown result = %#v, credentials = %d", result, credentials)
	}
}

func TestStep8ServiceFallsBackForExactlyFiveEligibleReasonsWithGateCredential(t *testing.T) {
	eligible := []Step8Reason{
		ReasonDaemonRefused,
		ReasonDaemonUnavailable,
		ReasonDaemonAvailabilityTimeout,
		ReasonDaemonPolicyDisabled,
		ReasonUnsupportedVersion,
	}
	for _, reason := range eligible {
		t.Run(string(reason), func(t *testing.T) {
			secret := []byte("opaque")
			gate := &gateFake{credential: secret}
			client := &serviceSSHClient{}
			factory := &serviceSSHFactory{runtime: &SSHRuntime{client: client}}
			markers := &step8Markers{}
			audit := Step8AuditEvent{}
			service := Step8Service{
				Observe: step8ObserveFunc(func(context.Context, profile.Profile) Observation {
					return Observation{Decision: DecisionSSHEligible, Reason: reason}
				}),
				Gate:    PostObservationGate{Policy: gate, Trust: gate, Credentials: gate},
				SSH:     factory,
				Markers: markers,
				Audit: step8AuditFunc(func(_ context.Context, event Step8AuditEvent) error {
					audit = event
					return nil
				}),
				NowUnixMs: func() int64 {
					return 1
				},
				Tickets: newFallbackTicketStore(time.Now),
			}

			wss := service.Run(context.Background(), Step8Request{RequestID: "request-ssh", Generation: 1, Profile: serviceSavedProfile(), WSSConsent: true})
			result := service.RunSSH(context.Background(), Step8Request{RequestID: "request-ssh", Generation: 1, Profile: serviceSavedProfile(), FallbackTicket: wss.FallbackTicket, FallbackClass: wss.FallbackClass, SSHConsent: true})
			if result.Decision != DecisionSSHEligible || result.Class != ResultProofSuccess || !result.Cleanup {
				t.Fatalf("result=%+v", result)
			}
			if got, want := joinTrace(gate.calls), "policy,trust,credential"; got != want {
				t.Fatalf("gate order=%q want=%q", got, want)
			}
			if factory.calls != 1 || client.proofs != 1 || client.closes != 1 {
				t.Fatalf("runtime calls open=%d proof=%d close=%d", factory.calls, client.proofs, client.closes)
			}
			if factory.secret != client.secret || factory.secret != &secret[0] {
				t.Fatal("SSH dial and proof did not receive the gate credential reference")
			}
			for _, b := range secret {
				if b != 0 {
					t.Fatal("credential was not zeroed after runtime cleanup")
				}
			}
			if err := ValidateMarker(markers.marker); err != nil {
				t.Fatalf("marker=%+v err=%v", markers.marker, err)
			}
			if audit.Transport != TransportSSH || audit.Class != ResultProofSuccess || !audit.Cleanup {
				t.Fatalf("audit=%+v", audit)
			}
		})
	}
}

func TestStep8ServiceFallbackKeepsPrimaryFailureAndSuppressesMarker(t *testing.T) {
	secret := []byte("opaque")
	gate := &gateFake{credential: secret}
	client := &serviceSSHClient{proofErr: errors.New("proof"), closeErr: errors.New("cleanup")}
	factory := &serviceSSHFactory{runtime: &SSHRuntime{client: client}}
	markers := &step8Markers{}
	audit := Step8AuditEvent{}
	service := Step8Service{
		Observe: step8ObserveFunc(func(context.Context, profile.Profile) Observation {
			return Observation{Decision: DecisionSSHEligible, Reason: ReasonDaemonUnavailable}
		}),
		Gate:    PostObservationGate{Policy: gate, Trust: gate, Credentials: gate},
		SSH:     factory,
		Markers: markers,
		Audit: step8AuditFunc(func(_ context.Context, event Step8AuditEvent) error {
			audit = event
			return nil
		}),
		Tickets: newFallbackTicketStore(time.Now),
	}

	wss := service.Run(context.Background(), Step8Request{RequestID: "request-cleanup", Generation: 1, Profile: serviceSavedProfile(), WSSConsent: true})
	result := service.RunSSH(context.Background(), Step8Request{RequestID: "request-cleanup", Generation: 1, Profile: serviceSavedProfile(), FallbackTicket: wss.FallbackTicket, FallbackClass: wss.FallbackClass, SSHConsent: true})
	if result.Class != ResultProofFailure || result.Cleanup {
		t.Fatalf("result=%+v", result)
	}
	if client.closes != 1 || markers.marker != (Marker{}) {
		t.Fatalf("closes=%d marker=%+v", client.closes, markers.marker)
	}
	if audit.Transport != TransportSSH || audit.Class != ResultProofFailure || audit.Cleanup {
		t.Fatalf("audit=%+v", audit)
	}
	for _, b := range secret {
		if b != 0 {
			t.Fatal("credential was not zeroed after failed cleanup")
		}
	}
}

func TestStep8ServiceSSHAuditFailureDoesNotReplaceSuccessfulProof(t *testing.T) {
	gate := &gateFake{credential: []byte("opaque")}
	markers := &step8Markers{}
	service := Step8Service{
		Observe: step8ObserveFunc(func(context.Context, profile.Profile) Observation {
			return Observation{Decision: DecisionSSHEligible, Reason: ReasonDaemonUnavailable}
		}),
		Gate:    PostObservationGate{Policy: gate, Trust: gate, Credentials: gate},
		SSH:     &serviceSSHFactory{runtime: &SSHRuntime{client: &serviceSSHClient{}}},
		Markers: markers,
		Audit: step8AuditFunc(func(context.Context, Step8AuditEvent) error {
			return errors.New("audit unavailable")
		}),
		Tickets: newFallbackTicketStore(time.Now),
	}
	wss := service.Run(context.Background(), Step8Request{RequestID: "request-audit", Generation: 1, Profile: serviceSavedProfile(), WSSConsent: true})
	result := service.RunSSH(context.Background(), Step8Request{RequestID: "request-audit", Generation: 1, Profile: serviceSavedProfile(), FallbackTicket: wss.FallbackTicket, FallbackClass: wss.FallbackClass, SSHConsent: true})
	if result.Decision != DecisionSSHEligible || result.Class != ResultProofSuccess || result.ProofRevision != ProofRevision || !result.Cleanup {
		t.Fatalf("audit failure replaced successful proof: %+v", result)
	}
	if err := ValidateMarker(markers.marker); err != nil {
		t.Fatalf("successful proof did not write marker: %+v, %v", markers.marker, err)
	}
}

func TestStep8ServiceWSSAuditFailureDoesNotReplaceSuccessfulProof(t *testing.T) {
	trace := []string{}
	markers := &step8Markers{}
	service := Step8Service{
		Observe: step8ObserveFunc(func(context.Context, profile.Profile) Observation {
			return Observation{Decision: DecisionWSSSelected, Reason: ReasonWSSSelected}
		}),
		Credentials: step8CredentialsFunc(func(context.Context, string, profile.CredentialMode) ([]byte, error) {
			return []byte("opaque"), nil
		}),
		WSS: step8WSSFunc(func(context.Context, profile.Profile) (Step8WSSSession, error) {
			return &step8Session{trace: &trace}, nil
		}),
		Markers: markers,
		Audit: step8AuditFunc(func(context.Context, Step8AuditEvent) error {
			return errors.New("audit unavailable")
		}),
	}
	result := service.Run(context.Background(), Step8Request{RequestID: "wss-audit", Profile: serviceSavedProfile(), WSSConsent: true})
	if result.Decision != DecisionWSSSelected || result.Class != ResultProofSuccess || result.ProofRevision != ProofRevision || !result.Cleanup {
		t.Fatalf("audit failure replaced successful proof: %+v", result)
	}
	if err := ValidateMarker(markers.marker); err != nil {
		t.Fatalf("successful proof did not write marker: %+v, %v", markers.marker, err)
	}
}

func TestStep8ServiceNeverFallsBackForWSSOrTerminalObservations(t *testing.T) {
	tests := []Observation{
		{Decision: DecisionWSSSelected, Reason: ReasonWSSSelected},
		{Decision: DecisionTerminal, Reason: ReasonIdentityFailure},
		{Decision: DecisionTerminal, Reason: ReasonProtocolFailure},
		{Decision: DecisionTerminal, Reason: ReasonMalformedResponse},
		{Decision: DecisionTerminal, Reason: ReasonDowngradeBlocked},
		{Decision: DecisionTerminal, Reason: ReasonCancelled},
		{Decision: DecisionTerminal, Reason: ReasonOperationTimeout},
		{Decision: DecisionTerminal, Reason: ReasonLimitExceeded},
		{Decision: DecisionTerminal, Reason: ReasonCredentialsUnavailable},
		{Decision: DecisionTerminal, Reason: Step8Reason("proof_timeout")},
		{Decision: DecisionTerminal, Reason: Step8Reason("cleanup_timeout")},
		{Decision: DecisionTerminal, Reason: Step8Reason("cleanup_failure")},
		{Decision: DecisionTerminal, Reason: Step8Reason("authentication_failed")},
		{Decision: DecisionTerminal, Reason: Step8Reason("authorization_denied")},
		{Decision: DecisionTerminal, Reason: Step8Reason("framing_failure")},
		{Decision: DecisionTerminal, Reason: Step8Reason("unknown")},
	}
	for _, observation := range tests {
		t.Run(string(observation.Reason), func(t *testing.T) {
			gate := &gateFake{}
			factory := &serviceSSHFactory{}
			service := Step8Service{
				Observe: step8ObserveFunc(func(context.Context, profile.Profile) Observation { return observation }),
				Gate:    PostObservationGate{Policy: gate, Trust: gate, Credentials: gate},
				SSH:     factory,
			}
			if observation.Decision == DecisionWSSSelected {
				service.Credentials = step8CredentialsFunc(func(context.Context, string, profile.CredentialMode) ([]byte, error) { return []byte("opaque"), nil })
				trace := []string{}
				service.WSS = step8WSSFunc(func(context.Context, profile.Profile) (Step8WSSSession, error) {
					return &step8Session{trace: &trace}, nil
				})
			}
			_ = service.Run(context.Background(), Step8Request{RequestID: "request-terminal", Profile: serviceSavedProfile()})
			if factory.calls != 0 || len(gate.calls) != 0 {
				t.Fatalf("fallback calls runtime=%d gates=%v", factory.calls, gate.calls)
			}
		})
	}
}

func TestStep8ServiceRequiresExplicitWSSConsentBeforeTrustCredentialsOrNetwork(t *testing.T) {
	observeCalls, credentialCalls, wssCalls := 0, 0, 0
	service := Step8Service{
		Observe: step8ObserveFunc(func(context.Context, profile.Profile) Observation {
			observeCalls++
			return Observation{Decision: DecisionWSSSelected, Reason: ReasonWSSSelected}
		}),
		Credentials: step8CredentialsFunc(func(context.Context, string, profile.CredentialMode) ([]byte, error) {
			credentialCalls++
			return []byte("opaque"), nil
		}),
		WSS: step8WSSFunc(func(context.Context, profile.Profile) (Step8WSSSession, error) {
			wssCalls++
			return &step8Session{}, nil
		}),
	}
	result := service.Run(context.Background(), Step8Request{RequestID: "wss-no-consent", Generation: 1, Profile: serviceSavedProfile()})
	if result.Class != ResultConsentDeclined || observeCalls != 0 || credentialCalls != 0 || wssCalls != 0 {
		t.Fatalf("result=%+v observe=%d credential=%d wss=%d", result, observeCalls, credentialCalls, wssCalls)
	}
}

func TestStep8ServiceIssuesTicketThenRequiresSeparateSSHConsent(t *testing.T) {
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	client := &serviceSSHClient{}
	factory := &serviceSSHFactory{runtime: &SSHRuntime{client: client}}
	gate := &gateFake{credential: []byte("opaque")}
	service := Step8Service{
		Observe: step8ObserveFunc(func(context.Context, profile.Profile) Observation {
			return Observation{Decision: DecisionSSHEligible, Reason: ReasonDaemonUnavailable}
		}),
		Gate:    PostObservationGate{Policy: gate, Trust: gate, Credentials: gate},
		SSH:     factory,
		Tickets: newFallbackTicketStore(func() time.Time { return now }),
	}
	wss := service.Run(context.Background(), Step8Request{RequestID: "wss-1", Generation: 1, Profile: serviceSavedProfile(), WSSConsent: true})
	if wss.Decision != DecisionSSHEligible || wss.FallbackTicket == "" || factory.calls != 0 {
		t.Fatalf("wss=%+v ssh calls=%d", wss, factory.calls)
	}
	rejected := service.RunSSH(context.Background(), Step8Request{RequestID: "wss-1", Generation: 1, Profile: serviceSavedProfile(), FallbackTicket: wss.FallbackTicket, FallbackClass: wss.FallbackClass})
	if rejected.Class != ResultConsentDeclined || factory.calls != 0 {
		t.Fatalf("rejected=%+v ssh calls=%d", rejected, factory.calls)
	}
	accepted := service.RunSSH(context.Background(), Step8Request{RequestID: "wss-1", Generation: 1, Profile: serviceSavedProfile(), FallbackTicket: wss.FallbackTicket, FallbackClass: wss.FallbackClass, SSHConsent: true})
	if accepted.Class != ResultProofSuccess || factory.calls != 1 || client.proofs != 1 {
		t.Fatalf("accepted=%+v ssh calls=%d proofs=%d", accepted, factory.calls, client.proofs)
	}
}

func TestStep8ServiceRejectedTicketAdmissionsHaveZeroSSHEffects(t *testing.T) {
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	claim := fallbackTicketClaim{Profile: "profile-1", RequestID: "wss-1", Generation: 1, Class: ReasonDaemonUnavailable}
	for _, tt := range []struct {
		name    string
		prepare func(*fallbackTicketStore, string)
		request func(string) Step8Request
	}{
		{"forgery", func(*fallbackTicketStore, string) {}, func(string) Step8Request {
			return Step8Request{RequestID: "wss-1", Generation: 1, Profile: serviceSavedProfile(), FallbackTicket: "forged", FallbackClass: ReasonDaemonUnavailable, SSHConsent: true}
		}},
		{"mismatch", func(*fallbackTicketStore, string) {}, func(ticket string) Step8Request {
			return Step8Request{RequestID: "wss-1", Generation: 2, Profile: serviceSavedProfile(), FallbackTicket: ticket, FallbackClass: ReasonDaemonUnavailable, SSHConsent: true}
		}},
		{"replay", func(tickets *fallbackTicketStore, ticket string) { _ = tickets.consume(ticket, claim) }, func(ticket string) Step8Request {
			return Step8Request{RequestID: "wss-1", Generation: 1, Profile: serviceSavedProfile(), FallbackTicket: ticket, FallbackClass: ReasonDaemonUnavailable, SSHConsent: true}
		}},
		{"cancelled", func(tickets *fallbackTicketStore, _ string) { tickets.cancel(claim) }, func(ticket string) Step8Request {
			return Step8Request{RequestID: "wss-1", Generation: 1, Profile: serviceSavedProfile(), FallbackTicket: ticket, FallbackClass: ReasonDaemonUnavailable, SSHConsent: true}
		}},
		{"superseded", func(tickets *fallbackTicketStore, _ string) { tickets.supersede("profile-1", 2) }, func(ticket string) Step8Request {
			return Step8Request{RequestID: "wss-1", Generation: 1, Profile: serviceSavedProfile(), FallbackTicket: ticket, FallbackClass: ReasonDaemonUnavailable, SSHConsent: true}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tickets := newFallbackTicketStore(func() time.Time { return now })
			ticket, err := tickets.issue(claim)
			if err != nil {
				t.Fatal(err)
			}
			tt.prepare(tickets, ticket)
			gate := &gateFake{credential: []byte("opaque")}
			client := &serviceSSHClient{}
			factory := &serviceSSHFactory{runtime: &SSHRuntime{client: client}}
			service := Step8Service{Gate: PostObservationGate{Policy: gate, Trust: gate, Credentials: gate}, SSH: factory, Tickets: tickets}
			result := service.RunSSH(context.Background(), tt.request(ticket))
			if result.Class != ResultDowngradeBlocked || factory.calls != 0 || client.proofs != 0 || len(gate.calls) != 0 {
				t.Fatalf("result=%+v factory=%d proofs=%d gates=%v", result, factory.calls, client.proofs, gate.calls)
			}
		})
	}
}

type serviceSSHFactory struct {
	runtime *SSHRuntime
	calls   int
	secret  *byte
}

func (f *serviceSSHFactory) Open(_ context.Context, _ Step8Result, _ profile.Profile, secret []byte) (*SSHRuntime, Step8Result) {
	f.calls++
	if len(secret) > 0 {
		f.secret = &secret[0]
	}
	if f.runtime == nil {
		return nil, terminalGateResult(Step8Result{RequestID: "request-ssh"}, ResultSessionFailure)
	}
	return f.runtime, Step8Result{RequestID: "request-ssh", Decision: DecisionSSHEligible, Class: ResultProofSuccess}
}

type serviceSSHClient struct {
	proofs   int
	closes   int
	secret   *byte
	proofErr error
	closeErr error
}

func (f *serviceSSHClient) Close() error { f.closes++; return f.closeErr }
func (*serviceSSHClient) EnsureMapepireServerJAR(context.Context, string) (mapepirestdio.VerifiedMapepireArtifactReceipt, error) {
	return mapepirestdio.VerifiedMapepireArtifactReceipt{}, nil
}
func (f *serviceSSHClient) FixedMapepireProof(_ context.Context, _ mapepirestdio.VerifiedMapepireArtifactReceipt, _ string, secret []byte) (remote.FixedProofMetadata, error) {
	f.proofs++
	if len(secret) > 0 {
		f.secret = &secret[0]
	}
	if f.proofErr != nil {
		return remote.FixedProofMetadata{}, f.proofErr
	}
	return remote.FixedProofMetadata{Rows: 1, Revision: ProofRevision}, nil
}

type step8ObserveFunc func(context.Context, profile.Profile) Observation

func (f step8ObserveFunc) Observe(ctx context.Context, p profile.Profile) Observation {
	return f(ctx, p)
}

type step8CredentialsFunc func(context.Context, string, profile.CredentialMode) ([]byte, error)

func (f step8CredentialsFunc) Get(ctx context.Context, key string, mode profile.CredentialMode) ([]byte, error) {
	return f(ctx, key, mode)
}

type step8WSSFunc func(context.Context, profile.Profile) (Step8WSSSession, error)

func (f step8WSSFunc) Open(ctx context.Context, p profile.Profile) (Step8WSSSession, error) {
	return f(ctx, p)
}

type step8AuditFunc func(context.Context, Step8AuditEvent) error

func (f step8AuditFunc) Record(ctx context.Context, event Step8AuditEvent) error {
	return f(ctx, event)
}

type step8Session struct {
	trace *[]string
	err   error
}

func (s *step8Session) Prove(context.Context, string, []byte) (ProofMetadata, error) {
	*s.trace = append(*s.trace, "prove")
	return ProofMetadata{Rows: 1, ProofRevision: ProofRevision}, s.err
}
func (s *step8Session) Close() error { *s.trace = append(*s.trace, "close"); return nil }

type step8Markers struct {
	trace  *[]string
	marker Marker
}

func (m *step8Markers) Clear(context.Context, profile.Profile) error {
	if m.trace != nil {
		*m.trace = append(*m.trace, "clear")
	}
	return nil
}
func (m *step8Markers) Write(_ context.Context, _ profile.Profile, marker Marker) error {
	m.marker = marker
	if m.trace != nil {
		*m.trace = append(*m.trace, "marker")
	}
	return nil
}
func joinTrace(trace []string) string {
	result := ""
	for i, item := range trace {
		if i > 0 {
			result += ","
		}
		result += item
	}
	return result
}
func serviceSavedProfile() profile.Profile {
	return profile.Profile{SchemaVersion: profile.SchemaVersionV3, Name: "profile-1", Host: "host.example", Port: 22, Username: "user", CredentialMode: profile.CredentialModePrompt}
}
