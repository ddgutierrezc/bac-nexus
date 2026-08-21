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
func (f *fakeCredentialStore) Set(profile string, secret []byte) error { return f.err }
func (f *fakeCredentialStore) Delete(profile string) error             { return f.err }

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
	err    error
	calls  int
	gotCtx context.Context
}

func (f *fakeRecoveryCoordinator) Recover(ctx context.Context) error {
	f.calls++
	f.gotCtx = ctx
	return f.err
}

// fakeLeaseStore wraps a real source.LeaseStore. The freshness check is
// performed by the service using the Resolver, so the fake only needs
// to delegate to the real store.
type fakeLeaseStore struct {
	store *source.LeaseStore
}

func (f *fakeLeaseStore) Acquire(snap *source.Snapshot, selection catalog.Candidate, policy source.ClientPolicy) (source.Cursor, error) {
	return f.store.Acquire(snap, selection, policy)
}

func (f *fakeLeaseStore) Lookup(cursor source.Cursor) (catalog.Candidate, error) {
	return f.store.Lookup(cursor)
}

func (f *fakeLeaseStore) OpenReader(cursor source.Cursor, selection catalog.Candidate, policy source.ClientPolicy) (*source.LeaseReader, error) {
	return f.store.OpenReader(cursor, selection, policy)
}

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

// ---------------------------------------------------------------------------
// Test input builders: one canonical constructor per surface.
// ---------------------------------------------------------------------------

const testProfile = "test-profile"

// freshCandidate returns a canonical catalog candidate used by every
// freshness and source-read test.
func freshCandidate() catalog.Candidate {
	return catalog.Candidate{
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
}

// allowCatalogDecision is the canonical allow decision for catalog resolve.
func allowCatalogDecision() security.Decision_ {
	return security.Decision_{
		Selector: security.SelectorResolveCatalog,
		Class:    security.CapabilityCatalogResolve,
		Target:   security.TargetIBMiCatalog,
		Decision: security.DecisionAllow,
		Reason:   "allowlisted selector and matching target class",
	}
}

// allowSourceDecision is the canonical allow decision for source read.
func allowSourceDecision() security.Decision_ {
	return security.Decision_{
		Selector: security.SelectorReadSource,
		Class:    security.CapabilitySourceRead,
		Target:   security.TargetIBMiSource,
		Decision: security.DecisionAllow,
		Reason:   "allowlisted selector and matching target class",
	}
}

// fixedClock returns a deterministic time source for service tests.
func fixedClock() func() time.Time {
	return func() time.Time { return time.Unix(0, 0).UTC() }
}

// makeFiftyCandidates returns 50 distinct catalog candidates.
func makeFiftyCandidates() []catalog.Candidate {
	out := make([]catalog.Candidate, 0, 50)
	for i := 0; i < 50; i++ {
		out = append(out, freshCandidate())
	}
	return out
}

// makeFiftyOneCandidates returns 51 candidates to trigger the 50-cap.
func makeFiftyOneCandidates() []catalog.Candidate {
	out := makeFiftyCandidates()
	out = append(out, freshCandidate())
	return out
}

// serviceTestInput bundles the canonical valid input set for service
// tests that do not need a lease store.
type serviceTestInput struct {
	svc      *Service
	creds    *fakeCredentialStore
	authz    *fakeAuthorizer
	aud      *fakeAuditor
	resolver *fakeCatalogResolver
	acquirer *fakeAcquirer
	recovery *fakeRecoveryCoordinator
}

func newServiceTestInput() serviceTestInput {
	creds := &fakeCredentialStore{secret: []byte("test-secret")}
	authz := &fakeAuthorizer{decision: allowCatalogDecision()}
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
		Profile:     testProfile,
		Now:         fixedClock(),
	})
	return serviceTestInput{
		svc:      svc,
		creds:    creds,
		authz:    authz,
		aud:      aud,
		resolver: resolver,
		acquirer: acquirer,
		recovery: recovery,
	}
}

