// Package security defines the local-principal authorization boundary
// for v1 MCP operations. The current local OS principal is the trust
// boundary; advisory selectors are an accidental-exposure control, not
// Copilot or OpenCode authentication. The package has no remote, path,
// shell, SQL, or SSH surface; every blocking call honors
// context.Context cancellation.
package security

import (
	"context"
	"crypto/subtle"
	"errors"
)

// Errors returned by the authorization boundary. Each error is a
// deterministic classification; the message never contains source,
// path, host, user, credential, or raw error material.
var (
	ErrUnauthorized         = errors.New("unauthorized")
	ErrHostKeyChanged       = errors.New("host_key_changed")
	ErrTrustEvidenceMissing = errors.New("trust_evidence_missing")
)

// Selector is the approved advisory capability identifier. It is a
// allowlist entry, not a product authentication claim.
type Selector string

// Approved advisory selectors. Each selector maps to exactly one
// capability class; adding a selector requires an explicit decision.
const (
	SelectorResolveCatalog Selector = "resolve_catalog_candidates"
	SelectorReadSource     Selector = "read_selected_source"
)

// CapabilityClass is the read-only capability class bound to a
// selector. The class determines which target class is acceptable.
type CapabilityClass string

const (
	CapabilityCatalogResolve CapabilityClass = "catalog_resolve"
	CapabilitySourceRead     CapabilityClass = "source_read"
)

// CapabilityTarget is the resource target class of a capability
// invocation. Targets outside the allowlist always fail closed.
type CapabilityTarget string

const (
	TargetIBMiCatalog CapabilityTarget = "ibmi_catalog"
	TargetIBMiSource  CapabilityTarget = "ibmi_source"
)

// Decision is the deterministic authorization outcome. Raw error
// material is never used as a decision.
type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
)

// Decision_ is the typed outcome of an authorization call. The Reason
// is a bounded, non-sensitive classification.
type Decision_ struct {
	Selector Selector
	Class    CapabilityClass
	Target   CapabilityTarget
	Decision Decision
	Reason   string
}

// Authorizer is the consumer-owned authorization boundary. The
// boundary takes only (ctx, selector, target); it never reads parent
// process, product identity, or clientInfo content.
type Authorizer interface {
	Authorize(ctx context.Context, selector Selector, target CapabilityTarget) (Decision_, error)
}

// Policy is the read-only allowlist policy. It depends on no remote
// or path input and consumes the supplied selectors directly. The same
// local OS principal receives equal authorization on every supported
// platform.
type Policy struct {
	allowed map[Selector]CapabilityClass
	targets map[CapabilityClass]CapabilityTarget
}

// NewPolicy returns the canonical v1 allowlist policy. The policy has
// no configuration, no remote dependency, and no path input.
func NewPolicy() *Policy {
	return &Policy{
		allowed: map[Selector]CapabilityClass{
			SelectorResolveCatalog: CapabilityCatalogResolve,
			SelectorReadSource:     CapabilitySourceRead,
		},
		targets: map[CapabilityClass]CapabilityTarget{
			CapabilityCatalogResolve: TargetIBMiCatalog,
			CapabilitySourceRead:     TargetIBMiSource,
		},
	}
}

// Authorize applies the read-only allowlist to a selector and target
// pair. The result is deterministic: an unknown or malformed selector
// is denied before any side effect, and a selector/target mismatch is
// denied at the same boundary.
func (p *Policy) Authorize(ctx context.Context, selector Selector, target CapabilityTarget) (Decision_, error) {
	if err := ctx.Err(); err != nil {
		return emptyDecision(), err
	}
	normalized, ok := normalizeSelector(selector)
	if !ok {
		return Decision_{Selector: selector, Decision: DecisionDeny, Reason: "selector not allowlisted"}, nil
	}
	class, allowed := p.allowed[normalized]
	if !allowed {
		return Decision_{Selector: selector, Decision: DecisionDeny, Reason: "selector not allowlisted"}, nil
	}
	expected, allowed := p.targets[class]
	if !allowed || expected != target {
		return Decision_{Selector: selector, Class: class, Target: target, Decision: DecisionDeny, Reason: "target class mismatch"}, nil
	}
	return Decision_{Selector: selector, Class: class, Target: target, Decision: DecisionAllow, Reason: "allowlisted selector and matching target class"}, nil
}

