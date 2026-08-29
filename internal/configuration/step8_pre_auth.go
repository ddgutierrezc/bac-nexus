package configuration

import (
	"context"
	"errors"

	"bac-nexus/internal/profile"
)

const managedDaemonPort = 8076

type managedDaemonProbeFactory func(string, int, *profile.TrustEvidence) (DaemonProbe, error)

// ManagedStep8PreAuth adapts the managed daemon version probe to the
// credential-free observation consumed by Step8Service.
type ManagedStep8PreAuth struct {
	newProbe managedDaemonProbeFactory
}

// NewManagedStep8PreAuth composes the release-owned managed daemon probe with
// the saved profile's independent TLS evidence.
func NewManagedStep8PreAuth() ManagedStep8PreAuth {
	return ManagedStep8PreAuth{
		newProbe: func(host string, port int, trust *profile.TrustEvidence) (DaemonProbe, error) {
			return NewManagedDaemonProbe(host, port, trust)
		},
	}
}

func (a ManagedStep8PreAuth) Observe(ctx context.Context, p profile.Profile) Observation {
	if errors.Is(ctx.Err(), context.Canceled) {
		return observationForReason(ReasonCancelled)
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return observationForReason(ReasonOperationTimeout)
	}
	if ValidateStep8Profile(p) != nil || a.newProbe == nil {
		return observationForReason(ReasonDowngradeBlocked)
	}

	probe, err := a.newProbe(p.Host, managedDaemonPort, &p.TLSTrust)
	if err != nil || probe == nil {
		return observationForDaemonError(ctx, err)
	}
	version, err := probe.Probe(ctx)
	if err != nil {
		return observationForDaemonError(ctx, err)
	}
	if version != daemonVersion {
		return observationForReason(ReasonUnsupportedVersion)
	}
	return observationForReason(ReasonWSSSelected)
}

func observationForDaemonError(ctx context.Context, err error) Observation {
	if errors.Is(ctx.Err(), context.Canceled) {
		return observationForReason(ReasonCancelled)
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return observationForReason(ReasonOperationTimeout)
	}
	var resolveError *ResolveError
	if errors.As(err, &resolveError) {
		switch resolveError.Class {
		case FailureAvailability:
			return observationForReason(ReasonDaemonUnavailable)
		case FailurePolicy:
			return observationForReason(ReasonDaemonPolicyDisabled)
		case FailureUnsupported:
			return observationForReason(ReasonUnsupportedVersion)
		case FailureIdentity, FailureTrust:
			return observationForReason(ReasonIdentityFailure)
		case FailureProtocol:
			return observationForReason(ReasonProtocolFailure)
		case FailureCredentials:
			return observationForReason(ReasonCredentialsUnavailable)
		}
	}
	return observationForReason(ReasonDowngradeBlocked)
}

func observationForReason(reason Step8Reason) Observation {
	return Observation{Decision: DecisionForReason(reason), Reason: reason}
}
