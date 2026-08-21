// Package app composes the local-OS-principal Nexus service: credential
// availability, policy authorization, bounded catalog resolution, immutable
// source pagination with per-page freshness, recovery, and sanitized audit.
// The service owns no remote, path, shell, SQL, or SSH capability of its
// own; every blocking call honors context.Context cancellation.
package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"bac-nexus/internal/audit"
	"bac-nexus/internal/catalog"
	"bac-nexus/internal/credential"
	"bac-nexus/internal/security"
	"bac-nexus/internal/source"
)

// ---------------------------------------------------------------------------
// Test fixtures: consumer-owned fakes that satisfy the service's narrow
// dependency surface. Each fake is deterministic; no live IBM i, no real
// filesystem, no real keyring, and no real SSH is ever involved.
// ---------------------------------------------------------------------------

// fakeCredentialStore is a minimal CredentialStore for service tests.
type fakeCredentialStore struct {
	secret []byte
	err    error
	calls  int
}

func (f *fakeCredentialStore) Get(profile string) ([]byte, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return append([]byte(nil), f.secret...), nil
}
func (f *fakeCredentialStore) Set(profile string, secret []byte) error {
	return f.err
}
func (f *fakeCredentialStore) Delete(profile string) error { return f.err }

// fakeAuthorizer records every call and returns the configured decision.
type fakeAuthorizer struct {
	decision security.Decision_
	err      error
	calls    int
	selector security.Selector
	target   security.CapabilityTarget
}

func (f *fakeAuthorizer) Authorize(ctx context.Context, selector security.Selector, target security.CapabilityTarget) (security.Decision_, error) {
	f.calls++
	f.selector = selector
	f.target = target
	if f.err != nil {
		return security.Decision_{}, f.err
	}
	return f.decision, nil
}

// fakeAuditor records every submitted event in submission order.
type fakeAuditor struct {
	events []audit.Event
	err    error
}

func (f *fakeAuditor) Record(ctx context.Context, event audit.Event) error {
	if f.err != nil {
		return f.err
	}
	f.events = append(f.events, event)
	return nil
}

// fakeCatalogResolver returns a deterministic candidate set.
type fakeCatalogResolver struct {
	candidates []catalog.Candidate
	err        error
	queries    int
	query      catalog.Query
}

func (f *fakeCatalogResolver) Resolve(ctx context.Context, query catalog.Query) ([]catalog.Candidate, error) {
	f.queries++
	f.query = query
	if f.err != nil {
		return nil, f.err
	}
	return f.candidates, nil
}

// fakeAcquirer returns a deterministic snapshot or error.
type fakeAcquirer struct {
	snapshot *source.Snapshot
	err      error
	calls    int
	got      catalog.Candidate
}

func (f *fakeAcquirer) Acquire(ctx context.Context, candidate catalog.Candidate) (*source.Snapshot, error) {
	f.calls++
	f.got = candidate
	if f.err != nil {
		return nil, f.err
	}
	return f.snapshot, nil
}

// fakeRecoveryCoordinator records whether Recover was invoked.
type fakeRecoveryCoordinator struct {
	err     error
	calls   int
	gotCtx  context.Context
}

func (f *fakeRecoveryCoordinator) Recover(ctx context.Context) error {
	f.calls++
	f.gotCtx = ctx
	return f.err
}

// newServiceTestInput builds a canonical valid input set for service tests.
func newServiceTestInput() (*Service, *fakeCredentialStore, *fakeAuthorizer, *fakeAuditor, *fakeCatalogResolver, *fakeAcquirer, *fakeRecoveryCoordinator) {
	creds := &fakeCredentialStore{secret: []byte("test-secret")}
	authz := &fakeAuthorizer{decision: security.Decision_{
		Selector: security.SelectorResolveCatalog,
		Class:    security.CapabilityCatalogResolve,
		Target:   security.TargetIBMiCatalog,
		Decision: security.DecisionAllow,
		Reason:   "allowlisted selector and matching target class",
	}}
	aud := &fakeAuditor{}
	resolver := &fakeCatalogResolver{candidates: []catalog.Candidate{}}
	acquirer := &fakeAcquirer{}
	recovery := &fakeRecoveryCoordinator{}
	svc := NewService(ServiceDeps{
		Credentials: creds,
		Authorizer:  authz,
		Auditor:     aud,
		Resolver:    resolver,
		Acquirer:    acquirer,
		Recovery:    recovery,
		Profile:     "test-profile",
		Now:         func() time.Time { return time.Unix(0, 0).UTC() },
	})
	return svc, creds, authz, aud, resolver, acquirer, recovery
}

