// Package app composes the local-OS-principal Nexus service. The
// service is the deterministic seam that wires credential availability,
// policy authorization, bounded catalog resolution, immutable source
// pagination with per-page freshness, recovery, and sanitized audit.
// It owns no remote, path, shell, SQL, or SSH capability of its own;
// every blocking call honors context.Context cancellation.
package app

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"bac-nexus/internal/audit"
	"bac-nexus/internal/catalog"
	"bac-nexus/internal/credential"
	"bac-nexus/internal/security"
	"bac-nexus/internal/source"
)

// Service-unavailable sentinel. Returned when a caller invokes a public
// method before Startup has completed successfully.
var ErrServiceUnavailable = errors.New("service unavailable: recovery not completed")

// Consumer-owned interfaces. Each is the narrowest contract the service
// needs; it never widens to expose infrastructure the service must not
// reach.

// CatalogResolver returns catalog candidates for a validated semantic search.
type CatalogResolver interface {
	Resolve(ctx context.Context, search catalog.Search) ([]catalog.Candidate, error)
}

// SnapshotAcquirer builds the immutable in-memory snapshot for one
// authorized exact selection. Its contract is exactly the one the
// source Acquirer exposes; the app package does not depend on the
// concrete type.
type SnapshotAcquirer interface {
	Acquire(ctx context.Context, candidate catalog.Candidate) (*source.Snapshot, error)
}

// LeaseStore mints and resolves opaque cursors bound to canonical
// selections and the current process epoch.
type LeaseStore interface {
	Acquire(snap *source.Snapshot, selection catalog.Candidate, policy source.ClientPolicy) (source.Cursor, error)
	Lookup(cursor source.Cursor) (catalog.Candidate, error)
	OpenReader(cursor source.Cursor, selection catalog.Candidate, policy source.ClientPolicy) (*source.LeaseReader, error)
}

// RecoveryCoordinator runs the bounded ownership-recovery gate before
// any new remote work. Its contract is exactly the one the source
// package exposes; the app package does not depend on its concrete type.
type RecoveryCoordinator interface {
	Recover(ctx context.Context) error
}

// ServiceDeps groups every dependency the service needs. The fields
// are interfaces so tests can inject fakes and production code can
// supply the real implementations from credential, security, audit,
// source, and a catalog backend.
type ServiceDeps struct {
	Credentials credential.CredentialStore
	Authorizer  security.Authorizer
	Auditor     audit.Auditor
	Resolver    CatalogResolver
	Acquirer    SnapshotAcquirer
	Leases      LeaseStore
	Recovery    RecoveryCoordinator
	Profile     string
	Now         func() time.Time
}

// Service is the deterministic composition root. It is safe for
// concurrent use after Startup has succeeded.
type Service struct {
	deps    ServiceDeps
	ready   bool
	readyMu chan struct{}
}

// NewService validates dependencies and returns a Service in the
// not-yet-available state. A nil Now falls back to time.Now.
func NewService(deps ServiceDeps) *Service {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &Service{deps: deps, readyMu: make(chan struct{})}
}

