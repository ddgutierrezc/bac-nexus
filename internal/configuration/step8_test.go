package configuration

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"bac-nexus/internal/credential"
	"bac-nexus/internal/profile"
)

func TestStep8DecisionReasonsAreExhaustiveAndFailClosed(t *testing.T) {
	tests := []struct {
		reason   Step8Reason
		decision Decision
	}{
		{ReasonWSSSelected, DecisionWSSSelected},
		{ReasonDaemonRefused, DecisionSSHEligible}, {ReasonDaemonUnavailable, DecisionSSHEligible},
		{ReasonDaemonAvailabilityTimeout, DecisionSSHEligible}, {ReasonDaemonPolicyDisabled, DecisionSSHEligible},
		{ReasonUnsupportedVersion, DecisionSSHEligible}, {ReasonIdentityFailure, DecisionTerminal},
		{ReasonProtocolFailure, DecisionTerminal}, {ReasonMalformedResponse, DecisionTerminal},
		{ReasonDowngradeBlocked, DecisionTerminal}, {ReasonCancelled, DecisionTerminal},
		{ReasonOperationTimeout, DecisionTerminal}, {ReasonLimitExceeded, DecisionTerminal},
	}
	for _, tt := range tests {
		t.Run(string(tt.reason), func(t *testing.T) {
			if got := DecisionForReason(tt.reason); got != tt.decision {
				t.Fatalf("decision=%q want %q", got, tt.decision)
			}
		})
	}
	if got := DecisionForReason(Step8Reason("unknown")); got != DecisionTerminal {
		t.Fatalf("unknown decision=%q", got)
	}
	if got := DecisionForReason(ReasonCredentialsUnavailable); got != DecisionTerminal {
		t.Fatalf("credential decision=%q", got)
	}
}

func TestStep8ResultClassesMapOnlyKnownValues(t *testing.T) {
	classes := []ResultClass{ResultIdentityFailure, ResultTrustMismatch, ResultProtocolFailure, ResultFramingFailure, ResultMalformedResponse, ResultDowngradeBlocked, ResultCredentialsUnavailable, ResultAuthenticationFailed, ResultAuthorizationDenied, ResultCancelled, ResultOperationTimeout, ResultProofTimeout, ResultCleanupTimeout, ResultCleanupFailure, ResultLimitExceeded, ResultConsentDeclined, ResultArtifactFailure, ResultJavaFailure, ResultUploadFailure, ResultLaunchFailure, ResultSessionFailure, ResultProofFailure}
	for _, class := range classes {
		if !IsTerminalResult(class) {
			t.Fatalf("%q is not terminal", class)
		}
	}
	for _, class := range []ResultClass{ResultTrustMismatch, ResultCredentialsUnavailable, ResultDowngradeBlocked} {
		if class == ResultIdentityFailure {
			t.Fatalf("distinct class collapsed: %q", class)
		}
	}
	if IsTerminalResult(ResultClass("unknown")) {
		t.Fatal("unknown result was accepted")
	}
}

