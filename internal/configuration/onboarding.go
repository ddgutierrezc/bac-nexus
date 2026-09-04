package configuration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"bac-nexus/internal/credential"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
)

const automaticTOFUProvenance = "automatic-tofu-v1:first-contact-unverified"

// hostKeyInspectionTimeout matches the bounded SSH operation budget while
// limiting only the credential-free inspection phase.
const hostKeyInspectionTimeout = SSHRuntimeOperationTimeout

type OnboardingCode string

const (
	OnboardingStarted   OnboardingCode = "started"
	OnboardingRejected  OnboardingCode = "rejected"
	OnboardingSaved     OnboardingCode = "saved"
	OnboardingFailed    OnboardingCode = "failed"
	OnboardingCancelled OnboardingCode = "cancelled"
)

type OperationIdentity struct {
	ID         string
	Generation uint64
}
type OnboardingRequest struct {
	Name, Host, Username string
	Port                 int
}
type OnboardingResult struct {
	Code            OnboardingCode
	Profile         profile.Profile
	CleanupRequired bool
	// CredentialRetained is true only when a keyring-backed transaction could
	// not complete its compensation. It is a classification, never a secret.
	CredentialRetained bool
	Diagnostic         OnboardingDiagnostic
}

// OnboardingDiagnostic is deliberately small: it is safe to present to an
// operator but never carries source errors, credentials, tickets, or paths.
type OnboardingDiagnostic struct {
	Phase     OnboardingFailurePhase
	Class     OnboardingFailureClass
	Reference string
	Written   bool
}

type OnboardingFailurePhase string

const (
	OnboardingPhaseHostKeyInspection   OnboardingFailurePhase = "host_key_inspection"
	OnboardingPhaseExistingIdentity    OnboardingFailurePhase = "existing_identity"
	OnboardingPhaseAuthenticatedProof  OnboardingFailurePhase = "authenticated_proof"
	OnboardingPhaseBootstrapAudit      OnboardingFailurePhase = "bootstrap_audit"
	OnboardingPhaseKeyringPrecondition OnboardingFailurePhase = "keyring_precondition"
	OnboardingPhaseCommit              OnboardingFailurePhase = "commit"
	OnboardingPhaseSave                OnboardingFailurePhase = "save"
)

type OnboardingFailureClass string

const (
	OnboardingClassHostKeyFailure          OnboardingFailureClass = "host_key_failure"
	OnboardingClassHostKeyTimeout          OnboardingFailureClass = "timeout"
	OnboardingClassHostKeyNegotiation      OnboardingFailureClass = "negotiation"
	OnboardingClassHostKeyNoKey            OnboardingFailureClass = "no_key"
	OnboardingClassHostKeyUnavailable      OnboardingFailureClass = "unavailable"
	OnboardingClassHostKeyInvalidCandidate OnboardingFailureClass = "invalid_candidate"
	OnboardingClassIdentityFailure         OnboardingFailureClass = "identity_failure"
	OnboardingClassProofFailure            OnboardingFailureClass = "proof_failure"
	OnboardingClassBootstrapAuditFailure   OnboardingFailureClass = "bootstrap_audit_failure"
	OnboardingClassKeyringUnavailable      OnboardingFailureClass = "keyring_unavailable"
	OnboardingClassCommitFailure           OnboardingFailureClass = "commit_failure"
	OnboardingClassSaveFailure             OnboardingFailureClass = "save_failure"
)

// OnboardingFailureRecorder is owned by configuration because callers decide
// when a primary onboarding failure is safe to persist. Implementations may
// only return an opaque local reference.
type OnboardingFailureRecorder interface {
	Record(context.Context, OnboardingFailurePhase, OnboardingFailureClass, bool, bool) (string, error)
}
type OnboardingAuditEvent struct {
	Code    string
	Profile string
}
type OnboardingDeps struct {
	Inspect func(context.Context, string, int) (remote.HostKeyObservation, error)
	// Existing returns the persisted profile for the derived profile name. A nil
	// profile means this is a first-use enrollment; an error is ambiguous and
	// must fail closed.
	Existing func(context.Context, string) (*profile.Profile, error)
	// Proof returns the typed Step 8 result so its already-safe terminal class
	// can be retained without copying request IDs, tickets, outcomes, or errors.
	Proof      func(context.Context, profile.Profile, []byte) Step8Result
	Save       func(profile.Profile) error
	Delete     func(string) error
	Audit      func(context.Context, OnboardingAuditEvent) error
	Capability func() credential.Capability
	Keyring    credential.CredentialStore
	Now        func() time.Time
	// Commit is the production persistence boundary. When set it owns the
	// prepared journal, ordered mutations, compensation, and committed audit.
	// Tests may omit it to exercise the small in-memory seam below.
	Commit      func(context.Context, profile.Profile, []byte, func(context.Context) error) profile.OnboardingCommitResult
	Diagnostics OnboardingFailureRecorder
}
type onboardingCall struct {
	done   chan struct{}
	result OnboardingResult
	cancel context.CancelFunc
}
type secretLease struct {
	secret     []byte
	expiresAt  time.Time
	generation uint64
}
type OnboardingService struct {
	deps   OnboardingDeps
	mu     sync.Mutex
	next   uint64
	calls  map[string]*onboardingCall
	leases map[string]*secretLease
}

