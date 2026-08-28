package security

import (
	"context"
	"crypto/subtle"

	"bac-nexus/internal/profile"
)

// SSHTrustFailure is a bounded reason for a failed independent SSH trust check.
type SSHTrustFailure string

const (
	SSHTrustMissing     SSHTrustFailure = "missing"
	SSHTrustMismatch    SSHTrustFailure = "mismatch"
	SSHTrustUnapproved  SSHTrustFailure = "unapproved"
	SSHTrustUnavailable SSHTrustFailure = "unavailable"
)

// SSHTrustError intentionally exposes no host, fingerprint, or policy detail.
type SSHTrustError struct {
	Failure SSHTrustFailure
}

func (e *SSHTrustError) Error() string { return "ssh_trust_blocked" }

// SSHTrust verifies an observed SSH fingerprint solely against SSH enrollment.
// It has no enrollment, persistence, TLS, or remote-acquisition capability.
type SSHTrust struct {
	ObservedFingerprint string
}

func (t SSHTrust) VerifySSH(ctx context.Context, p profile.Profile) error {
	if ctx.Err() != nil {
		return &SSHTrustError{Failure: SSHTrustUnavailable}
	}
	evidence := p.SSHTrust
	if evidence.Mode == "" && evidence.Pin == "" {
		return &SSHTrustError{Failure: SSHTrustMissing}
	}
	if evidence.Mode != profile.TrustModeTOFU && evidence.Mode != profile.TrustModePin {
		return &SSHTrustError{Failure: SSHTrustUnapproved}
	}
	if t.ObservedFingerprint == "" || evidence.Pin == "" {
		return &SSHTrustError{Failure: SSHTrustMissing}
	}
	if evidence.Provenance == "" || len(evidence.Provenance) > 128 || profile.ValidateHostKey(evidence.Pin, profile.HostKeyTrustVerified) != nil || profile.ValidateHostKey(t.ObservedFingerprint, profile.HostKeyTrustVerified) != nil {
		return &SSHTrustError{Failure: SSHTrustUnapproved}
	}
	if subtle.ConstantTimeCompare([]byte(evidence.Pin), []byte(t.ObservedFingerprint)) != 1 {
		return &SSHTrustError{Failure: SSHTrustMismatch}
	}
	return nil
}