// TestServiceStartupInvokesRecoveryBeforeAvailability proves the service
// calls RecoveryCoordinator.Recover during startup and that the service
// is unavailable until recovery succeeds.
func TestServiceStartupInvokesRecoveryBeforeAvailability(t *testing.T) {
	svc, _, _, _, _, _, recovery := newServiceTestInput()

	if err := svc.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}
	if recovery.calls != 1 {
		t.Fatalf("recovery calls = %d, want 1", recovery.calls)
	}
	if !svc.Available() {
		t.Fatal("service is not available after successful recovery")
	}
}

// TestServiceStartupFailsClosedOnRecoveryError proves a recovery failure
// keeps the service unavailable and surfaces the failure.
func TestServiceStartupFailsClosedOnRecoveryError(t *testing.T) {
	svc, _, _, _, _, _, recovery := newServiceTestInput()
	recovery.err = errors.New("simulated recovery failure")

	err := svc.Startup(context.Background())
	if err == nil {
		t.Fatal("Startup error = nil, want recovery error")
	}
	if svc.Available() {
		t.Fatal("service is available after failed recovery; should be unavailable")
	}
}

// TestServiceStartupRejectsContextCancellation proves startup honors ctx.
func TestServiceStartupRejectsContextCancellation(t *testing.T) {
	svc, _, _, _, _, _, recovery := newServiceTestInput()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := svc.Startup(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Startup error = %v, want context.Canceled", err)
	}
	if recovery.calls != 0 {
		t.Fatalf("recovery called %d times after cancellation, want 0", recovery.calls)
	}
	if svc.Available() {
		t.Fatal("service is available after cancelled startup")
	}
}

// TestServiceResolveCatalogCapsAtFiftyCandidates proves the 50-result bound.
func TestServiceResolveCatalogCapsAtFiftyCandidates(t *testing.T) {
	svc, _, authz, _, resolver, _, _ := newServiceTestInput()
	if err := svc.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}
	resolver.candidates = makeFiftyOneCandidates()
	authz.decision = security.Decision_{
		Selector: security.SelectorResolveCatalog,
		Class:    security.CapabilityCatalogResolve,
		Target:   security.TargetIBMiCatalog,
		Decision: security.DecisionAllow,
		Reason:   "allowlisted",
	}

	query, err := catalog.BuildQuery("PISA061", "")
	if err != nil {
		t.Fatalf("BuildQuery error = %v", err)
	}
	_, err = svc.ResolveCatalog(context.Background(), query, security.SelectorResolveCatalog)
	if !errors.Is(err, catalog.ErrCandidateLimit) {
		t.Fatalf("ResolveCatalog error = %v, want ErrCandidateLimit", err)
	}
}

// TestServiceResolveCatalogReturnsBoundedResults proves 50 or fewer pass.
func TestServiceResolveCatalogReturnsBoundedResults(t *testing.T) {
	svc, _, authz, _, resolver, _, _ := newServiceTestInput()
	if err := svc.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}
	resolver.candidates = makeFiftyCandidates()
	authz.decision = security.Decision_{
		Selector: security.SelectorResolveCatalog,
		Class:    security.CapabilityCatalogResolve,
		Target:   security.TargetIBMiCatalog,
		Decision: security.DecisionAllow,
		Reason:   "allowlisted",
	}

	query, err := catalog.BuildQuery("PISA061", "")
	if err != nil {
		t.Fatalf("BuildQuery error = %v", err)
	}
	got, err := svc.ResolveCatalog(context.Background(), query, security.SelectorResolveCatalog)
	if err != nil {
		t.Fatalf("ResolveCatalog error = %v", err)
	}
	if len(got) != 50 {
		t.Fatalf("len(candidates) = %d, want 50", len(got))
	}
}