func NewOnboardingService(deps OnboardingDeps) *OnboardingService {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &OnboardingService{deps: deps, calls: map[string]*onboardingCall{}, leases: map[string]*secretLease{}}
}
func validOnboardingRequest(request OnboardingRequest) bool {
	return profile.ValidateName(request.Name) == nil && profile.ValidateHost(request.Host) == nil && profile.ValidateUsername(request.Username) == nil && profile.ValidatePort(request.Port) == nil
}

// Capture moves terminal bytes directly into an application-owned, expiring
// lease. Its result contains only an opaque identity and a secret-free code.
func (s *OnboardingService) Capture(ctx context.Context, request OnboardingRequest, prompt remote.SecretPrompt, input, output *os.File, label string) (OperationIdentity, remote.PromptCode) {
	if s == nil || !validOnboardingRequest(request) {
		return OperationIdentity{}, remote.PromptUnavailable
	}
	secret, code := prompt.Capture(ctx, input, output, label)
	if code != remote.PromptCaptured || len(secret) < 1 || len(secret) > 1024 {
		remote.Zero(secret)
		if code == remote.PromptCaptured {
			code = remote.PromptUnavailable
		}
		return OperationIdentity{}, code
	}
	owned := append([]byte(nil), secret...)
	remote.Zero(secret)
	s.mu.Lock()
	for id, lease := range s.leases {
		remote.Zero(lease.secret)
		delete(s.leases, id)
	}
	s.next++
	id := OperationIdentity{ID: fmt.Sprintf("onboarding-%d", s.next), Generation: s.next}
	s.leases[id.ID] = &secretLease{secret: owned, expiresAt: s.deps.Now().Add(2 * time.Minute), generation: id.Generation}
	s.mu.Unlock()
	return id, remote.PromptCaptured
}

// StartCaptured atomically consumes a lease. The worker owns the bytes until
// it returns; its done channel closes only after the buffer is zeroed.
func (s *OnboardingService) StartCaptured(parent context.Context, request OnboardingRequest, id OperationIdentity) OnboardingCode {
	if s == nil || parent == nil || parent.Err() != nil || !validOnboardingRequest(request) {
		return OnboardingRejected
	}
	s.mu.Lock()
	lease := s.leases[id.ID]
	if lease == nil || lease.generation != id.Generation || !s.deps.Now().Before(lease.expiresAt) {
		if lease != nil {
			remote.Zero(lease.secret)
			delete(s.leases, id.ID)
		}
		s.mu.Unlock()
		return OnboardingRejected
	}
	owned := lease.secret
	delete(s.leases, id.ID)
	if s.deps.Inspect == nil || s.deps.Proof == nil || s.deps.Save == nil || s.deps.Audit == nil {
		remote.Zero(owned)
		s.mu.Unlock()
		return OnboardingRejected
	}
	ctx, cancel := context.WithCancel(parent)
	call := &onboardingCall{done: make(chan struct{}), cancel: cancel}
	s.calls[id.ID] = call
	s.mu.Unlock()
	go func() { defer close(call.done); defer remote.Zero(owned); call.result = s.run(ctx, request, owned) }()
	return OnboardingStarted
}
func (s *OnboardingService) Wait(ctx context.Context, id string) OnboardingResult {
	s.mu.Lock()
	call := s.calls[id]
	s.mu.Unlock()
	if call == nil {
		return OnboardingResult{Code: OnboardingFailed}
	}
	select {
	case <-ctx.Done():
		return OnboardingResult{Code: OnboardingCancelled}
	case <-call.done:
		return call.result
	}
}
func (s *OnboardingService) Cancel(id string) {
	s.mu.Lock()
	call := s.calls[id]
	s.mu.Unlock()
	if call != nil {
		call.cancel()
	}
	s.mu.Lock()
	if lease := s.leases[id]; lease != nil {
		remote.Zero(lease.secret)
		delete(s.leases, id)
	}
	s.mu.Unlock()
}
func (s *OnboardingService) Revoke(id OperationIdentity) { s.Cancel(id.ID) }

