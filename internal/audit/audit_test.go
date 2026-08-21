package audit

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestRecorderAcceptsAllowlistedEvent proves the recorder accepts a
// fully-allowlisted event without mutation and exposes it through the
// Events accessor.
func TestRecorderAcceptsAllowlistedEvent(t *testing.T) {
	recorder := NewRecorder()
	event := validAuditEvent()
	if err := recorder.Record(context.Background(), event); err != nil {
		t.Fatalf("Record error = %v", err)
	}
	events := recorder.Events()
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	if events[0] != event {
		t.Fatalf("recorded event mismatch: got %+v, want %+v", events[0], event)
	}
}

// TestRecorderRejectsDisallowedCapability proves that a capability not
// on the allowlist is rejected deterministically. The recorder must
// not store rejected events.
func TestRecorderRejectsDisallowedCapability(t *testing.T) {
	tests := []Capability{
		"rogue-write",
		"execute-sql",
		"run-shell",
		"ssh-exec",
		"",
		"catalog_resolve", // missing approved sentinel
		"Catalog_Resolve", // case-sensitive
	}
	for _, capability := range tests {
		t.Run(string(capability), func(t *testing.T) {
			recorder := NewRecorder()
			event := validAuditEvent()
			event.Capability = capability
			if err := recorder.Record(context.Background(), event); err == nil {
				t.Fatal("expected rejection for non-allowlisted capability")
			}
			if len(recorder.Events()) != 0 {
				t.Fatalf("rejected event was stored: %+v", recorder.Events())
			}
		})
	}
}

// TestRecorderRejectsUnknownConnectorClassification proves that the
// connector classification is restricted to the approved enum.
func TestRecorderRejectsUnknownConnectorClassification(t *testing.T) {
	tests := []Connector{"postgres", "sqlserver", "sap", "", "IBMI"}
	for _, connector := range tests {
		t.Run(string(connector), func(t *testing.T) {
			recorder := NewRecorder()
			event := validAuditEvent()
			event.Connector = connector
			if err := recorder.Record(context.Background(), event); err == nil {
				t.Fatal("expected rejection for non-allowlisted connector")
			}
		})
	}
}

// TestRecorderRejectsUnknownTargetClass proves the target class is
// restricted to the approved enum.
func TestRecorderRejectsUnknownTargetClass(t *testing.T) {
	tests := []TargetClass{"oracle", "sap", "sharepoint", "", "IBMI_CATALOG"}
	for _, target := range tests {
		t.Run(string(target), func(t *testing.T) {
			recorder := NewRecorder()
			event := validAuditEvent()
			event.TargetClass = target
			if err := recorder.Record(context.Background(), event); err == nil {
				t.Fatal("expected rejection for non-allowlisted target class")
			}
		})
	}
}

// TestRecorderRejectsUnknownResultClass proves the result class is
// restricted to allow/deny/error.
func TestRecorderRejectsUnknownResultClass(t *testing.T) {
	tests := []ResultClass{"", "approved", "rejected", "maybe", "success", "ALLOW"}
	for _, result := range tests {
		t.Run(string(result), func(t *testing.T) {
			recorder := NewRecorder()
			event := validAuditEvent()
			event.Result = result
			if err := recorder.Record(context.Background(), event); err == nil {
				t.Fatal("expected rejection for non-allowlisted result class")
			}
		})
	}
}

// TestValidateEventRejectsSensitiveReasonSubstrings proves the audit
// allowlist excludes credential, source, token, digest, cursor, path,
// command, SQL, model content, clientInfo, and raw error material
// from any reason field.
func TestValidateEventRejectsSensitiveReasonSubstrings(t *testing.T) {
	sensitive := []string{
		"credential leaked",
		"secret text",
		"token exposed",
		"digest 0x4242",
		"cursor abcdef",
		"path /home/nexus",
		"host ibmi.example",
		"user NEXUS$USER",
		"command issued",
		"sql executed",
		"model content",
		"clientInfo name",
		"raw error: connection refused",
		"line content",
		"source text",
		"stderr trace",
	}
	for _, content := range sensitive {
		t.Run(content, func(t *testing.T) {
			event := validAuditEvent()
			event.Reason = content
			if err := ValidateEvent(event); err == nil {
				t.Fatalf("expected rejection for sensitive reason %q", content)
			}
		})
	}
}

// TestValidateEventAcceptsBoundedReason covers reasons that are short,
// non-sensitive, and consistent with the allowlist vocabulary.
func TestValidateEventAcceptsBoundedReason(t *testing.T) {
	reasons := []string{
		"ok",
		"deny: selector not allowlisted",
		"deny: target class mismatch",
		"deny: unauthorized selector",
		"error: credentials_unavailable",
		"error: host_key_changed",
		"deny: missing evidence",
		"deny: capability denied",
	}
	for _, reason := range reasons {
		t.Run(reason, func(t *testing.T) {
			event := validAuditEvent()
			event.Reason = reason
			if err := ValidateEvent(event); err != nil {
				t.Fatalf("ValidateEvent(%q) error = %v", reason, err)
			}
		})
	}
}