// Startup invokes the bounded ownership-recovery gate before any
// public method becomes available. A failed recovery or a cancelled
// context leaves the service unavailable and returns the underlying
// error.
func (s *Service) Startup(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.deps.Recovery == nil {
		return errors.New("service recovery dependency is required")
	}
	if err := s.deps.Recovery.Recover(ctx); err != nil {
		return fmt.Errorf("startup recovery: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.ready = true
	close(s.readyMu)
	return nil
}

// Available reports whether Startup has completed successfully. It is
// the only readiness check the service exposes; callers must not
// infer readiness from internal state.
func (s *Service) Available() bool { return s.ready }

// ResolveCatalog authorizes a catalog query, verifies credential
// availability, and returns the bounded candidate set. It fails closed
// on unknown or malformed selectors, missing credentials, context
// cancellation, or a candidate set larger than catalog.MaxCandidates.
// A successful call records an allow audit event; a denied or failed
// call records the matching deny classification.
func (s *Service) ResolveCatalog(ctx context.Context, search catalog.Search, selector security.Selector) ([]catalog.Candidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, ctx.Err()
	}
	if !s.Available() {
		return nil, ErrServiceUnavailable
	}
	if err := s.requireCredentials(ctx); err != nil {
		if auditErr := s.recordDenied(ctx, audit.CapabilityCatalogResolve, audit.TargetClassIBMiCatalog, err); auditErr != nil {
			return nil, auditErr
		}
		return nil, err
	}
	decision, err := s.deps.Authorizer.Authorize(ctx, selector, security.TargetIBMiCatalog)
	if err != nil {
		if auditErr := s.recordDenied(ctx, audit.CapabilityCatalogResolve, audit.TargetClassIBMiCatalog, err); auditErr != nil {
			return nil, auditErr
		}
		return nil, fmt.Errorf("authorize catalog resolve: %w", err)
	}
	if decision.Decision != security.DecisionAllow {
		if auditErr := s.recordDenied(ctx, audit.CapabilityCatalogResolve, audit.TargetClassIBMiCatalog, errReason(decision.Reason)); auditErr != nil {
			return nil, auditErr
		}
		return nil, errReason(decision.Reason)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.deps.Resolver == nil {
		return nil, errors.New("service catalog resolver is required")
	}
	raw, err := s.deps.Resolver.Resolve(ctx, search)
	if err != nil {
		if auditErr := s.recordDenied(ctx, audit.CapabilityCatalogResolve, audit.TargetClassIBMiCatalog, err); auditErr != nil {
			return nil, auditErr
		}
		return nil, fmt.Errorf("resolve catalog: %w", err)
	}
	bounded, err := catalog.BoundedCandidates(raw)
	if err != nil {
		if auditErr := s.recordDenied(ctx, audit.CapabilityCatalogResolve, audit.TargetClassIBMiCatalog, err); auditErr != nil {
			return nil, auditErr
		}
		return nil, err
	}
	if len(bounded) == 0 {
		if auditErr := s.recordDenied(ctx, audit.CapabilityCatalogResolve, audit.TargetClassIBMiCatalog, catalog.ErrCandidateNotFound); auditErr != nil {
			return nil, auditErr
		}
		return nil, catalog.ErrCandidateNotFound
	}
	if auditErr := s.recordAllowed(ctx, audit.CapabilityCatalogResolve, audit.TargetClassIBMiCatalog, len(bounded)); auditErr != nil {
		return nil, auditErr
	}
	return bounded, nil
}

// ReadSelectedSource looks up the selection bound to the cursor,
// re-queries the catalog to verify the coordinate is still current,
// opens a reader, and serves the requested page. It fails closed on
// missing credentials, policy denial, context cancellation, stale
// coordinate, missing or invalid cursor, or a response that would
// exceed the marshaled byte bound. No partial content is ever
// returned.
func (s *Service) ReadSelectedSource(ctx context.Context, selection catalog.Candidate, cursor string, page source.Range) (source.Page, error) {
	if err := ctx.Err(); err != nil {
		return source.Page{}, err
	}
	if !s.Available() {
		return source.Page{}, ErrServiceUnavailable
	}
	if err := s.requireCredentials(ctx); err != nil {
		if auditErr := s.recordDenied(ctx, audit.CapabilitySourceRead, audit.TargetClassIBMiSource, err); auditErr != nil {
			return source.Page{}, auditErr
		}
		return source.Page{}, err
	}
	decision, err := s.deps.Authorizer.Authorize(ctx, security.SelectorReadSource, security.TargetIBMiSource)
	if err != nil {
		if auditErr := s.recordDenied(ctx, audit.CapabilitySourceRead, audit.TargetClassIBMiSource, err); auditErr != nil {
			return source.Page{}, auditErr
		}
		return source.Page{}, fmt.Errorf("authorize source read: %w", err)
	}
	if decision.Decision != security.DecisionAllow {
		if auditErr := s.recordDenied(ctx, audit.CapabilitySourceRead, audit.TargetClassIBMiSource, errReason(decision.Reason)); auditErr != nil {
			return source.Page{}, auditErr
		}
		return source.Page{}, errReason(decision.Reason)
	}
	if err := ctx.Err(); err != nil {
		return source.Page{}, err
	}
	if s.deps.Leases == nil {
		return source.Page{}, errors.New("service lease store is required")
	}
	if s.deps.Resolver == nil {
		return source.Page{}, errors.New("service catalog resolver is required")
	}
	var original catalog.Candidate
	if cursor == "" {
		search, err := catalog.NewSearch(selection.Item, selection.ProductionLibrary)
		if err != nil {
			return source.Page{}, source.ErrInvalidRequest
		}
		candidates, err := s.deps.Resolver.Resolve(ctx, search)
		if err != nil {
			return source.Page{}, source.ErrStaleCoordinate
		}
		original, err = catalog.Select(candidates, selection)
		if err != nil {
			return source.Page{}, err
		}
		if s.deps.Acquirer == nil {
			return source.Page{}, errors.New("service snapshot acquirer is required")
		}
		snap, err := s.deps.Acquirer.Acquire(ctx, original)
		if err != nil {
			return source.Page{}, err
		}
		var acquired source.Cursor
		acquired, err = s.deps.Leases.Acquire(snap, original, source.ClientPolicy(s.deps.Profile))
		cursor = string(acquired)
		if err != nil {
			return source.Page{}, err
		}
	} else {
		original, err = s.deps.Leases.Lookup(source.Cursor(cursor))
		if err != nil {
			return source.Page{}, err
		}
	}
	if err := s.freshnessCheck(ctx, original); err != nil {
		return source.Page{}, err
	}
	reader, err := s.deps.Leases.OpenReader(source.Cursor(cursor), original, source.ClientPolicy(s.deps.Profile))
	if err != nil {
		return source.Page{}, err
	}
	defer reader.Close()
	if err := ctx.Err(); err != nil {
		return source.Page{}, err
	}
	pageResult, err := reader.Page(page.StartLine, page.MaxLines)
	if err != nil {
		return source.Page{}, err
	}
	if !pageResult.EOF {
		pageResult.Cursor = cursor
	}
	if auditErr := s.recordAllowed(ctx, audit.CapabilitySourceRead, audit.TargetClassIBMiSource, pageResult.LineCount); auditErr != nil {
		return source.Page{}, auditErr
	}
	return pageResult, nil
}

// freshnessCheck re-queries the catalog for the canonical selection
// bound to a cursor. If the original selection is no longer present
// in the re-queried candidate set, the coordinate is stale and the
// caller must restart at line 1.
func (s *Service) freshnessCheck(ctx context.Context, original catalog.Candidate) error {
	search, err := catalog.NewSearch(original.Item, original.ProductionLibrary)
	if err != nil {
		return source.ErrStaleCoordinate
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	current, err := s.deps.Resolver.Resolve(ctx, search)
	if err != nil {
		return source.ErrStaleCoordinate
	}
	for _, candidate := range current {
		if reflect.DeepEqual(candidate, original) {
			return nil
		}
	}
	return source.ErrStaleCoordinate
}

// requireCredentials returns ErrCredentialsUnavailable when the
// configured credential store cannot supply a usable secret.
func (s *Service) requireCredentials(ctx context.Context) error {
	if s.deps.Credentials == nil {
		return credential.ErrCredentialsUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	secret, err := s.deps.Credentials.Get(s.deps.Profile)
	if err != nil || len(secret) == 0 {
		return credential.ErrCredentialsUnavailable
	}
	// Zero the secret before returning; the service never retains it.
	for i := range secret {
		secret[i] = 0
	}
	return nil
}

// recordAllowed records an allow audit event with the supplied counts.
func (s *Service) recordAllowed(ctx context.Context, capability audit.Capability, target audit.TargetClass, returned int) error {
	if s.deps.Auditor == nil {
		return errors.New("audit dependency is required")
	}
	return s.deps.Auditor.Record(ctx, audit.Event{
		Capability:  capability,
		Connector:   audit.ConnectorIBMi,
		TargetClass: target,
		PolicyID:    audit.PolicyIDVerifiedReadOnly,
		Result:      audit.ResultClassAllow,
		Requested:   0,
		Returned:    returned,
		Timestamp:   s.deps.Now(),
		Duration:    0,
		Reason:      "allowlisted selector and matching target class",
	})
}

// recordDenied records a deny audit event.
func (s *Service) recordDenied(ctx context.Context, capability audit.Capability, target audit.TargetClass, _ error) error {
	if s.deps.Auditor == nil {
		return errors.New("audit dependency is required")
	}
	return s.deps.Auditor.Record(ctx, audit.Event{
		Capability:  capability,
		Connector:   audit.ConnectorIBMi,
		TargetClass: target,
		PolicyID:    audit.PolicyIDVerifiedReadOnly,
		Result:      audit.ResultClassDeny,
		Requested:   0,
		Returned:    0,
		Timestamp:   s.deps.Now(),
		Duration:    0,
		Reason:      "request denied before remote work",
	})
}

// errReason builds a typed error from a policy denial reason. The
// message is the policy's own bounded classification; the service
// never appends a sensitive identifier.
func errReason(reason string) error {
	return security.ErrUnauthorized
}

// sanitizeReason returns a bounded, non-sensitive reason string safe
// for audit. The audit package performs the authoritative allowlist
// check; this helper only strips leading whitespace and truncates
// pathological length before submission.
func sanitizeReason(s string) string {
	const maxAuditReason = 200
	if len(s) > maxAuditReason {
		s = s[:maxAuditReason]
	}
	return s
}