// Shutdown revokes every unconsumed lease, cancels active workers, and waits
// until each worker has zeroed its owned secret buffer.
func (s *OnboardingService) Shutdown() {
	if s == nil {
		return
	}
	s.mu.Lock()
	calls := make([]*onboardingCall, 0, len(s.calls))
	for _, call := range s.calls {
		calls = append(calls, call)
	}
	for id, lease := range s.leases {
		remote.Zero(lease.secret)
		delete(s.leases, id)
	}
	s.mu.Unlock()
	for _, call := range calls {
		call.cancel()
	}
	for _, call := range calls {
		<-call.done
	}
}

func (s *OnboardingService) run(ctx context.Context, request OnboardingRequest, secret []byte) OnboardingResult {
	inspectionCtx, cancelInspection := context.WithTimeout(ctx, hostKeyInspectionTimeout)
	observation, err := s.deps.Inspect(inspectionCtx, request.Host, request.Port)
	cancelInspection()
	if ctx.Err() != nil {
		return OnboardingResult{Code: OnboardingCancelled}
	}
	if err != nil || observation.Verified || profile.ValidateHostKey(observation.Fingerprint, profile.HostKeyTrustTOFU) != nil {
		return s.failed(ctx, OnboardingPhaseHostKeyInspection, onboardingHostKeyFailureClass(err), false, false)
	}
	p := profile.Profile{SchemaVersion: profile.SchemaVersionV3, Name: request.Name, Host: request.Host, Port: request.Port, Username: request.Username, HostKeyFingerprint: observation.Fingerprint, HostKeyTrust: profile.HostKeyTrustTOFU, HostKeyProvenance: automaticTOFUProvenance, CredentialMode: profile.CredentialModePrompt}
	if s.deps.Existing != nil {
		existing, existingErr := s.deps.Existing(ctx, p.Name)
		if existingErr != nil || existing != nil && !sameOnboardingIdentity(*existing, p) {
			_ = s.deps.Audit(ctx, OnboardingAuditEvent{Code: "identity_changed", Profile: p.Name})
			return s.failed(ctx, OnboardingPhaseExistingIdentity, OnboardingClassIdentityFailure, false, false)
		}
		if existing != nil {
			p = *existing
		}
	}
	proof := s.deps.Proof(ctx, p, secret)
	if ctx.Err() != nil {
		return OnboardingResult{Code: OnboardingCancelled}
	}
	if !successfulOnboardingProof(proof) {
		return s.failed(ctx, OnboardingPhaseAuthenticatedProof, onboardingProofFailureClass(proof), false, false)
	}
	if p.HostKeyProvenance == automaticTOFUProvenance {
		if s.deps.Audit(ctx, OnboardingAuditEvent{Code: "identity_bootstrap_allowed", Profile: p.Name}) != nil {
			if ctx.Err() != nil {
				return OnboardingResult{Code: OnboardingCancelled}
			}
			return s.failed(ctx, OnboardingPhaseBootstrapAudit, OnboardingClassBootstrapAuditFailure, false, false)
		}
	}
	committedAudit := func(ctx context.Context) error {
		return s.deps.Audit(ctx, OnboardingAuditEvent{Code: "identity_pin_committed", Profile: p.Name})
	}
	if s.deps.Commit != nil {
		if s.deps.Capability != nil && s.deps.Capability() == credential.CapabilitySupported {
			if s.deps.Keyring == nil {
				return s.failed(ctx, OnboardingPhaseKeyringPrecondition, OnboardingClassKeyringUnavailable, false, false)
			}
			p.CredentialMode = profile.CredentialModeKeyring
		}
		result := s.deps.Commit(ctx, p, secret, committedAudit)
		if !result.Saved {
			if ctx.Err() != nil {
				return OnboardingResult{Code: OnboardingCancelled}
			}
			return s.failed(ctx, OnboardingPhaseCommit, OnboardingClassCommitFailure, result.CleanupRequired, result.CleanupRequired && p.CredentialMode == profile.CredentialModeKeyring)
		}
		return OnboardingResult{Code: OnboardingSaved, Profile: p}
	}
	if s.deps.Capability != nil && s.deps.Capability() == credential.CapabilitySupported {
		if s.deps.Keyring == nil || s.deps.Keyring.Set(p.Name, secret) != nil {
			return s.failed(ctx, OnboardingPhaseKeyringPrecondition, OnboardingClassKeyringUnavailable, false, false)
		}
		p.CredentialMode = profile.CredentialModeKeyring
	}
	if err := s.deps.Save(p); err != nil {
		if p.CredentialMode == profile.CredentialModeKeyring {
			s.deps.Keyring.Delete(p.Name)
		}
		return s.failed(ctx, OnboardingPhaseSave, OnboardingClassSaveFailure, true, false)
	}
	if committedAudit(ctx) != nil {
		cleanupRequired := s.deps.Delete == nil || s.deps.Delete(p.Name) != nil
		if p.CredentialMode == profile.CredentialModeKeyring {
			if s.deps.Keyring == nil || s.deps.Keyring.Delete(p.Name) != nil {
				cleanupRequired = true
			}
		}
		return s.failed(ctx, OnboardingPhaseSave, OnboardingClassSaveFailure, cleanupRequired, false)
	}
	return OnboardingResult{Code: OnboardingSaved, Profile: p}
}