// TestValidateEventRejectsNegativeCounts proves count fields are
// bounded to non-negative integers. A negative count would not
// represent a real line/page count and must fail closed.
func TestValidateEventRejectsNegativeCounts(t *testing.T) {
	event := validAuditEvent()
	event.Requested = -1
	if err := ValidateEvent(event); err == nil {
		t.Fatal("expected rejection for negative Requested")
	}
	event = validAuditEvent()
	event.Returned = -1
	if err := ValidateEvent(event); err == nil {
		t.Fatal("expected rejection for negative Returned")
	}
}

// TestValidateEventRejectsOversizedReason proves the reason field has
// a deterministic upper bound. Unbounded reasons would risk leaking
// sensitive content and inflate audit volume.
func TestValidateEventRejectsOversizedReason(t *testing.T) {
	event := validAuditEvent()
	event.Reason = strings.Repeat("a", 257)
	if err := ValidateEvent(event); err == nil {
		t.Fatal("expected rejection for oversized reason")
	}
}

// TestValidateEventRejectsZeroTimestamp proves the timestamp must be
// non-zero so the audit record cannot be silently emitted without
// timing context.
func TestValidateEventRejectsZeroTimestamp(t *testing.T) {
	event := validAuditEvent()
	event.Timestamp = time.Time{}
	if err := ValidateEvent(event); err == nil {
		t.Fatal("expected rejection for zero timestamp")
	}
}

// TestValidateEventRejectsNegativeDuration proves the duration must
// be non-negative. A negative duration cannot represent a real
// elapsed time.
func TestValidateEventRejectsNegativeDuration(t *testing.T) {
	event := validAuditEvent()
	event.Duration = -1 * time.Second
	if err := ValidateEvent(event); err == nil {
		t.Fatal("expected rejection for negative duration")
	}
}

// TestRecorderRespectsCancelledContext proves the recorder respects
// context cancellation deterministically.
func TestRecorderRespectsCancelledContext(t *testing.T) {
	recorder := NewRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := recorder.Record(ctx, validAuditEvent()); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if len(recorder.Events()) != 0 {
		t.Fatal("cancelled recording appended an event")
	}
}

// TestNoopAuditorAcceptsEventUnderValidContext proves the noop auditor
// discards events silently under a valid context.
func TestNoopAuditorAcceptsEventUnderValidContext(t *testing.T) {
	if err := (Noop{}).Record(context.Background(), validAuditEvent()); err != nil {
		t.Fatalf("Record error = %v", err)
	}
}

// TestNoopAuditorRejectsCancelledContext proves the noop auditor
// still respects context cancellation.
func TestNoopAuditorRejectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (Noop{}).Record(ctx, validAuditEvent()); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

// TestRecorderReturnsIndependentEventsSlice proves the recorder
// returns a defensive copy from Events so callers cannot mutate
// stored events after the fact.
func TestRecorderReturnsIndependentEventsSlice(t *testing.T) {
	recorder := NewRecorder()
	if err := recorder.Record(context.Background(), validAuditEvent()); err != nil {
		t.Fatalf("Record error = %v", err)
	}
	first := recorder.Events()
	first[0].Result = "tampered"
	second := recorder.Events()
	if second[0].Result == "tampered" {
		t.Fatal("Events() returned a mutable reference to stored events")
	}
}

// TestRecorderAccumulatesAllValidEvents proves the recorder stores
// every accepted event in submission order.
func TestRecorderAccumulatesAllValidEvents(t *testing.T) {
	recorder := NewRecorder()
	for index := 0; index < 5; index++ {
		event := validAuditEvent()
		event.Requested = index
		if err := recorder.Record(context.Background(), event); err != nil {
			t.Fatalf("Record(%d) error = %v", index, err)
		}
	}
	events := recorder.Events()
	if len(events) != 5 {
		t.Fatalf("event count = %d, want 5", len(events))
	}
	for index, event := range events {
		if event.Requested != index {
			t.Fatalf("event[%d].Requested = %d, want %d", index, event.Requested, index)
		}
	}
}

// TestAuditPackageHasNoRemotePathOrShellSurface is a structural
// reflection test: the public surface of the audit package must
// never expose generic remote, path, shell, SQL, or SSH operations.
func TestAuditPackageHasNoRemotePathOrShellSurface(t *testing.T) {
	checks := []struct {
		typ   reflect.Type
		label string
	}{
		{typ: reflect.TypeOf((*Recorder)(nil)), label: "Recorder"},
		{typ: reflect.TypeOf((*Noop)(nil)), label: "Noop"},
		{typ: reflect.TypeOf((*Auditor)(nil)).Elem(), label: "Auditor"},
	}
	for _, check := range checks {
		for i := 0; i < check.typ.NumMethod(); i++ {
			name := check.typ.Method(i).Name
			lower := strings.ToLower(name)
			for _, forbidden := range []string{"ssh", "exec", "shell", "path", "command", "sql", "dial", "connect", "remote", "clientinfo", "parent"} {
				if strings.Contains(lower, forbidden) {
					t.Fatalf("%s has forbidden method %q (matched %q)", check.label, name, forbidden)
				}
			}
		}
	}
}

func validAuditEvent() Event {
	return Event{
		Capability:  CapabilityCatalogResolve,
		Connector:   ConnectorIBMi,
		TargetClass: TargetClassIBMiCatalog,
		PolicyID:    "verified-readonly",
		Result:      ResultClassAllow,
		Requested:   50,
		Returned:    7,
		Timestamp:   time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC),
		Duration:    250 * time.Millisecond,
		Reason:      "ok",
	}
}