// readServiceTestInput bundles a service configured for source-read tests,
// including a lease store that wraps a real source.LeaseStore.
type readServiceTestInput struct {
	svc      *Service
	creds    *fakeCredentialStore
	authz    *fakeAuthorizer
	aud      *fakeAuditor
	resolver *fakeCatalogResolver
	acquirer *fakeAcquirer
	recovery *fakeRecoveryCoordinator
	leases   *fakeLeaseStore
	lease    *source.LeaseStore
}

// newReadServiceTestInput builds a service with a lease store and
// optional overrides applied to the input fields before startup.
func newReadServiceTestInput(t *testing.T, candidate catalog.Candidate, currentCatalog []catalog.Candidate) readServiceTestInput {
	t.Helper()
	lease := source.NewLeaseStoreForTest(time.Now, nil)
	creds := &fakeCredentialStore{secret: []byte("test")}
	authz := &fakeAuthorizer{decision: allowSourceDecision()}
	aud := &fakeAuditor{}
	resolver := &fakeCatalogResolver{candidates: currentCatalog}
	acquirer := &fakeAcquirer{}
	recovery := &fakeRecoveryCoordinator{}
	leases := &fakeLeaseStore{store: lease}
	svc := NewService(ServiceDeps{
		Credentials: creds,
		Authorizer:  authz,
		Auditor:     aud,
		Resolver:    resolver,
		Acquirer:    acquirer,
		Recovery:    recovery,
		Leases:      leases,
		Profile:     testProfile,
		Now:         fixedClock(),
	})
	if err := svc.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}
	_ = candidate
	return readServiceTestInput{
		svc:      svc,
		creds:    creds,
		authz:    authz,
		aud:      aud,
		resolver: resolver,
		acquirer: acquirer,
		recovery: recovery,
		leases:   leases,
		lease:    lease,
	}
}

// mintCursor mints a cursor bound to the supplied candidate and policy
// against the supplied lease store. It fails the test on error.
func mintCursor(t *testing.T, lease *source.LeaseStore, content []byte, candidate catalog.Candidate, policy source.ClientPolicy) source.Cursor {
	t.Helper()
	snap, err := source.NewSnapshot(content)
	if err != nil {
		t.Fatalf("NewSnapshot error = %v", err)
	}
	cursor, err := lease.Acquire(snap, candidate, policy)
	if err != nil {
		t.Fatalf("Acquire error = %v", err)
	}
	return cursor
}

// ---------------------------------------------------------------------------
// Startup tests
// ---------------------------------------------------------------------------

// TestServiceStartupInvokesRecoveryBeforeAvailability proves the service
// calls RecoveryCoordinator.Recover during startup and that the service
// is unavailable until recovery succeeds.
func TestServiceStartupInvokesRecoveryBeforeAvailability(t *testing.T) {
	in := newServiceTestInput()

	if err := in.svc.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}
	if in.recovery.calls != 1 {
		t.Fatalf("recovery calls = %d, want 1", in.recovery.calls)
	}
	if !in.svc.Available() {
		t.Fatal("service is not available after successful recovery")
	}
}

// TestServiceStartupFailsClosedOnRecoveryError proves a recovery failure
// keeps the service unavailable and surfaces the failure.
func TestServiceStartupFailsClosedOnRecoveryError(t *testing.T) {
	in := newServiceTestInput()
	in.recovery.err = errors.New("simulated recovery failure")

	err := in.svc.Startup(context.Background())
	if err == nil {
		t.Fatal("Startup error = nil, want recovery error")
	}
	if in.svc.Available() {
		t.Fatal("service is available after failed recovery; should be unavailable")
	}
}

// TestServiceStartupRejectsContextCancellation proves startup honors ctx.
func TestServiceStartupRejectsContextCancellation(t *testing.T) {
	in := newServiceTestInput()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := in.svc.Startup(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Startup error = %v, want context.Canceled", err)
	}
	if in.recovery.calls != 0 {
		t.Fatalf("recovery called %d times after cancellation, want 0", in.recovery.calls)
	}
	if in.svc.Available() {
		t.Fatal("service is available after cancelled startup")
	}
}

