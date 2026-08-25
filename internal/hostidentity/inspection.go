// Package hostidentity defines the connector-neutral, no-auth SSH identity inspection boundary.
package hostidentity

import (
	"context"
	"errors"
)

var ErrInvalidCandidate = errors.New("invalid host identity candidate")

type Failure string

const (
	FailureCancelled        Failure = "cancelled"
	FailureTimeout          Failure = "timeout"
	FailureNegotiation      Failure = "negotiation"
	FailureNoKey            Failure = "no_key"
	FailureUnavailable      Failure = "unavailable"
	FailureInvalidCandidate Failure = "invalid_candidate"
)

// FailureError contains only a safe classification suitable for presentation.
type FailureError struct{ Failure Failure }

func (e *FailureError) Error() string { return string(e.Failure) }

func SafeFailure(err error) Failure {
	var failure *FailureError
	if errors.As(err, &failure) {
		return failure.Failure
	}
	return FailureUnavailable
}

// Candidate is the complete, unverified evidence observed during SSH key exchange.
type Candidate struct {
	Algorithm   string
	Fingerprint string
}

// Inspector deliberately accepts only an endpoint. It cannot receive credentials,
// profile state, authentication/session material, or persistence dependencies.
type Inspector interface {
	InspectHostKey(context.Context, string, int) (Candidate, error)
}
