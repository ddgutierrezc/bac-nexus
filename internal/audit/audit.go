// Package audit records structured, allowlisted outcomes for v1 MCP
// operations. Every event contains only the approved classification
// and count metadata: capability, connector classification, target
// class, policy identifier, timestamp, duration, requested/returned
// counts, result classification, and a bounded non-sensitive reason.
// Source text, hash, cursor, coordinate, path, host, user, command,
// SQL, credential reference, model content, clientInfo, and raw
// error material are never accepted.
package audit

import (
	"context"
	"errors"
	"strings"
	"time"

	"bac-nexus/internal/configuration"
)

// Errors returned by the audit boundary. Each error is a deterministic
// classification; the message never contains source, path, host, user,
// credential, or raw error material.
var (
	ErrCapabilityRejected = errors.New("audit: capability not allowlisted")
	ErrConnectorRejected  = errors.New("audit: connector not allowlisted")
	ErrTargetRejected     = errors.New("audit: target class not allowlisted")
	ErrPolicyRejected     = errors.New("audit: policy identifier not allowlisted")
	ErrResultRejected     = errors.New("audit: result class not allowlisted")
	ErrReasonRejected     = errors.New("audit: reason not allowlisted")
	ErrCountRejected      = errors.New("audit: count fields must be non-negative")
	ErrDurationRejected   = errors.New("audit: duration must be non-negative")
	ErrTimestampRejected  = errors.New("audit: timestamp is required")
	ErrReasonOversized    = errors.New("audit: reason exceeds bounded length")
)

// Capability is the allowlisted operation class. Adding a capability
// requires an explicit decision and a matching red test.
type Capability string

const (
	CapabilityCatalogResolve          Capability = "catalog_resolve"
	CapabilitySourceRead              Capability = "source_read"
	CapabilityLifecycleCompletion     Capability = "lifecycle_completion"
	CapabilityConfigurationDiagnostic Capability = "configuration_diagnostic"
)

// Connector is the allowlisted connector classification. The audit
// package does not recognize per-host or per-product connectors.
type Connector string

const (
	ConnectorIBMi Connector = "ibmi"
)

// TargetClass is the allowlisted target-system classification. It
// identifies the system class, never a host or path.
type TargetClass string

const (
	TargetClassIBMiCatalog             TargetClass = "ibmi_catalog"
	TargetClassIBMiSource              TargetClass = "ibmi_source"
	TargetClassLifecycle               TargetClass = "lifecycle"
	TargetClassConfigurationDiagnostic TargetClass = "configuration_diagnostic"
)

// PolicyID identifies the specific allowlisted client-policy
// identifier. It is a short, pre-registered identifier; it is never a
// raw path, host, user, or clientInfo value.
type PolicyID string

const PolicyIDVerifiedReadOnly PolicyID = "verified-readonly"

// ResultClass is the deterministic outcome classification. The
// allowlist is closed: only allow/deny/error are accepted.
type ResultClass string

const (
	ResultClassAllow     ResultClass = "allow"
	ResultClassDeny      ResultClass = "deny"
	ResultClassError     ResultClass = "error"
	ResultClassSucceeded ResultClass = "succeeded"
	ResultClassCancelled ResultClass = "cancelled"
	ResultClassTimedOut  ResultClass = "timed_out"
	ResultClassFailed    ResultClass = "failed"
)

// Event is the allowlisted structured audit outcome. Every field is
// bounded and non-sensitive by construction. The package never copies
// untrusted content into Event fields; the caller is responsible for
// supplying only the approved metadata.
type Event struct {
	Capability  Capability
	Connector   Connector
	TargetClass TargetClass
	PolicyID    PolicyID
	Result      ResultClass
	Requested   int
	Returned    int
	Timestamp   time.Time
	Duration    time.Duration
	Reason      string
}

// TransportEvent is a bounded, metadata-only dual-transport outcome.
type TransportEvent struct{ Transport, Reason, Outcome, Protocol, PolicyID, TrustOutcome, Version string }

func ValidateTransportEvent(e TransportEvent) error {
	if e.Transport != "wss" && e.Transport != "ssh" {
		return ErrConnectorRejected
	}
	if e.Outcome != "selected" && e.Outcome != "failed" {
		return ErrResultRejected
	}
	if e.Reason != "" && e.Reason != "availability" && e.Reason != "policy" && e.Reason != "unsupported" && e.Reason != "identity" && e.Reason != "trust" && e.Reason != "credentials" && e.Reason != "authorization" && e.Reason != "protocol" && e.Reason != "consent" {
		return ErrReasonRejected
	}
	if len(e.Protocol) > 32 {
		return ErrReasonOversized
	}
	if e.PolicyID != "" && e.PolicyID != "verified-readonly" {
		return ErrPolicyRejected
	}
	if e.TrustOutcome != "" && e.TrustOutcome != "verified" && e.TrustOutcome != "untrusted" && e.TrustOutcome != "blocked" {
		return ErrReasonRejected
	}
	if len(e.Version) > 64 {
		return ErrReasonOversized
	}
	return nil
}
func (r *Recorder) RecordTransport(ctx context.Context, e TransportEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateTransportEvent(e); err != nil {
		return err
	}
	r.transportEvents = append(r.transportEvents, e)
	return nil
}

// maxReasonBytes caps the reason length so a malformed caller cannot
// inflate audit volume or smuggle content past the allowlist
// substring filter.
const maxReasonBytes = 256

// Auditor is the consumer-owned audit boundary. Implementations must
// not echo error material from the supplied event.
type Auditor interface {
	Record(ctx context.Context, event Event) error
}

