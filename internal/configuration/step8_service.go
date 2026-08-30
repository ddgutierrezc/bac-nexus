package configuration

import (
	"context"
	"time"

	"bac-nexus/internal/profile"
)

// Step8PreAuth observes policy-owned WSS compatibility without credentials.
type Step8PreAuth interface {
	Observe(context.Context, profile.Profile) Observation
}

// Step8WSSFactory opens a trusted WSS session after a WSS selection.
type Step8WSSFactory interface {
	Open(context.Context, profile.Profile) (Step8WSSSession, error)
}

// Step8WSSSession exposes only the existing authenticated fixed-proof flow.
type Step8WSSSession interface {
	Prove(context.Context, string, []byte) (ProofMetadata, error)
	Close() error
}

// Step8MarkerStore owns only bounded historical marker persistence.
type Step8MarkerStore interface {
	Clear(context.Context, profile.Profile) error
	Write(context.Context, profile.Profile, Marker) error
}

// Step8AuditEvent intentionally contains no endpoint, secret, query, row, or error data.
type Step8AuditEvent struct {
	Transport Transport
	Class     ResultClass
	Revision  string
	Cleanup   bool
}

// Step8Auditor records only the service's allowlisted proof outcome.
type Step8Auditor interface {
	Record(context.Context, Step8AuditEvent) error
}

// Step8SSHRuntimeFactory starts only the already-admitted fixed SSH runtime.
type Step8SSHRuntimeFactory interface {
	Open(context.Context, Step8Result, profile.Profile, []byte) (*SSHRuntime, Step8Result)
}

// Step8Service owns the saved-profile WSS proof path. SSH eligibility is a
// typed continuation for the next slice and never starts fallback here.
type Step8Service struct {
	Observe     Step8PreAuth
	Credentials CredentialProvider
	WSS         Step8WSSFactory
	Gate        PostObservationGate
	SSH         Step8SSHRuntimeFactory
	Markers     Step8MarkerStore
	Audit       Step8Auditor
	NowUnixMs   func() int64
	Tickets     *fallbackTicketStore
}

func (s Step8Service) Run(ctx context.Context, request Step8Request) (result Step8Result) {
	result = Step8Result{RequestID: request.RequestID}
	if request.RequestID == "" || ValidateStep8Profile(request.Profile) != nil {
		return s.finish(ctx, request.Profile, terminalGateResult(result, ResultDowngradeBlocked))
	}
	if err := ctx.Err(); err != nil {
		return s.finish(ctx, request.Profile, terminalGateResult(result, ResultCancelled))
	}
	if !request.WSSConsent {
		return s.finish(ctx, request.Profile, terminalGateResult(result, ResultConsentDeclined))
	}
	if s.Markers != nil {
		if err := s.Markers.Clear(ctx, request.Profile); err != nil {
			return s.finish(ctx, request.Profile, terminalGateResult(result, ResultCleanupFailure))
		}
	}
	if s.Observe == nil {
		return s.finish(ctx, request.Profile, terminalGateResult(result, ResultDowngradeBlocked))
	}
	observation := s.Observe.Observe(ctx, request.Profile)
	if observation.Decision != DecisionForReason(observation.Reason) {
		return s.finish(ctx, request.Profile, terminalGateResult(result, ResultDowngradeBlocked))
	}
	switch observation.Decision {
	case DecisionTerminal:
		return s.finish(ctx, request.Profile, terminalGateResult(result, TerminalResultForObservation(observation.Reason)))
	case DecisionSSHEligible:
		return s.offerSSH(ctx, request, observation.Reason)
	case DecisionWSSSelected:
		return s.runWSS(ctx, request)
	default:
		return s.finish(ctx, request.Profile, terminalGateResult(result, ResultDowngradeBlocked))
	}
}

func (s Step8Service) offerSSH(ctx context.Context, request Step8Request, reason Step8Reason) Step8Result {
	result := Step8Result{RequestID: request.RequestID, Decision: DecisionSSHEligible, Class: ResultProofSuccess, FallbackClass: reason}
	if s.Tickets == nil {
		return s.finish(ctx, request.Profile, terminalGateResult(result, ResultDowngradeBlocked))
	}
	ticket, err := s.Tickets.issue(fallbackTicketClaim{Profile: request.Profile.Name, RequestID: request.RequestID, Generation: request.Generation, Class: reason})
	if err != nil {
		return s.finish(ctx, request.Profile, terminalGateResult(result, ResultDowngradeBlocked))
	}
	result.FallbackTicket = ticket
	return s.finish(ctx, request.Profile, result)
}

// RunSSH requires a separately consented, atomically consumed ticket. It never
// runs WSS observation and cannot infer SSH consent from a WSS request.
func (s Step8Service) RunSSH(ctx context.Context, request Step8Request) Step8Result {
	result := Step8Result{RequestID: request.RequestID}
	if request.RequestID == "" || ValidateStep8Profile(request.Profile) != nil || !request.SSHConsent {
		return s.finishTransport(ctx, request.Profile, TransportSSH, terminalGateResult(result, ResultConsentDeclined))
	}
	if err := ctx.Err(); err != nil {
		return s.finishTransport(ctx, request.Profile, TransportSSH, terminalGateResult(result, ResultCancelled))
	}
	claim := fallbackTicketClaim{Profile: request.Profile.Name, RequestID: request.RequestID, Generation: request.Generation, Class: request.FallbackClass}
	if s.Tickets == nil || s.Tickets.consume(request.FallbackTicket, claim) != nil {
		return s.finishTransport(ctx, request.Profile, TransportSSH, terminalGateResult(result, ResultDowngradeBlocked))
	}
	request.Consent = true
	return s.runSSH(ctx, request, Observation{Decision: DecisionSSHEligible, Reason: request.FallbackClass})
}