func onboardingHostKeyFailureClass(err error) OnboardingFailureClass {
	if errors.Is(err, context.DeadlineExceeded) {
		return OnboardingClassHostKeyTimeout
	}
	if err == nil {
		return OnboardingClassHostKeyInvalidCandidate
	}
	var probe *remote.HostKeyProbeError
	if !errors.As(err, &probe) {
		return OnboardingClassHostKeyUnavailable
	}
	switch probe.Kind {
	case remote.HostKeyProbeTimeout:
		return OnboardingClassHostKeyTimeout
	case remote.HostKeyProbeNegotiation:
		return OnboardingClassHostKeyNegotiation
	case remote.HostKeyProbeNoKey:
		return OnboardingClassHostKeyNoKey
	default:
		return OnboardingClassHostKeyFailure
	}
}

func successfulOnboardingProof(result Step8Result) bool {
	return (result.Decision == DecisionWSSSelected || result.Decision == DecisionSSHEligible) &&
		result.Class == ResultProofSuccess && result.ProofRevision == ProofRevision && result.Cleanup
}

func onboardingProofFailureClass(result Step8Result) OnboardingFailureClass {
	if result.Decision == DecisionTerminal && IsTerminalResult(result.Class) && result.Class != ResultCancelled {
		return OnboardingFailureClass(result.Class)
	}
	return OnboardingClassProofFailure
}

func (s *OnboardingService) failed(ctx context.Context, phase OnboardingFailurePhase, class OnboardingFailureClass, cleanupRequired, credentialRetained bool) OnboardingResult {
	result := OnboardingResult{Code: OnboardingFailed, CleanupRequired: cleanupRequired, CredentialRetained: credentialRetained, Diagnostic: OnboardingDiagnostic{Phase: phase, Class: class}}
	if ctx.Err() != nil || s.deps.Diagnostics == nil {
		return result
	}
	reference, err := s.deps.Diagnostics.Record(ctx, phase, class, cleanupRequired, credentialRetained)
	if err == nil && validOnboardingDiagnosticReference(reference) {
		result.Diagnostic.Reference, result.Diagnostic.Written = reference, true
	}
	return result
}

func validOnboardingDiagnosticReference(reference string) bool {
	if len(reference) != len("ONB-")+32 || reference[:4] != "ONB-" {
		return false
	}
	for _, value := range reference[4:] {
		if !(value >= '0' && value <= '9' || value >= 'a' && value <= 'f') {
			return false
		}
	}
	return true
}

func sameOnboardingIdentity(existing, observed profile.Profile) bool {
	return existing.Name == observed.Name &&
		existing.Host == observed.Host &&
		existing.Port == observed.Port &&
		existing.Username == observed.Username &&
		existing.HostKeyFingerprint == observed.HostKeyFingerprint &&
		existing.HostKeyTrust == observed.HostKeyTrust
}
