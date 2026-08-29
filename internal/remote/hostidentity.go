package remote

import (
	"context"
	"errors"
	"strings"

	"bac-nexus/internal/hostidentity"
	"bac-nexus/internal/profile"
)

// HostIdentityInspector adapts the no-auth SSH probe to the connector-neutral UI boundary.
type HostIdentityInspector struct{}

func (HostIdentityInspector) InspectHostKey(ctx context.Context, host string, port int) (hostidentity.Candidate, error) {
	observation, err := InspectHostKey(ctx, host, port)
	if err != nil {
		return hostidentity.Candidate{}, mapHostIdentityFailure(err)
	}
	return mapHostIdentityObservation(observation)
}

func mapHostIdentityObservation(observation HostKeyObservation) (hostidentity.Candidate, error) {
	if observation.Verified || observation.TrustCandidate != profile.HostKeyTrustTOFU || strings.TrimSpace(observation.Algorithm) == "" || profile.ValidateHostKey(observation.Fingerprint, profile.HostKeyTrustTOFU) != nil {
		return hostidentity.Candidate{}, &hostidentity.FailureError{Failure: hostidentity.FailureInvalidCandidate}
	}
	return hostidentity.Candidate{Algorithm: observation.Algorithm, Fingerprint: observation.Fingerprint}, nil
}

func mapHostIdentityFailure(err error) error {
	if errors.Is(err, context.Canceled) {
		return &hostidentity.FailureError{Failure: hostidentity.FailureCancelled}
	}
	var probe *HostKeyProbeError
	if errors.As(err, &probe) {
		switch probe.Kind {
		case HostKeyProbeTimeout:
			return &hostidentity.FailureError{Failure: hostidentity.FailureTimeout}
		case HostKeyProbeNegotiation:
			return &hostidentity.FailureError{Failure: hostidentity.FailureNegotiation}
		default:
			return &hostidentity.FailureError{Failure: hostidentity.FailureNoKey}
		}
	}
	return &hostidentity.FailureError{Failure: hostidentity.FailureUnavailable}
}