// ---------------------------------------------------------------------------
// Catalog resolution tests
// ---------------------------------------------------------------------------

// TestServiceResolveCatalogBounds covers both the 50-result cap and
// the 50-or-fewer success path. The two cases exercise the same
// production path with different input cardinalities.
func TestServiceResolveCatalogBounds(t *testing.T) {
	tests := []struct {
		name       string
		candidates []catalog.Candidate
		wantErr    error
		wantLen    int
	}{
		{name: "fifty one is capped", candidates: makeFiftyOneCandidates(), wantErr: catalog.ErrCandidateLimit},
		{name: "fifty is allowed", candidates: makeFiftyCandidates(), wantLen: 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := newServiceTestInput()
			if err := in.svc.Startup(context.Background()); err != nil {
				t.Fatalf("Startup error = %v", err)
			}
			in.resolver.candidates = tt.candidates
			in.authz.decision = allowCatalogDecision()
			query, err := catalog.BuildQuery("PISA061", "")
			if err != nil {
				t.Fatalf("BuildQuery error = %v", err)
			}
			got, err := in.svc.ResolveCatalog(context.Background(), query, security.SelectorResolveCatalog)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ResolveCatalog error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveCatalog error = %v", err)
			}
			if len(got) != tt.wantLen {
				t.Fatalf("len(candidates) = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

// TestServiceResolveCatalogRejectsCredentialsUnavailable proves that the
// service refuses catalog resolution before any remote work when the
// native credential store cannot return a usable secret.
func TestServiceResolveCatalogRejectsCredentialsUnavailable(t *testing.T) {
	in := newServiceTestInput()
	if err := in.svc.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}
	authzCalled := 0
	svc := NewService(ServiceDeps{
		Credentials: in.creds,
		Authorizer:  &countingAuthorizer{decision: allowCatalogDecision(), onCall: func() { authzCalled++ }},
		Auditor:     in.aud,
		Resolver:    in.resolver,
		Acquirer:    in.acquirer,
		Recovery:    in.recovery,
		Profile:     testProfile,
		Now:         fixedClock(),
	})
	if err := svc.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}
	in.creds.err = credential.ErrCredentialsUnavailable

	query, err := catalog.BuildQuery("PISA061", "")
	if err != nil {
		t.Fatalf("BuildQuery error = %v", err)
	}
	_, err = svc.ResolveCatalog(context.Background(), query, security.SelectorResolveCatalog)
	if !errors.Is(err, credential.ErrCredentialsUnavailable) {
		t.Fatalf("ResolveCatalog error = %v, want ErrCredentialsUnavailable", err)
	}
	if authzCalled != 0 {
		t.Fatalf("authorizer was called %d times after credential failure, want 0", authzCalled)
	}
	if in.resolver.queries != 0 {
		t.Fatalf("resolver was called %d times after credential failure, want 0", in.resolver.queries)
	}
}

// TestServiceResolveCatalogRejectsPolicyDenial proves the service fails
// closed on a denied selector before any resolver or remote work.
func TestServiceResolveCatalogRejectsPolicyDenial(t *testing.T) {
	in := newServiceTestInput()
	if err := in.svc.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}
	in.authz.decision = security.Decision_{
		Selector: security.SelectorResolveCatalog,
		Decision: security.DecisionDeny,
		Reason:   "selector not allowlisted",
	}

	query, err := catalog.BuildQuery("PISA061", "")
	if err != nil {
		t.Fatalf("BuildQuery error = %v", err)
	}
	_, err = in.svc.ResolveCatalog(context.Background(), query, security.Selector("rogue"))
	if err == nil {
		t.Fatal("ResolveCatalog error = nil, want denial")
	}
	if in.resolver.queries != 0 {
		t.Fatalf("resolver was called %d times after policy denial, want 0", in.resolver.queries)
	}
}

// TestServiceRejectsContextCancellationOnResolveCatalog proves the catalog
// resolve path honors context cancellation before any resolver call.
func TestServiceRejectsContextCancellationOnResolveCatalog(t *testing.T) {
	in := newServiceTestInput()
	if err := in.svc.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	query, err := catalog.BuildQuery("PISA061", "")
	if err != nil {
		t.Fatalf("BuildQuery error = %v", err)
	}
	_, err = in.svc.ResolveCatalog(ctx, query, security.SelectorResolveCatalog)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ResolveCatalog error = %v, want context.Canceled", err)
	}
	if in.resolver.queries != 0 {
		t.Fatalf("resolver was called %d times after cancellation, want 0", in.resolver.queries)
	}
}

