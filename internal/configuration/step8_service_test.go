package configuration

import (
	"context"
	"errors"
	"testing"

	"bac-nexus/internal/profile"
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

	result := service.Run(context.Background(), Step8Request{RequestID: "request-1", Profile: serviceSavedProfile()})
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
	result := service.Run(context.Background(), Step8Request{RequestID: "request-2", Profile: serviceSavedProfile()})
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
	result := service.Run(context.Background(), Step8Request{RequestID: "request-4", Profile: serviceSavedProfile()})
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
	result := service.Run(context.Background(), Step8Request{RequestID: "request-5", Profile: serviceSavedProfile()})
	if result.Decision != DecisionTerminal || result.Class != ResultDowngradeBlocked || credentials != 0 {
		t.Fatalf("unknown result = %#v, credentials = %d", result, credentials)
	}
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
