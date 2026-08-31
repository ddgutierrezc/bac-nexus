package configuration

// FallbackGrant is a policy-owned, secret-free claim. It cannot be created by
// the TUI and is valid only for the exact operation identity and eligible reason.
type FallbackGrant struct {
	RequestID  string
	Generation uint64
	Reason     Step8Reason
}

type PolicyFallbackAuthorizer struct{}

func (PolicyFallbackAuthorizer) Authorize(requestID string, generation uint64, reason Step8Reason) (FallbackGrant, bool) {
	if requestID == "" || generation == 0 || DecisionForReason(reason) != DecisionSSHEligible {
		return FallbackGrant{}, false
	}
	return FallbackGrant{RequestID: requestID, Generation: generation, Reason: reason}, true
}

type PolicySSHConsent struct{}

// From is the only adapter that turns a policy grant into SSH consent. The
// Step8Service still issues and immediately consumes its single-use ticket.
func (PolicySSHConsent) From(grant FallbackGrant, requestID string, generation uint64) (Step8Request, bool) {
	if grant.RequestID != requestID || grant.Generation != generation || grant.RequestID == "" || generation == 0 || DecisionForReason(grant.Reason) != DecisionSSHEligible {
		return Step8Request{}, false
	}
	return Step8Request{RequestID: requestID, Generation: generation, SSHConsent: true, FallbackClass: grant.Reason}, true
}