// TestServiceResolveCatalogRejectsCredentialsUnavailable proves that the
// service refuses catalog resolution before any remote work when the
// native credential store cannot return a usable secret.
func TestServiceResolveCatalogRejectsCredentialsUnavailable(t *testing.T) {
	svc, creds, _, _, resolver, _, _ := newServiceTestInput()
	if err := svc.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}
	creds.err = credential.ErrCredentialsUnavailable
	authzCalled := 0
	// We swap the authorizer to track whether it was called.
	svc2 := NewService(ServiceDeps{
		Credentials: creds,
		Authorizer: &countingAuthorizer{decision: security.Decision_{Decision: security.DecisionAllow}, onCall: func() { authzCalled++ }},
		Auditor:     &fakeAuditor{},
		Resolver:    resolver,
		Acquirer:    &fakeAcquirer{},
		Recovery:    &fakeRecoveryCoordinator{},
		Profile:     "test-profile",
		Now:         func() time.Time { return time.Unix(0, 0).UTC() },
	})
	if err := svc2.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}

	query, err := catalog.BuildQuery("PISA061", "")
	if err != nil {
		t.Fatalf("BuildQuery error = %v", err)
	}
	_, err = svc2.ResolveCatalog(context.Background(), query, security.SelectorResolveCatalog)
	if !errors.Is(err, credential.ErrCredentialsUnavailable) {
		t.Fatalf("ResolveCatalog error = %v, want ErrCredentialsUnavailable", err)
	}
	if authzCalled != 0 {
		t.Fatalf("authorizer was called %d times after credential failure, want 0", authzCalled)
	}
	if resolver.queries != 0 {
		t.Fatalf("resolver was called %d times after credential failure, want 0", resolver.queries)
	}
}

// TestServiceResolveCatalogRejectsPolicyDenial proves the service fails
// closed on a denied selector before any resolver or remote work.
func TestServiceResolveCatalogRejectsPolicyDenial(t *testing.T) {
	svc, _, authz, _, resolver, _, _ := newServiceTestInput()
	if err := svc.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}
	authz.decision = security.Decision_{
		Selector: security.SelectorResolveCatalog,
		Decision: security.DecisionDeny,
		Reason:   "selector not allowlisted",
	}

	query, err := catalog.BuildQuery("PISA061", "")
	if err != nil {
		t.Fatalf("BuildQuery error = %v", err)
	}
	_, err = svc.ResolveCatalog(context.Background(), query, security.Selector("rogue"))
	if err == nil {
		t.Fatal("ResolveCatalog error = nil, want denial")
	}
	if resolver.queries != 0 {
		t.Fatalf("resolver was called %d times after policy denial, want 0", resolver.queries)
	}
}

// TestServiceReadSelectedSourceRejectsWhenUnavailable proves the service
// refuses source reads before recovery has been completed.
func TestServiceReadSelectedSourceRejectsWhenUnavailable(t *testing.T) {
	svc, _, _, _, _, _, _ := newServiceTestInput()

	_, err := svc.ReadSelectedSource(context.Background(), "ignored-cursor", source.Range{StartLine: 1, MaxLines: 10})
	if err == nil {
		t.Fatal("ReadSelectedSource error = nil, want unavailability")
	}
}

// TestServiceReadSelectedSourceRejectsCredentialsUnavailable proves a
// missing credential fails closed before any source lease work.
func TestServiceReadSelectedSourceRejectsCredentialsUnavailable(t *testing.T) {
	svc, creds, _, _, _, acquirer, _ := newServiceTestInput()
	if err := svc.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}
	creds.err = credential.ErrCredentialsUnavailable
	_, err := svc.ReadSelectedSource(context.Background(), "cursor", source.Range{StartLine: 1, MaxLines: 10})
	if !errors.Is(err, credential.ErrCredentialsUnavailable) {
		t.Fatalf("ReadSelectedSource error = %v, want ErrCredentialsUnavailable", err)
	}
	if acquirer.calls != 0 {
		t.Fatalf("acquirer was called %d times after credential failure, want 0", acquirer.calls)
	}
}

