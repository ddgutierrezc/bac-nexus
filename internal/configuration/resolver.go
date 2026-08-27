package configuration

import (
	"bac-nexus/internal/profile"
	"context"
	"errors"
	"fmt"
	"strings"
)

type Transport string

const (
	TransportUnknown Transport = "unknown"
	TransportWSS     Transport = "wss"
	TransportSSH     Transport = "ssh"
)

type FailureClass string

const (
	FailureNone          FailureClass = ""
	FailureAvailability  FailureClass = "availability"
	FailurePolicy        FailureClass = "policy"
	FailureUnsupported   FailureClass = "unsupported"
	FailureIdentity      FailureClass = "identity"
	FailureTrust         FailureClass = "trust"
	FailureCredentials   FailureClass = "credentials"
	FailureAuthorization FailureClass = "authorization"
	FailureProtocol      FailureClass = "protocol"
	FailureConsent       FailureClass = "consent"
)

type ResolveError struct{ Class FailureClass }

func (e *ResolveError) Error() string {
	switch e.Class {
	case FailureCredentials:
		return "credential failure"
	case FailureAuthorization:
		return "authorization failure"
	case FailureIdentity:
		return "identity failure"
	case FailureTrust:
		return "trust failure"
	case FailureConsent:
		return "fallback consent required"
	}
	return string(e.Class)
}
func failure(err error) FailureClass {
	var e *ResolveError
	if errors.As(err, &e) {
		return e.Class
	}
	return FailureProtocol
}

type ProfilePolicy struct{ DaemonAllowed, FallbackAllowed, FallbackConsent, SSHTrustValid bool }
type DaemonProbe interface {
	Probe(context.Context) (string, error)
}
type SSHFallback interface {
	Trust(context.Context) error
	Start(context.Context) error
}
type TransportAuditEvent struct{ Transport, Reason, Outcome, Protocol, PolicyID, TrustOutcome, Version string }
type TransportAuditor interface {
	RecordTransport(context.Context, TransportAuditEvent) error
}
type Resolution struct {
	Transport             Transport
	Version               string
	AuthenticationPending bool
	Class                 FailureClass
	FallbackReason        FailureClass
}
type Resolver struct {
	Daemon DaemonProbe
	SSH    SSHFallback
	Audit  TransportAuditor
}

// EnrollTrust accepts only an explicitly confirmed, transport-specific field.
// It returns evidence for persistence; observations never enter this API.
func EnrollTrust(transport Transport, mode profile.TrustMode, pin, provenance, confirmation string) (profile.TrustEvidence, error) {
	if confirmation != "enroll "+pin || provenance == "" {
		return profile.TrustEvidence{}, errors.New("trust enrollment confirmation required")
	}
	if transport == TransportWSS {
		if mode != profile.TrustModeCA && mode != profile.TrustModePin && mode != profile.TrustModeTOFU {
			return profile.TrustEvidence{}, errors.New("invalid TLS trust mode")
		}
	} else if transport == TransportSSH {
		if mode == profile.TrustModeCA {
			return profile.TrustEvidence{}, errors.New("SSH trust cannot use CA mode")
		}
	} else {
		return profile.TrustEvidence{}, errors.New("invalid trust transport")
	}
	if pin == "" && mode != profile.TrustModeCA {
		return profile.TrustEvidence{}, errors.New("pinned trust evidence is missing")
	}
	if mode == profile.TrustModeCA && pin != "" {
		return profile.TrustEvidence{}, errors.New("CA trust cannot contain a pin")
	}
	if transport == TransportWSS && pin != "" && (!strings.HasPrefix(pin, "sha256/") || len(pin) != 50) {
		return profile.TrustEvidence{}, errors.New("invalid TLS trust pin")
	}
	if transport == TransportSSH && pin != "" {
		if err := profile.ValidateHostKey(pin, profile.HostKeyTrustVerified); err != nil {
			return profile.TrustEvidence{}, errors.New("invalid SSH trust pin")
		}
	}
	if len(provenance) > 128 {
		return profile.TrustEvidence{}, fmt.Errorf("trust provenance is too long")
	}
	return profile.TrustEvidence{Mode: mode, Pin: pin, Provenance: provenance}, nil
}

func (r Resolver) Resolve(ctx context.Context, policy ProfilePolicy) (Resolution, error) {
	if err := ctx.Err(); err != nil {
		return Resolution{Class: FailureProtocol}, err
	}
	if policy.DaemonAllowed && r.Daemon != nil {
		version, err := r.Daemon.Probe(ctx)
		if err == nil && version == "2.3.5" {
			out := Resolution{Transport: TransportWSS, Version: version, AuthenticationPending: true}
			r.record(ctx, TransportAuditEvent{Transport: "wss", Outcome: "selected", Protocol: version, Version: version, PolicyID: "verified-readonly", TrustOutcome: "verified"})
			return out, nil
		}
		if err != nil && !eligible(failure(err)) {
			return r.failed(ctx, failure(err), "wss")
		}
		if err == nil {
			policyReason := FailureUnsupported
			if !policy.FallbackAllowed {
				return r.failed(ctx, policyReason, "wss")
			}
			return r.fallback(ctx, policy, policyReason)
		}
		if !policy.FallbackAllowed {
			return r.failed(ctx, failure(err), "wss")
		}
		return r.fallback(ctx, policy, failure(err))
	}
	if !policy.FallbackAllowed {
		return r.failed(ctx, FailurePolicy, "wss")
	}
	return r.fallback(ctx, policy, FailurePolicy)
}
func eligible(c FailureClass) bool {
	return c == FailureAvailability || c == FailurePolicy || c == FailureUnsupported
}
func (r Resolver) fallback(ctx context.Context, p ProfilePolicy, reason FailureClass) (Resolution, error) {
	if !p.SSHTrustValid {
		return r.failed(ctx, FailureTrust, "ssh")
	}
	if !p.FallbackConsent {
		return r.failed(ctx, FailureConsent, "ssh")
	}
	if r.SSH == nil {
		return r.failed(ctx, FailureAvailability, "ssh")
	}
	if err := r.SSH.Trust(ctx); err != nil {
		return r.failed(ctx, FailureTrust, "ssh")
	}
	if err := r.SSH.Start(ctx); err != nil {
		return r.failed(ctx, failure(err), "ssh")
	}
	out := Resolution{Transport: TransportSSH, AuthenticationPending: true, FallbackReason: reason}
	r.record(ctx, TransportAuditEvent{Transport: "ssh", Reason: string(reason), Outcome: "selected", PolicyID: "verified-readonly", TrustOutcome: "verified"})
	return out, nil
}
func (r Resolver) failed(ctx context.Context, c FailureClass, transport string) (Resolution, error) {
	r.record(ctx, TransportAuditEvent{Transport: transport, Outcome: "failed", Reason: string(c), PolicyID: "verified-readonly", TrustOutcome: "blocked"})
	return Resolution{Transport: Transport(transport), Class: c}, &ResolveError{Class: c}
}
func (r Resolver) record(ctx context.Context, e TransportAuditEvent) {
	if r.Audit != nil {
		_ = r.Audit.RecordTransport(ctx, e)
	}
}

func LocalReadiness(_ ProfilePolicy) ReadinessReport {
	return ReadinessReport{ProductStatus: ReadyForControlledIBMiValidation, ValidationStatus: NotValidatedOnIBMi, Transport: TransportUnknown, AuthenticationPending: true}
}