// NewPersistentDiagnosticAuditor maps configuration diagnostics to the durable
// audit's fixed, metadata-only schema. It has no external-client mutation seam.
func NewPersistentDiagnosticAuditor(sink Auditor, now func() time.Time) configuration.DiagnosticAuditor {
	if now == nil {
		now = time.Now
	}
	return persistentDiagnosticAuditor{sink: sink, now: now}
}

type persistentDiagnosticAuditor struct {
	sink Auditor
	now  func() time.Time
}

func (a persistentDiagnosticAuditor) Record(ctx context.Context, event configuration.DiagnosticAuditEvent) error {
	if a.sink == nil {
		return ErrAuditUnavailable
	}
	result, ok := diagnosticAuditResult(event.Classification)
	if !ok {
		return ErrAuditUnavailable
	}
	return a.sink.Record(ctx, Event{
		Capability:  CapabilityConfigurationDiagnostic,
		Connector:   ConnectorIBMi,
		TargetClass: TargetClassConfigurationDiagnostic,
		PolicyID:    PolicyIDVerifiedReadOnly,
		Result:      result,
		Timestamp:   a.now().UTC(),
		Duration:    event.Duration,
	})
}

func diagnosticAuditResult(classification configuration.DiagnosticClassification) (ResultClass, bool) {
	switch classification {
	case configuration.DiagnosticSucceeded:
		return ResultClassSucceeded, true
	case configuration.DiagnosticCancelled:
		return ResultClassCancelled, true
	case configuration.DiagnosticTimedOut:
		return ResultClassTimedOut, true
	case configuration.DiagnosticFailed:
		return ResultClassFailed, true
	default:
		return "", false
	}
}

// Recorder is an in-memory test/dev Auditor. It stores every accepted
// event in submission order and exposes a defensive copy.
type Recorder struct {
	events          []Event
	transportEvents []TransportEvent
}

// NewRecorder returns an empty Recorder.
func NewRecorder() *Recorder { return &Recorder{} }

// Record validates the event against the allowlist and stores it
// when valid. Validation errors never echo the offending value.
func (r *Recorder) Record(ctx context.Context, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateEvent(event); err != nil {
		return err
	}
	r.events = append(r.events, event)
	return nil
}

// Events returns a defensive copy of the stored events. Callers may
// not mutate the stored slice.
func (r *Recorder) Events() []Event {
	out := make([]Event, len(r.events))
	copy(out, r.events)
	return out
}

func (r *Recorder) TransportEvents() []TransportEvent {
	out := make([]TransportEvent, len(r.transportEvents))
	copy(out, r.transportEvents)
	return out
}

// Noop is an Auditor that discards every accepted event. It still
// rejects invalid events and respects context cancellation.
type Noop struct{}

// Record validates the event and discards it.
func (Noop) Record(ctx context.Context, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return ValidateEvent(event)
}

// ValidateEvent enforces the allowlist. Each violation returns a
// deterministic classification; the offending value is never
// reflected in the error message.
func ValidateEvent(event Event) error {
	switch event.Capability {
	case CapabilityCatalogResolve, CapabilitySourceRead, CapabilityLifecycleCompletion:
		switch event.Result {
		case ResultClassAllow, ResultClassDeny, ResultClassError:
		default:
			return ErrResultRejected
		}
	case CapabilityConfigurationDiagnostic:
		if event.TargetClass != TargetClassConfigurationDiagnostic {
			return ErrTargetRejected
		}
		switch event.Result {
		case ResultClassSucceeded, ResultClassCancelled, ResultClassTimedOut, ResultClassFailed:
		default:
			return ErrResultRejected
		}
	default:
		return ErrCapabilityRejected
	}
	if event.Connector != ConnectorIBMi {
		return ErrConnectorRejected
	}
	if event.PolicyID != PolicyIDVerifiedReadOnly {
		return ErrPolicyRejected
	}
	switch event.TargetClass {
	case TargetClassIBMiCatalog, TargetClassIBMiSource, TargetClassLifecycle, TargetClassConfigurationDiagnostic:
	default:
		return ErrTargetRejected
	}
	if event.Requested < 0 || event.Returned < 0 {
		return ErrCountRejected
	}
	if event.Duration < 0 {
		return ErrDurationRejected
	}
	if event.Timestamp.IsZero() {
		return ErrTimestampRejected
	}
	if err := validateReason(event.Reason); err != nil {
		return err
	}
	return nil
}

// validateReason enforces the bounded length and the substring
// allowlist. The forbidden substrings cover every category of
// sensitive material that must never appear in a reason field.
func validateReason(reason string) error {
	if len(reason) > maxReasonBytes {
		return ErrReasonOversized
	}
	lower := strings.ToLower(reason)
	for _, forbidden := range forbiddenReasonSubstrings {
		if strings.Contains(lower, forbidden) {
			return ErrReasonRejected
		}
	}
	return nil
}

// forbiddenReasonSubstrings lists every category of sensitive
// material that must never appear in a reason field. The list is the
// authoritative boundary; adding an entry requires an explicit
// decision and a matching red test.
var forbiddenReasonSubstrings = []string{
	"credential",
	"secret",
	"token",
	"digest",
	"cursor",
	"path",
	"host",
	"user",
	"command",
	"sql",
	"model",
	"clientinfo",
	"client_info",
	"stderr",
	"trace",
	"line content",
	"source text",
	"raw error",
	"connection refused",
	"connection reset",
}
