package security

import (
	"context"
	"errors"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/profile"
)

const approvedStep8PolicyID = "verified-readonly"

// ErrSSHPolicyDenied is intentionally free of profile, endpoint, and policy details.
var ErrSSHPolicyDenied = errors.New("ssh_fallback_policy_denied")

// Step8SSHPolicy authorizes only the approved saved-profile SSH fallback policy.
// SSH identity verification remains an independent later gate.
type Step8SSHPolicy struct{}

var _ configuration.SSHPolicy = Step8SSHPolicy{}

func NewStep8SSHPolicy() Step8SSHPolicy { return Step8SSHPolicy{} }

func (Step8SSHPolicy) AllowSSH(ctx context.Context, p profile.Profile) error {
	if ctx.Err() != nil || configuration.ValidateStep8Profile(p) != nil || !p.FallbackAllowed || p.EndpointPolicyRef != approvedStep8PolicyID {
		return ErrSSHPolicyDenied
	}
	return nil
}