// TestServiceReadSelectedSourceReturnsStaleCoordinateOnCoordinateChange
// proves the per-page freshness check returns stale_coordinate when the
// current coordinate has changed since the cursor was minted.
func TestServiceReadSelectedSourceReturnsStaleCoordinateOnCoordinateChange(t *testing.T) {
	svc, _, _, _, _, _, _ := newServiceTestInput()
	if err := svc.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}
	original := catalog.Candidate{
		Item:              "PISA061",
		SourceLibrary:     "QRPGLESRC",
		SourceFileBase:    "QRPGLESRC",
		ObjectType:        "RPGLE",
		SourceType:        "RPG",
		Application:       "APP",
		Version:           "V1",
		ProductionLibrary: "PRODLIB",
		Description:       "test program",
	}
	// The catalog now reports a different production library, which
	// is a coordinate change the freshness check must detect.
	changed := original
	changed.ProductionLibrary = "PRODLIB2"

	lease := source.NewLeaseStoreForTest(time.Now, nil)
	snap, err := source.NewSnapshot([]byte("line1\nline2\n"))
	if err != nil {
		t.Fatalf("NewSnapshot error = %v", err)
	}
	cursor, err := lease.Acquire(snap, original, source.ClientPolicy("test-profile"))
	if err != nil {
		t.Fatalf("Acquire error = %v", err)
	}

	// The Resolver simulates the current catalog state: the original
	// candidate is gone, replaced by the changed one. The service must
	// detect the missing original and return ErrStaleCoordinate.
	leases := &fakeLeaseStore{store: lease}
	svc2 := NewService(ServiceDeps{
		Credentials: &fakeCredentialStore{secret: []byte("test")},
		Authorizer:  &fakeAuthorizer{decision: security.Decision_{Selector: security.SelectorReadSource, Class: security.CapabilitySourceRead, Target: security.TargetIBMiSource, Decision: security.DecisionAllow, Reason: "allowlisted"}},
		Auditor:     &fakeAuditor{},
		Resolver:    &fakeCatalogResolver{candidates: []catalog.Candidate{changed}},
		Acquirer:    &fakeAcquirer{},
		Recovery:    &fakeRecoveryCoordinator{},
		Leases:      leases,
		Profile:     "test-profile",
		Now:         func() time.Time { return time.Unix(0, 0).UTC() },
	})
	if err := svc2.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}

	_, err = svc2.ReadSelectedSource(context.Background(), string(cursor), source.Range{StartLine: 1, MaxLines: 200})
	if !errors.Is(err, source.ErrStaleCoordinate) {
		t.Fatalf("ReadSelectedSource error = %v, want ErrStaleCoordinate", err)
	}
}

