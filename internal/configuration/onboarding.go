package configuration

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"bac-nexus/internal/credential"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
)

const automaticTOFUProvenance = "automatic-tofu-v1:first-contact-unverified"

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
type OnboardingRequest struct{ Host, Username string }
type OnboardingResult struct {
	Code            OnboardingCode
	Profile         profile.Profile
	CleanupRequired bool
	// CredentialRetained is true only when a keyring-backed transaction could
	// not complete its compensation. It is a classification, never a secret.
	CredentialRetained bool
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
	Existing   func(context.Context, string) (*profile.Profile, error)
	Proof      func(context.Context, profile.Profile, []byte) error
	Save       func(profile.Profile) error
	Delete     func(string) error
	Audit      func(context.Context, OnboardingAuditEvent) error
	Capability func() credential.Capability
	Keyring    credential.CredentialStore
	// Commit is the production persistence boundary. When set it owns the
	// prepared journal, ordered mutations, compensation, and committed audit.
	// Tests may omit it to exercise the small in-memory seam below.
	Commit func(context.Context, profile.Profile, []byte, func(context.Context) error) profile.OnboardingCommitResult
}
type onboardingCall struct {
	done   chan struct{}
	result OnboardingResult
	cancel context.CancelFunc
}
type OnboardingService struct {
	deps  OnboardingDeps
	mu    sync.Mutex
	next  uint64
	calls map[string]*onboardingCall
}

func NewOnboardingService(deps OnboardingDeps) *OnboardingService {
	return &OnboardingService{deps: deps, calls: map[string]*onboardingCall{}}
}
func (s *OnboardingService) StartCaptured(parent context.Context, request OnboardingRequest, secret []byte) (OperationIdentity, OnboardingCode) {
	if s == nil || parent == nil || parent.Err() != nil || len(secret) == 0 || profile.ValidateHost(request.Host) != nil || profile.ValidateUsername(request.Username) != nil || s.deps.Inspect == nil || s.deps.Proof == nil || s.deps.Save == nil || s.deps.Audit == nil {
		remote.Zero(secret)
		return OperationIdentity{}, OnboardingRejected
	}
	owned := append([]byte(nil), secret...)
	remote.Zero(secret)
	ctx, cancel := context.WithCancel(parent)
	s.mu.Lock()
	s.next++
	id := OperationIdentity{ID: fmt.Sprintf("onboarding-%d", s.next), Generation: s.next}
	call := &onboardingCall{done: make(chan struct{}), cancel: cancel}
	s.calls[id.ID] = call
	s.mu.Unlock()
	go func() { defer remote.Zero(owned); defer close(call.done); call.result = s.run(ctx, request, owned) }()
	return id, OnboardingStarted
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
}
func (s *OnboardingService) run(ctx context.Context, request OnboardingRequest, secret []byte) OnboardingResult {
	observation, err := s.deps.Inspect(ctx, request.Host, 22)
	if ctx.Err() != nil {
		return OnboardingResult{Code: OnboardingCancelled}
	}
	if err != nil || observation.Verified || profile.ValidateHostKey(observation.Fingerprint, profile.HostKeyTrustTOFU) != nil {
		return OnboardingResult{Code: OnboardingFailed}
	}
	name := strings.ReplaceAll(strings.ToLower(request.Username+"-"+request.Host), ".", "-")
	if len(name) > 64 {
		name = name[:64]
	}
	p := profile.Profile{SchemaVersion: profile.SchemaVersionV3, Name: name, Host: request.Host, Port: 22, Username: request.Username, HostKeyFingerprint: observation.Fingerprint, HostKeyTrust: profile.HostKeyTrustTOFU, HostKeyProvenance: automaticTOFUProvenance, CredentialMode: profile.CredentialModePrompt}
	if s.deps.Existing != nil {
		existing, existingErr := s.deps.Existing(ctx, name)
		if existingErr != nil || existing != nil && !sameOnboardingIdentity(*existing, p) {
			_ = s.deps.Audit(ctx, OnboardingAuditEvent{Code: "identity_changed", Profile: p.Name})
			return OnboardingResult{Code: OnboardingFailed}
		}
		if existing != nil {
			p = *existing
		}
	}
	if p.HostKeyProvenance == automaticTOFUProvenance {
		if s.deps.Audit(ctx, OnboardingAuditEvent{Code: "identity_bootstrap_allowed", Profile: p.Name}) != nil {
			if ctx.Err() != nil {
				return OnboardingResult{Code: OnboardingCancelled}
			}
			return OnboardingResult{Code: OnboardingFailed}
		}
	}
	if err := s.deps.Proof(ctx, p, secret); err != nil {
		if ctx.Err() != nil {
			return OnboardingResult{Code: OnboardingCancelled}
		}
		return OnboardingResult{Code: OnboardingFailed}
	}
	committedAudit := func(ctx context.Context) error {
		return s.deps.Audit(ctx, OnboardingAuditEvent{Code: "identity_pin_committed", Profile: p.Name})
	}
	if s.deps.Commit != nil {
		if s.deps.Capability != nil && s.deps.Capability() == credential.CapabilitySupported {
			if s.deps.Keyring == nil {
				return OnboardingResult{Code: OnboardingFailed}
			}
			p.CredentialMode = profile.CredentialModeKeyring
		}
		result := s.deps.Commit(ctx, p, secret, committedAudit)
		if !result.Saved {
			if ctx.Err() != nil {
				return OnboardingResult{Code: OnboardingCancelled}
			}
			return OnboardingResult{
				Code:               OnboardingFailed,
				CleanupRequired:    result.CleanupRequired,
				CredentialRetained: result.CleanupRequired && p.CredentialMode == profile.CredentialModeKeyring,
			}
		}
		return OnboardingResult{Code: OnboardingSaved, Profile: p}
	}
	if s.deps.Capability != nil && s.deps.Capability() == credential.CapabilitySupported {
		if s.deps.Keyring == nil || s.deps.Keyring.Set(p.Name, secret) != nil {
			return OnboardingResult{Code: OnboardingFailed}
		}
		p.CredentialMode = profile.CredentialModeKeyring
	}
	if err := s.deps.Save(p); err != nil {
		if p.CredentialMode == profile.CredentialModeKeyring {
			s.deps.Keyring.Delete(p.Name)
		}
		return OnboardingResult{Code: OnboardingFailed, CleanupRequired: true}
	}
	if committedAudit(ctx) != nil {
		cleanupRequired := s.deps.Delete == nil || s.deps.Delete(p.Name) != nil
		if p.CredentialMode == profile.CredentialModeKeyring {
			if s.deps.Keyring == nil || s.deps.Keyring.Delete(p.Name) != nil {
				cleanupRequired = true
			}
		}
		return OnboardingResult{Code: OnboardingFailed, CleanupRequired: cleanupRequired}
	}
	return OnboardingResult{Code: OnboardingSaved, Profile: p}
}

func sameOnboardingIdentity(existing, observed profile.Profile) bool {
	return existing.Name == observed.Name &&
		existing.Host == observed.Host &&
		existing.Port == observed.Port &&
		existing.Username == observed.Username &&
		existing.HostKeyFingerprint == observed.HostKeyFingerprint &&
		existing.HostKeyTrust == observed.HostKeyTrust
}