// ---------------------------------------------------------------------------
// Source read tests
// ---------------------------------------------------------------------------

func TestServiceReadSelectedSourceAcquiresFirstPageWithoutCursor(t *testing.T) {
	candidate := freshCandidate()
	in := newReadServiceTestInput(t, candidate, []catalog.Candidate{candidate})
	in.acquirer.snapshot, _ = source.NewSnapshot([]byte("first\nsecond\n"))
	page, err := in.svc.ReadSelectedSource(context.Background(), candidate, "", source.Range{StartLine: 1, MaxLines: 1})
	if err != nil {
		t.Fatalf("first page error = %v", err)
	}
	if page.LineCount != 1 || page.NextStartLine != 2 || in.acquirer.calls != 1 {
		t.Fatalf("page=%+v acquireCalls=%d", page, in.acquirer.calls)
	}
}

// TestServiceReadSelectedSourceRejectsWhenUnavailable proves the service
// refuses source reads before recovery has been completed.
func TestServiceReadSelectedSourceRejectsWhenUnavailable(t *testing.T) {
	in := newServiceTestInput()
	_, err := in.svc.ReadSelectedSource(context.Background(), freshCandidate(), "ignored-cursor", source.Range{StartLine: 1, MaxLines: 10})
	if err == nil {
		t.Fatal("ReadSelectedSource error = nil, want unavailability")
	}
}

// TestServiceReadSelectedSourceRejectsCredentialsUnavailable proves a
// missing credential fails closed before any source lease work.
func TestServiceReadSelectedSourceRejectsCredentialsUnavailable(t *testing.T) {
	in := newServiceTestInput()
	if err := in.svc.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}
	in.creds.err = credential.ErrCredentialsUnavailable
	_, err := in.svc.ReadSelectedSource(context.Background(), freshCandidate(), "cursor", source.Range{StartLine: 1, MaxLines: 10})
	if !errors.Is(err, credential.ErrCredentialsUnavailable) {
		t.Fatalf("ReadSelectedSource error = %v, want ErrCredentialsUnavailable", err)
	}
	if in.acquirer.calls != 0 {
		t.Fatalf("acquirer was called %d times after credential failure, want 0", in.acquirer.calls)
	}
}

// TestServiceReadSelectedSourceRejectsPolicyDenial proves a denied
// selector for source read fails closed before any lease or remote work.
func TestServiceReadSelectedSourceRejectsPolicyDenial(t *testing.T) {
	in := newServiceTestInput()
	if err := in.svc.Startup(context.Background()); err != nil {
		t.Fatalf("Startup error = %v", err)
	}
	in.authz.decision = security.Decision_{Decision: security.DecisionDeny, Reason: "selector not allowlisted"}

	_, err := in.svc.ReadSelectedSource(context.Background(), freshCandidate(), "cursor", source.Range{StartLine: 1, MaxLines: 10})
	if err == nil {
		t.Fatal("ReadSelectedSource error = nil, want denial")
	}
	if in.acquirer.calls != 0 {
		t.Fatalf("acquirer was called %d times after policy denial, want 0", in.acquirer.calls)
	}
}