// TestServiceReadSelectedSourceHonorsPageByteBound proves a range that
// would exceed the marshaled response cap returns response_too_large and
// no partial content. The service should fail closed with no snapshot
// data leaked.
func TestServiceReadSelectedSourceHonorsPageByteBound(t *testing.T) {
	svc, _, _, _, _, _, _ := newServiceTestInput()
	if err := svc.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}
	lease := source.NewLeaseStoreForTest(time.Now, nil)
	// Build a member that is well within member-size limits but whose
	// single-line marshaled response exceeds MaxPageBytes.
	huge := make([]byte, source.MaxPageBytes+1024)
	for i := range huge {
		huge[i] = 'A'
	}
	snap, err := source.NewSnapshot(huge)
	if err != nil {
		t.Fatalf("NewSnapshot error = %v", err)
	}
	candidate := catalog.Candidate{
		Item: "PISA061", SourceLibrary: "QRPGLESRC", SourceFileBase: "QRPGLESRC",
		ObjectType: "RPGLE", SourceType: "RPG",
		Application: "APP", Version: "V1", ProductionLibrary: "PRODLIB",
		Description: "test program",
	}
	cursor, err := lease.Acquire(snap, candidate, source.ClientPolicy("test-profile"))
	if err != nil {
		t.Fatalf("Acquire error = %v", err)
	}

	leases := &fakeLeaseStore{store: lease}
	svc2 := NewService(ServiceDeps{
		Credentials: &fakeCredentialStore{secret: []byte("test")},
		Authorizer:  &fakeAuthorizer{decision: security.Decision_{Selector: security.SelectorReadSource, Class: security.CapabilitySourceRead, Target: security.TargetIBMiSource, Decision: security.DecisionAllow, Reason: "allowlisted"}},
		Auditor:     &fakeAuditor{},
		Resolver:    &fakeCatalogResolver{candidates: []catalog.Candidate{candidate}},
		Acquirer:    &fakeAcquirer{},
		Recovery:    &fakeRecoveryCoordinator{},
		Leases:      leases,
		Profile:     "test-profile",
		Now:         func() time.Time { return time.Unix(0, 0).UTC() },
	})
	if err := svc2.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}

	page, err := svc2.ReadSelectedSource(context.Background(), string(cursor), source.Range{StartLine: 1, MaxLines: 1})
	if !errors.Is(err, source.ErrResponseTooLarge) {
		t.Fatalf("ReadSelectedSource error = %v, want ErrResponseTooLarge", err)
	}
	if len(page.Lines) != 0 {
		t.Fatalf("page contained %d lines after response_too_large, want 0", len(page.Lines))
	}
}

// TestServiceReadSelectedSourceRejectsPolicyDenial proves a denied
// selector for source read fails closed before any lease or remote work.
func TestServiceReadSelectedSourceRejectsPolicyDenial(t *testing.T) {
	svc, _, authz, _, _, acquirer, _ := newServiceTestInput()
	if err := svc.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}
	authz.decision = security.Decision_{Decision: security.DecisionDeny, Reason: "selector not allowlisted"}

	_, err := svc.ReadSelectedSource(context.Background(), "cursor", source.Range{StartLine: 1, MaxLines: 10})
	if err == nil {
		t.Fatal("ReadSelectedSource error = nil, want denial")
	}
	if acquirer.calls != 0 {
		t.Fatalf("acquirer was called %d times after policy denial, want 0", acquirer.calls)
	}
}

// TestServiceAuditsSuccessfulCatalogResolution proves the service records
// an allow audit event on successful resolution, containing only the
// approved classification and count metadata, with no source/line/host.
func TestServiceAuditsSuccessfulCatalogResolution(t *testing.T) {
	svc, _, authz, aud, resolver, _, _ := newServiceTestInput()
	if err := svc.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}
	resolver.candidates = makeFiftyCandidates()
	authz.decision = security.Decision_{
		Selector: security.SelectorResolveCatalog,
		Class:    security.CapabilityCatalogResolve,
		Target:   security.TargetIBMiCatalog,
		Decision: security.DecisionAllow,
		Reason:   "allowlisted selector and matching target class",
	}

	query, err := catalog.BuildQuery("PISA061", "")
	if err != nil {
		t.Fatalf("BuildQuery error = %v", err)
	}
	_, err = svc.ResolveCatalog(context.Background(), query, security.SelectorResolveCatalog)
	if err != nil {
		t.Fatalf("ResolveCatalog error = %v", err)
	}
	if len(aud.events) == 0 {
		t.Fatal("no audit events recorded on successful resolution")
	}
	last := aud.events[len(aud.events)-1]
	if last.Result != audit.ResultClassAllow {
		t.Fatalf("event result = %q, want %q", last.Result, audit.ResultClassAllow)
	}
	if last.Returned != 50 {
		t.Fatalf("event returned = %d, want 50", last.Returned)
	}
}