// emptyDecision returns a zero Decision_ value. The trailing
// underscore on the type name is parsed as a type literal, so the
// zero value is constructed through this helper.
func emptyDecision() Decision_ { return Decision_{} }

// normalizeSelector validates a selector against the allowlist
// preconditions: non-empty, no control bytes, no DEL, and exact
// case-sensitive match. The allowlist lookup itself happens in
// Authorize; this function only enforces input shape so that a
// malformed selector cannot reach the lookup map.
func normalizeSelector(selector Selector) (Selector, bool) {
	value := string(selector)
	if value == "" {
		return "", false
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return "", false
		}
	}
	return Selector(value), true
}

// Principal is the current local OS principal trust marker. The same
// local principal receives equal authorization on every supported
// platform; there is no parent-process, product-identity, or
// clientInfo check.
type Principal struct{}

// NewLocalPrincipal returns the principal marker for the current
// local OS user. It performs no parent verification, no product
// identity check, and never branches on clientInfo.
func NewLocalPrincipal() Principal { return Principal{} }

// HostKeyTrust is the pinned host-key trust mode. The package never
// silently enrolls a new key.
type HostKeyTrust string

const (
	HostKeyTrustTOFU     HostKeyTrust = "tofu"
	HostKeyTrustVerified HostKeyTrust = "verified"
)

// PinnedTarget captures the approved pin and provenance. A zero-value
// pin is not a valid pin; the seam fails closed.
type PinnedTarget struct {
	Trust       HostKeyTrust
	Fingerprint string
	Binding     []byte
	Provenance  string
}

// PinnedTrust verifies observed host evidence against a pinned
// target. The seam never silently enrolls, never rotates, and never
// inspects more than one observed fingerprint or binding at a time.
type PinnedTrust struct{}

// NewPinnedTrust returns a value-typed pinned TOFU seam. The seam has
// no state and performs no I/O.
func NewPinnedTrust() PinnedTrust { return PinnedTrust{} }

// Enroll is the explicit TOFU operation. It records only non-secret
// provenance in the returned pin; Verify never calls it implicitly.
func (PinnedTrust) Enroll(ctx context.Context, fingerprint string, binding []byte, provenance string) (PinnedTarget, error) {
	if err := ctx.Err(); err != nil {
		return PinnedTarget{}, err
	}
	if fingerprint == "" || len(binding) == 0 || provenance == "" {
		return PinnedTarget{}, ErrTrustEvidenceMissing
	}
	return PinnedTarget{Trust: HostKeyTrustTOFU, Fingerprint: fingerprint, Binding: append([]byte(nil), binding...), Provenance: provenance}, nil
}

// Verify applies the pinned TOFU policy. It fails closed on missing,
// malformed, mismatched, or ambiguous evidence. The comparison uses
// crypto/subtle.ConstantTimeCompare so prefix matches do not leak
// digest length.
func (PinnedTrust) Verify(ctx context.Context, pin PinnedTarget, observedFingerprint string, observedBinding []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validatePin(pin); err != nil {
		return err
	}
	if observedFingerprint == "" || len(observedBinding) == 0 {
		return ErrTrustEvidenceMissing
	}
	if observedFingerprint != pin.Fingerprint {
		return ErrHostKeyChanged
	}
	if subtle.ConstantTimeCompare(observedBinding, pin.Binding) != 1 {
		return ErrHostKeyChanged
	}
	return nil
}

// validatePin checks that a pin carries the evidence required for a
// meaningful trust decision. The seam never silently enrolls a new
// pin from observed evidence, so an empty pin is always rejected.
func validatePin(pin PinnedTarget) error {
	if pin.Trust != HostKeyTrustTOFU && pin.Trust != HostKeyTrustVerified {
		return ErrTrustEvidenceMissing
	}
	if pin.Fingerprint == "" || len(pin.Binding) == 0 {
		return ErrTrustEvidenceMissing
	}
	return nil
}