// TestServiceReadSelectedSourceFreshnessCases covers the two freshness
// outcomes: a matching coordinate continues the snapshot, and a changed
// coordinate returns ErrStaleCoordinate. Both cases share the same
// acquire/open path and the same freshness check.
func TestServiceReadSelectedSourceFreshnessCases(t *testing.T) {
	tests := []struct {
		name           string
		currentCatalog []catalog.Candidate
		wantErr        error
	}{
		{
			name:           "matching coordinate continues the snapshot",
			currentCatalog: []catalog.Candidate{freshCandidate()},
			wantErr:        nil,
		},
		{
			name: "changed coordinate returns stale_coordinate",
			currentCatalog: func() []catalog.Candidate {
				changed := freshCandidate()
				changed.ProductionLibrary = "PRODLIB2"
				return []catalog.Candidate{changed}
			}(),
			wantErr: source.ErrStaleCoordinate,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := freshCandidate()
			in := newReadServiceTestInput(t, original, tt.currentCatalog)
			cursor := mintCursor(t, in.lease, []byte("line1\nline2\n"), original, source.ClientPolicy(testProfile))
			_, err := in.svc.ReadSelectedSource(context.Background(), original, string(cursor), source.Range{StartLine: 1, MaxLines: 200})
			if tt.wantErr == nil && err != nil {
				t.Fatalf("ReadSelectedSource error = %v, want nil", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("ReadSelectedSource error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestServiceReadSelectedSourceHonorsPageByteBound proves a range that
// would exceed the marshaled response cap returns response_too_large and
// no partial content. The service should fail closed with no snapshot
// data leaked.
func TestServiceReadSelectedSourceHonorsPageByteBound(t *testing.T) {
	candidate := freshCandidate()
	in := newReadServiceTestInput(t, candidate, []catalog.Candidate{candidate})
	// Build a member that is well within member-size limits but whose
	// single-line marshaled response exceeds MaxPageBytes.
	huge := make([]byte, source.MaxPageBytes+1024)
	for i := range huge {
		huge[i] = 'A'
	}
	cursor := mintCursor(t, in.lease, huge, candidate, source.ClientPolicy(testProfile))

	page, err := in.svc.ReadSelectedSource(context.Background(), candidate, string(cursor), source.Range{StartLine: 1, MaxLines: 1})
	if !errors.Is(err, source.ErrResponseTooLarge) {
		t.Fatalf("ReadSelectedSource error = %v, want ErrResponseTooLarge", err)
	}
	if len(page.Lines) != 0 {
		t.Fatalf("page contained %d lines after response_too_large, want 0", len(page.Lines))
	}
}

// ---------------------------------------------------------------------------
// Audit tests
// ---------------------------------------------------------------------------

// TestServiceAuditsCatalogResolution covers both the allow and deny
// audit outcomes for catalog resolve. Each case asserts the matching
// result class and the requested/returned counts.
func TestServiceAuditsCatalogResolution(t *testing.T) {
	tests := []struct {
		name         string
		decision     security.Decision_
		wantResult   audit.ResultClass
		wantReturned int
	}{
		{
			name:         "successful resolution records allow",
			decision:     allowCatalogDecision(),
			wantResult:   audit.ResultClassAllow,
			wantReturned: 50,
		},
		{
			name:         "denied resolution records deny",
			decision:     security.Decision_{Decision: security.DecisionDeny, Reason: "selector not allowlisted"},
			wantResult:   audit.ResultClassDeny,
			wantReturned: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := newServiceTestInput()
			if err := in.svc.Startup(context.Background()); err != nil {
				t.Fatalf("Startup error = %v", err)
			}
			in.resolver.candidates = makeFiftyCandidates()
			in.authz.decision = tt.decision

			query, err := catalog.BuildQuery("PISA061", "")
			if err != nil {
				t.Fatalf("BuildQuery error = %v", err)
			}
			_, _ = in.svc.ResolveCatalog(context.Background(), query, security.SelectorResolveCatalog)
			if len(in.aud.events) == 0 {
				t.Fatal("no audit events recorded")
			}
			last := in.aud.events[len(in.aud.events)-1]
			if last.Result != tt.wantResult {
				t.Fatalf("event result = %q, want %q", last.Result, tt.wantResult)
			}
			if last.Returned != tt.wantReturned {
				t.Fatalf("event returned = %d, want %d", last.Returned, tt.wantReturned)
			}
		})
	}
}