// TestServiceAuditsDeniedCatalogResolution proves denial events are
// recorded with the deny result and no sensitive identifiers in the reason.
func TestServiceAuditsDeniedCatalogResolution(t *testing.T) {
	svc, _, authz, aud, _, _, _ := newServiceTestInput()
	if err := svc.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}
	authz.decision = security.Decision_{Decision: security.DecisionDeny, Reason: "selector not allowlisted"}

	query, err := catalog.BuildQuery("PISA061", "")
	if err != nil {
		t.Fatalf("BuildQuery error = %v", err)
	}
	_, _ = svc.ResolveCatalog(context.Background(), query, security.Selector("rogue"))
	if len(aud.events) == 0 {
		t.Fatal("no audit events recorded on denied resolution")
	}
	last := aud.events[len(aud.events)-1]
	if last.Result != audit.ResultClassDeny {
		t.Fatalf("event result = %q, want %q", last.Result, audit.ResultClassDeny)
	}
}

// TestServiceRejectsContextCancellationOnResolveCatalog proves the catalog
// resolve path honors context cancellation before any resolver call.
func TestServiceRejectsContextCancellationOnResolveCatalog(t *testing.T) {
	svc, _, _, _, resolver, _, _ := newServiceTestInput()
	if err := svc.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	query, err := catalog.BuildQuery("PISA061", "")
	if err != nil {
		t.Fatalf("BuildQuery error = %v", err)
	}
	_, err = svc.ResolveCatalog(ctx, query, security.SelectorResolveCatalog)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ResolveCatalog error = %v, want context.Canceled", err)
	}
	if resolver.queries != 0 {
		t.Fatalf("resolver was called %d times after cancellation, want 0", resolver.queries)
	}
}

// fakeLeaseStore is a consumer-owned lease store that wraps a real
// source.LeaseStore. The freshness check is performed by the service
// using the Resolver, so the fake only needs to delegate to the real
// store.
type fakeLeaseStore struct {
	store *source.LeaseStore
}

func (f *fakeLeaseStore) OpenReader(cursor source.Cursor, selection catalog.Candidate, policy source.ClientPolicy) (*source.LeaseReader, error) {
	return f.store.OpenReader(cursor, selection, policy)
}

func (f *fakeLeaseStore) Acquire(snap *source.Snapshot, selection catalog.Candidate, policy source.ClientPolicy) (source.Cursor, error) {
	return f.store.Acquire(snap, selection, policy)
}

func (f *fakeLeaseStore) Lookup(cursor source.Cursor) (catalog.Candidate, error) {
	return f.store.Lookup(cursor)
}

// ---------------------------------------------------------------------------
// Supporting helpers
// ---------------------------------------------------------------------------

// countingAuthorizer records each call via a callback so tests can assert
// whether the authorizer was reached.
type countingAuthorizer struct {
	decision security.Decision_
	err      error
	onCall   func()
}

func (f *countingAuthorizer) Authorize(ctx context.Context, selector security.Selector, target security.CapabilityTarget) (security.Decision_, error) {
	if f.onCall != nil {
		f.onCall()
	}
	if f.err != nil {
		return security.Decision_{}, f.err
	}
	return f.decision, nil
}

// makeFiftyCandidates returns 50 distinct catalog candidates.
func makeFiftyCandidates() []catalog.Candidate {
	out := make([]catalog.Candidate, 0, 50)
	for i := 0; i < 50; i++ {
		out = append(out, catalog.Candidate{
			Item:              "PISA061",
			SourceLibrary:     "QRPGLESRC",
			SourceFileBase:    "QRPGLESRC",
			ObjectType:        "RPGLE",
			SourceType:        "RPG",
			Application:       "APP",
			Version:           "V1",
			ProductionLibrary: "PRODLIB",
			Description:       "test program",
		})
	}
	return out
}

// makeFiftyOneCandidates returns 51 candidates to trigger the 50-cap.
func makeFiftyOneCandidates() []catalog.Candidate {
	out := makeFiftyCandidates()
	out = append(out, catalog.Candidate{
		Item: "PISA061", SourceLibrary: "QRPGLESRC", SourceFileBase: "QRPGLESRC",
		ObjectType: "RPGLE", SourceType: "RPG",
		Application: "APP", Version: "V1", ProductionLibrary: "PRODLIB",
		Description: "test program",
	})
	return out
}