func TestStep8SavedProfileAndCredentialContract(t *testing.T) {
	if err := ValidateStep8Profile(profile.Profile{Name: "draft"}); err == nil {
		t.Fatal("unsaved profile accepted")
	}
	for _, mode := range []profile.CredentialMode{profile.CredentialModePrompt, profile.CredentialModeKeyring} {
		if err := ValidateCredentialMode(mode); err != nil {
			t.Fatalf("mode %q rejected: %v", mode, err)
		}
	}
	if err := ValidateCredentialMode(profile.CredentialMode("vault")); err == nil {
		t.Fatal("legacy vault silently accepted")
	}
	key, err := credential.KeyForProfile("production")
	if err != nil || key != "ibmi/production" {
		t.Fatalf("key=%q err=%v", key, err)
	}
	if _, err := credential.KeyForProfile("bad/profile"); err == nil {
		t.Fatal("invalid profile key accepted")
	}
	p := profile.Profile{SchemaVersion: profile.SchemaVersionV2, Name: "saved", Host: "ibmi.example.test", Port: 22, Username: "USER", HostKeyFingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", HostKeyTrust: profile.HostKeyTrustVerified, CredentialMode: profile.CredentialModePrompt, EndpointPolicyRef: "policy"}
	if migrated, err := profile.MigrateToV3(p); err != nil || migrated.SchemaVersion != profile.SchemaVersionV3 {
		t.Fatalf("prompt migration=%#v err=%v", migrated, err)
	}
	p.CredentialMode = profile.CredentialModeVault
	if _, err := profile.MigrateToV3(p); err == nil {
		t.Fatal("vault mode silently migrated")
	}
}

func TestStep8CredentialFailuresAndProofMetadataFailClosed(t *testing.T) {
	for _, err := range []error{ErrPromptUnavailable, ErrPromptDenied, ErrKeyringUnavailable, ErrKeyringDenied, ErrCredentialNotFound, ErrInvalidCredentialMode} {
		if got := ClassifyCredentialFailure(err); got != ResultCredentialsUnavailable {
			t.Fatalf("%v => %q", err, got)
		}
	}
	if got := ClassifyCredentialFailure(errors.New("credentials_unavailable")); got != ResultClass("") {
		t.Fatalf("wrapped/string guess => %q", got)
	}
	if err := ValidateProofMetadata(ProofMetadata{Rows: 2, ProofRevision: "r"}); err == nil {
		t.Fatal("multiple proof rows accepted")
	}
	if err := ValidateProofMetadata(ProofMetadata{Rows: 1, ProofRevision: ProofRevision}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProofMetadata(ProofMetadata{Rows: 1, ProofRevision: "other"}); err == nil {
		t.Fatal("unknown proof revision accepted")
	}
}

func TestStep8MarkerIsBoundedAndInvalidated(t *testing.T) {
	marker := Marker{SchemaVersion: MarkerSchemaVersion, AtUnixMs: time.Now().UnixMilli(), Outcome: ResultProofSuccess, ProofRevision: ProofRevision}
	if err := ValidateMarker(marker); err != nil {
		t.Fatal(err)
	}
	if !MarkerValid(marker, ConfigUnchanged) {
		t.Fatal("fresh marker rejected")
	}
	for _, changed := range []ConfigChange{ConfigEndpointChanged, ConfigPolicyChanged, ConfigTrustChanged} {
		if MarkerValid(marker, changed) {
			t.Fatalf("marker valid after change: %v", changed)
		}
	}
	if MarkerIsReadiness(marker) {
		t.Fatal("marker became readiness evidence")
	}
}

func TestStep8ResultInvariantsFailClosed(t *testing.T) {
	if err := (Step8Result{RequestID: "req-1", Decision: DecisionWSSSelected, Class: ResultProofSuccess, ProofRevision: ProofRevision, Outcome: "validated"}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (Step8Result{Decision: DecisionSSHEligible, Class: ResultProofFailure}).Validate(); err == nil {
		t.Fatal("eligible terminal result accepted")
	}
	if err := (Step8Result{Decision: DecisionTerminal, Class: ResultClass("unknown")}).Validate(); err == nil {
		t.Fatal("unknown terminal result accepted")
	}
}

type gateFake struct {
	policyErr     error
	trustErr      error
	credentialErr error
	credential    []byte
	credentialKey string
	calls         []string
}

func (f *gateFake) AllowSSH(context.Context, profile.Profile) error {
	f.calls = append(f.calls, "policy")
	return f.policyErr
}

func (f *gateFake) VerifySSH(context.Context, profile.Profile) error {
	f.calls = append(f.calls, "trust")
	return f.trustErr
}

func (f *gateFake) Get(_ context.Context, key string, _ profile.CredentialMode) ([]byte, error) {
	f.calls = append(f.calls, "credential")
	f.credentialKey = key
	if f.credential == nil {
		f.credential = []byte("opaque")
	}
	return f.credential, f.credentialErr
}

func savedStep8Profile(t *testing.T) profile.Profile {
	t.Helper()
	p := profile.Profile{SchemaVersion: profile.SchemaVersionV2, Name: "saved", Host: "ibmi.example.test", Port: 22, Username: "USER", HostKeyFingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", HostKeyTrust: profile.HostKeyTrustVerified, CredentialMode: profile.CredentialModePrompt, EndpointPolicyRef: "policy"}
	p, err := profile.MigrateToV3(p)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPostObservationGateAppliesEligibleSSHInOrder(t *testing.T) {
	fake := &gateFake{}
	gate := PostObservationGate{Policy: fake, Trust: fake, Credentials: fake}
	result := gate.Apply(context.Background(), Step8Request{RequestID: "req-1", Profile: savedStep8Profile(t), Consent: true}, Observation{Decision: DecisionSSHEligible, Reason: ReasonDaemonUnavailable})
	if result.Decision != DecisionSSHEligible || result.Class != ResultProofSuccess {
		t.Fatalf("result=%+v", result)
	}
	if got, want := len(fake.calls), 3; got != want || fake.calls[0] != "policy" || fake.calls[1] != "trust" || fake.calls[2] != "credential" {
		t.Fatalf("calls=%v, want policy, trust, credential", fake.calls)
	}
	if fake.credentialKey != "ibmi/saved" {
		t.Fatalf("credential key=%q", fake.credentialKey)
	}
	for _, value := range fake.credential {
		if value != 0 {
			t.Fatalf("credential was not zeroed: %q", fake.credential)
		}
	}
}

func TestPostObservationGateBlocksBeforeCredentialAndFailsClosed(t *testing.T) {
	tests := []struct {
		name        string
		request     Step8Request
		observation Observation
		fake        gateFake
		want        ResultClass
		calls       []string
	}{
		{"invalid profile", Step8Request{RequestID: "req-1", Consent: true}, Observation{Decision: DecisionSSHEligible, Reason: ReasonDaemonUnavailable}, gateFake{}, ResultDowngradeBlocked, nil},
		{"terminal identity", Step8Request{RequestID: "req-1", Profile: savedStep8Profile(t), Consent: true}, Observation{Decision: DecisionTerminal, Reason: ReasonIdentityFailure}, gateFake{}, ResultIdentityFailure, nil},
		{"unknown observation", Step8Request{RequestID: "req-1", Profile: savedStep8Profile(t), Consent: true}, Observation{Decision: DecisionSSHEligible, Reason: Step8Reason("unknown")}, gateFake{}, ResultDowngradeBlocked, nil},
		{"policy denied", Step8Request{RequestID: "req-1", Profile: savedStep8Profile(t), Consent: true}, Observation{Decision: DecisionSSHEligible, Reason: ReasonDaemonUnavailable}, gateFake{policyErr: errors.New("denied")}, ResultAuthorizationDenied, []string{"policy"}},
		{"trust denied", Step8Request{RequestID: "req-1", Profile: savedStep8Profile(t), Consent: true}, Observation{Decision: DecisionSSHEligible, Reason: ReasonDaemonUnavailable}, gateFake{trustErr: errors.New("changed")}, ResultTrustMismatch, []string{"policy", "trust"}},
		{"consent absent", Step8Request{RequestID: "req-1", Profile: savedStep8Profile(t)}, Observation{Decision: DecisionSSHEligible, Reason: ReasonDaemonUnavailable}, gateFake{}, ResultConsentDeclined, []string{"policy", "trust"}},
		{"credential terminal", Step8Request{RequestID: "req-1", Profile: savedStep8Profile(t), Consent: true}, Observation{Decision: DecisionSSHEligible, Reason: ReasonDaemonUnavailable}, gateFake{credentialErr: ErrPromptDenied}, ResultCredentialsUnavailable, []string{"policy", "trust", "credential"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := tt.fake
			gate := PostObservationGate{Policy: &fake, Trust: &fake, Credentials: &fake}
			result := gate.Apply(context.Background(), tt.request, tt.observation)
			if result.Decision != DecisionTerminal || result.Class != tt.want {
				t.Fatalf("result=%+v", result)
			}
			if got := len(fake.calls); got != len(tt.calls) {
				t.Fatalf("calls=%v want=%v", fake.calls, tt.calls)
			}
			for i := range tt.calls {
				if fake.calls[i] != tt.calls[i] {
					t.Fatalf("calls=%v want=%v", fake.calls, tt.calls)
				}
			}
		})
	}
}

func TestTerminalResultForObservationMapsOnlyKnownTerminalReasons(t *testing.T) {
	tests := []struct {
		reason Step8Reason
		want   ResultClass
	}{
		{ReasonIdentityFailure, ResultIdentityFailure},
		{ReasonProtocolFailure, ResultProtocolFailure},
		{ReasonMalformedResponse, ResultMalformedResponse},
		{ReasonDowngradeBlocked, ResultDowngradeBlocked},
		{ReasonCancelled, ResultCancelled},
		{ReasonOperationTimeout, ResultOperationTimeout},
		{ReasonLimitExceeded, ResultLimitExceeded},
		{ReasonCredentialsUnavailable, ResultCredentialsUnavailable},
	}
	for _, tt := range tests {
		t.Run(string(tt.reason), func(t *testing.T) {
			if got := TerminalResultForObservation(tt.reason); got != tt.want {
				t.Fatalf("result=%q want=%q", got, tt.want)
			}
		})
	}
	if got := TerminalResultForObservation(Step8Reason("unknown")); got != ResultDowngradeBlocked {
		t.Fatalf("unknown result=%q", got)
	}
}

func TestPostObservationGateNeverDowngradesWSSOrTerminalObservations(t *testing.T) {
	tests := []struct {
		name        string
		observation Observation
		decision    Decision
		class       ResultClass
	}{
		{"wss selected", Observation{Decision: DecisionWSSSelected, Reason: ReasonWSSSelected}, DecisionWSSSelected, ResultProofSuccess},
		{"identity", Observation{Decision: DecisionTerminal, Reason: ReasonIdentityFailure}, DecisionTerminal, ResultIdentityFailure},
		{"protocol", Observation{Decision: DecisionTerminal, Reason: ReasonProtocolFailure}, DecisionTerminal, ResultProtocolFailure},
		{"malformed", Observation{Decision: DecisionTerminal, Reason: ReasonMalformedResponse}, DecisionTerminal, ResultMalformedResponse},
		{"downgrade", Observation{Decision: DecisionTerminal, Reason: ReasonDowngradeBlocked}, DecisionTerminal, ResultDowngradeBlocked},
		{"cancelled", Observation{Decision: DecisionTerminal, Reason: ReasonCancelled}, DecisionTerminal, ResultCancelled},
		{"timeout", Observation{Decision: DecisionTerminal, Reason: ReasonOperationTimeout}, DecisionTerminal, ResultOperationTimeout},
		{"limit", Observation{Decision: DecisionTerminal, Reason: ReasonLimitExceeded}, DecisionTerminal, ResultLimitExceeded},
		{"credentials", Observation{Decision: DecisionTerminal, Reason: ReasonCredentialsUnavailable}, DecisionTerminal, ResultCredentialsUnavailable},
		{"unknown", Observation{Decision: DecisionTerminal, Reason: Step8Reason("unknown")}, DecisionTerminal, ResultDowngradeBlocked},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &gateFake{}
			gate := PostObservationGate{Policy: fake, Trust: fake, Credentials: fake}
			result := gate.Apply(context.Background(), Step8Request{RequestID: "req-1", Profile: savedStep8Profile(t), Consent: true}, tt.observation)
			if result.Decision != tt.decision || result.Class != tt.class {
				t.Fatalf("result=%+v", result)
			}
			if len(fake.calls) != 0 {
				t.Fatalf("terminal/WSS observation reached fallback gates: %v", fake.calls)
			}
		})
	}
}

func TestPostObservationGateZeroesCredentialOnTerminalRetrievalFailure(t *testing.T) {
	fake := &gateFake{credential: []byte("opaque"), credentialErr: ErrKeyringDenied}
	gate := PostObservationGate{Policy: fake, Trust: fake, Credentials: fake}
	result := gate.Apply(context.Background(), Step8Request{RequestID: "req-1", Profile: savedStep8Profile(t), Consent: true}, Observation{Decision: DecisionSSHEligible, Reason: ReasonDaemonUnavailable})
	if result.Decision != DecisionTerminal || result.Class != ResultCredentialsUnavailable {
		t.Fatalf("result=%+v", result)
	}
	for _, value := range fake.credential {
		if value != 0 {
			t.Fatalf("credential was not zeroed: %q", fake.credential)
		}
	}
}

func TestStep8FallbackTicketsAreOpaqueBoundAndSingleUse(t *testing.T) {
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	tickets := newFallbackTicketStore(func() time.Time { return now })
	claim := fallbackTicketClaim{Profile: "saved", RequestID: "wss-1", Generation: 7, Class: ReasonDaemonUnavailable}
	ticket, err := tickets.issue(claim)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(ticket)
	if err != nil || len(raw) != 24 {
		t.Fatalf("ticket must be a 192-bit opaque value: len=%d err=%v", len(raw), err)
	}
	if err := tickets.consume(ticket, claim); err != nil {
		t.Fatalf("bound ticket was rejected: %v", err)
	}
	if err := tickets.consume(ticket, claim); err == nil {
		t.Fatal("replayed ticket was accepted")
	}
}

func TestStep8FallbackTicketRejectsForgedMismatchedExpiredAndSupersededClaims(t *testing.T) {
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	tickets := newFallbackTicketStore(func() time.Time { return now })
	claim := fallbackTicketClaim{Profile: "saved", RequestID: "wss-1", Generation: 7, Class: ReasonDaemonUnavailable}
	ticket, err := tickets.issue(claim)
	if err != nil {
		t.Fatal(err)
	}
	for _, rejected := range []struct {
		name   string
		ticket string
		claim  fallbackTicketClaim
	}{
		{"forged", "not-a-ticket", claim},
		{"profile mismatch", ticket, fallbackTicketClaim{Profile: "other", RequestID: "wss-1", Generation: 7, Class: ReasonDaemonUnavailable}},
		{"request mismatch", ticket, fallbackTicketClaim{Profile: "saved", RequestID: "wss-2", Generation: 7, Class: ReasonDaemonUnavailable}},
		{"generation mismatch", ticket, fallbackTicketClaim{Profile: "saved", RequestID: "wss-1", Generation: 8, Class: ReasonDaemonUnavailable}},
		{"class mismatch", ticket, fallbackTicketClaim{Profile: "saved", RequestID: "wss-1", Generation: 7, Class: ReasonDaemonPolicyDisabled}},
	} {
		t.Run(rejected.name, func(t *testing.T) {
			if err := tickets.consume(rejected.ticket, rejected.claim); err == nil {
				t.Fatal("mismatched ticket was accepted")
			}
		})
	}
	tickets.cancel(claim)
	if err := tickets.consume(ticket, claim); err == nil {
		t.Fatal("cancelled ticket was accepted")
	}
	supersededTickets := newFallbackTicketStore(func() time.Time { return now })
	supersededTicket, err := supersededTickets.issue(claim)
	if err != nil {
		t.Fatal(err)
	}
	supersededTickets.supersede("saved", 8)
	if err := supersededTickets.consume(supersededTicket, claim); err == nil {
		t.Fatal("superseded ticket was accepted")
	}

	expiringTickets := newFallbackTicketStore(func() time.Time { return now })
	expiringTicket, err := expiringTickets.issue(claim)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(5*time.Minute + time.Nanosecond)
	if err := expiringTickets.consume(expiringTicket, claim); err == nil {
		t.Fatal("expired ticket was accepted")
	}
}

func TestStep8FallbackTicketsAcceptExactlyFiveEligibleClasses(t *testing.T) {
	tickets := newFallbackTicketStore(time.Now)
	for _, reason := range []Step8Reason{ReasonDaemonRefused, ReasonDaemonUnavailable, ReasonDaemonAvailabilityTimeout, ReasonDaemonPolicyDisabled, ReasonUnsupportedVersion} {
		t.Run(string(reason), func(t *testing.T) {
			if _, err := tickets.issue(fallbackTicketClaim{Profile: "saved", RequestID: "wss-" + string(reason), Generation: 1, Class: reason}); err != nil {
				t.Fatalf("eligible class rejected: %v", err)
			}
		})
	}
	if _, err := tickets.issue(fallbackTicketClaim{Profile: "saved", RequestID: "wss-terminal", Generation: 1, Class: ReasonIdentityFailure}); err == nil {
		t.Fatal("terminal class issued a fallback ticket")
	}
}