func (s Step8Service) runSSH(ctx context.Context, request Step8Request, observation Observation) Step8Result {
	if s.SSH == nil {
		return s.finishTransport(ctx, request.Profile, TransportSSH, terminalGateResult(Step8Result{RequestID: request.RequestID}, ResultDowngradeBlocked))
	}
	result := s.Gate.ApplyWithCredential(ctx, request, observation, func(credential []byte) Step8Result {
		runtime, opened := s.SSH.Open(ctx, Step8Result{RequestID: request.RequestID, Decision: DecisionSSHEligible, Class: ResultProofSuccess}, request.Profile, credential)
		if opened.Decision != DecisionSSHEligible || opened.Class != ResultProofSuccess || runtime == nil {
			if opened.RequestID == "" {
				opened.RequestID = request.RequestID
			}
			if opened.Decision != DecisionTerminal {
				opened = terminalGateResult(opened, ResultSessionFailure)
			}
			return opened
		}
		_, proved := runtime.Prove(ctx, request.Profile, credential)
		if proved.RequestID == "" {
			proved.RequestID = request.RequestID
		}
		return proved
	})
	if result.Decision != DecisionSSHEligible || result.Class != ResultProofSuccess || !result.Cleanup {
		return s.finishTransport(ctx, request.Profile, TransportSSH, result)
	}
	if s.Audit != nil {
		if err := s.Audit.Record(ctx, Step8AuditEvent{Transport: TransportSSH, Class: result.Class, Revision: result.ProofRevision, Cleanup: result.Cleanup}); err != nil {
			return terminalGateResult(result, ResultProofFailure)
		}
	}
	if s.Markers != nil {
		now := int64(0)
		if s.NowUnixMs != nil {
			now = s.NowUnixMs()
		} else {
			now = time.Now().UnixMilli()
		}
		if err := s.Markers.Write(ctx, request.Profile, Marker{SchemaVersion: MarkerSchemaVersion, AtUnixMs: now, Outcome: ResultProofSuccess, ProofRevision: result.ProofRevision}); err != nil {
			return terminalGateResult(result, ResultCleanupFailure)
		}
	}
	return result
}

func (s Step8Service) runWSS(ctx context.Context, request Step8Request) (result Step8Result) {
	result = Step8Result{RequestID: request.RequestID}
	if s.Credentials == nil || s.WSS == nil {
		return s.finish(ctx, request.Profile, terminalGateResult(result, ResultDowngradeBlocked))
	}
	credential, err := s.Credentials.Get(ctx, "ibmi/"+request.Profile.Name, request.Profile.CredentialMode)
	if err != nil || len(credential) == 0 {
		zeroCredential(credential)
		return s.finish(ctx, request.Profile, terminalGateResult(result, gateContextResult(ctx, ResultCredentialsUnavailable)))
	}
	session, err := s.WSS.Open(ctx, request.Profile)
	if err != nil {
		zeroCredential(credential)
		return s.finish(ctx, request.Profile, terminalGateResult(result, gateContextResult(ctx, ResultSessionFailure)))
	}
	metadata, proofErr := session.Prove(ctx, request.Profile.Username, credential)
	cleanupErr := session.Close()
	zeroCredential(credential)
	if cleanupErr != nil {
		return s.finish(ctx, request.Profile, terminalGateResult(result, ResultCleanupFailure))
	}
	if proofErr != nil || ValidateProofMetadata(metadata) != nil {
		return s.finish(ctx, request.Profile, terminalGateResult(result, gateContextResult(ctx, ResultProofFailure)))
	}
	result = Step8Result{RequestID: request.RequestID, Decision: DecisionWSSSelected, Class: ResultProofSuccess, ProofRevision: metadata.ProofRevision, Outcome: "authenticated_fixed_proof", Cleanup: true}
	if s.Audit != nil {
		if err := s.Audit.Record(ctx, Step8AuditEvent{Transport: TransportWSS, Class: result.Class, Revision: result.ProofRevision, Cleanup: result.Cleanup}); err != nil {
			return terminalGateResult(result, ResultProofFailure)
		}
	}
	if s.Markers != nil {
		now := int64(0)
		if s.NowUnixMs != nil {
			now = s.NowUnixMs()
		} else {
			now = time.Now().UnixMilli()
		}
		if err := s.Markers.Write(ctx, request.Profile, Marker{SchemaVersion: MarkerSchemaVersion, AtUnixMs: now, Outcome: ResultProofSuccess, ProofRevision: result.ProofRevision}); err != nil {
			return terminalGateResult(result, ResultCleanupFailure)
		}
	}
	return result
}

func (s Step8Service) finish(ctx context.Context, p profile.Profile, result Step8Result) Step8Result {
	return s.finishTransport(ctx, p, TransportWSS, result)
}

func (s Step8Service) finishTransport(ctx context.Context, p profile.Profile, transport Transport, result Step8Result) Step8Result {
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, Step8AuditEvent{Transport: transport, Class: result.Class, Revision: result.ProofRevision, Cleanup: result.Cleanup})
	}
	return result
}
