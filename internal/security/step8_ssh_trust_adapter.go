package security

import (
	"context"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/profile"
)

// ObservedSSHFingerprintSource obtains one SSH host-key fingerprint using only
// the connection coordinates required for the observation.
type ObservedSSHFingerprintSource interface {
	ObserveSSHFingerprint(context.Context, string, int) (string, error)
}

// Step8SSHTrustAdapter compares a fresh SSH observation with saved SSH-only
// trust evidence. TLS evidence is deliberately never consulted.
type Step8SSHTrustAdapter struct {
	observer ObservedSSHFingerprintSource
}

var _ configuration.SSHTrust = Step8SSHTrustAdapter{}

func NewStep8SSHTrustAdapter(observer ObservedSSHFingerprintSource) Step8SSHTrustAdapter {
	return Step8SSHTrustAdapter{observer: observer}
}

func (a Step8SSHTrustAdapter) VerifySSH(ctx context.Context, p profile.Profile) error {
	if ctx.Err() != nil || a.observer == nil {
		return &SSHTrustError{Failure: SSHTrustUnavailable}
	}
	fingerprint, err := a.observer.ObserveSSHFingerprint(ctx, p.Host, p.Port)
	if err != nil || ctx.Err() != nil {
		return &SSHTrustError{Failure: SSHTrustUnavailable}
	}
	return (SSHTrust{ObservedFingerprint: fingerprint}).VerifySSH(ctx, p)
}
