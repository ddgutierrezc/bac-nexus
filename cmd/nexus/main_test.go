// Package main wires the v1 MCP stdio server entry point.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"bac-nexus/internal/app"
	"bac-nexus/internal/audit"
	"bac-nexus/internal/catalog"
	"bac-nexus/internal/connectors/ibmi/catalogados"
	"bac-nexus/internal/credential"
	"bac-nexus/internal/mapepire"
	internalmcp "bac-nexus/internal/mcp"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/security"
	"bac-nexus/internal/source"
)

// all fakes are minimum implementations used only to satisfy the
// composition-root contracts. Each method records a single count or
// returns a single canned result.
type runnerStub struct {
	runErr   error
	runCalls int
	events   *[]string
}

func (r *runnerStub) Run(ctx context.Context) error {
	r.runCalls++
	if r.events != nil {
		*r.events = append(*r.events, "run")
	}
	return r.runErr
}

type closeStub struct {
	name   string
	events *[]string
	calls  int
}

func (s *closeStub) Close() error {
	s.calls++
	if s.events != nil {
		*s.events = append(*s.events, s.name)
	}
	return nil
}

type successfulRecovery struct {
	calls  int
	events *[]string
}

func (s *successfulRecovery) Recover(ctx context.Context) error {
	s.calls++
	if s.events != nil {
		*s.events = append(*s.events, "recovery")
	}
	return ctx.Err()
}

type failingRecovery struct {
	err   error
	calls int
}

func (f *failingRecovery) Recover(ctx context.Context) error {
	f.calls++
	if err := ctx.Err(); err != nil {
		return err
	}
	return f.err
}

type fakeCredentialStore struct{}

func (fakeCredentialStore) Get(string) ([]byte, error) { return []byte("test"), nil }
func (fakeCredentialStore) Set(string, []byte) error   { return nil }
func (fakeCredentialStore) Delete(string) error        { return nil }

type catalogCredentialStub struct {
	name   string
	gets   int
	secret []byte
}

func (s *catalogCredentialStub) Get(name string) ([]byte, error) {
	s.name, s.gets = name, s.gets+1
	return append([]byte(nil), s.secret...), nil
}
func (s *catalogCredentialStub) Set(string, []byte) error { return nil }
func (s *catalogCredentialStub) Delete(string) error      { return nil }

type catalogSessionStub struct {
	username string
	password []byte
	requests []mapepire.Request
	closed   int
}

func (s *catalogSessionStub) Connect(_ context.Context, username string, password []byte) error {
	s.username, s.password = username, append([]byte(nil), password...)
	return nil
}
func (s *catalogSessionStub) Call(_ context.Context, request mapepire.Request) (mapepire.Response, error) {
	s.requests = append(s.requests, request)
	switch request.Type {
	case mapepire.OperationPrepareSQLExecute:
		return mapepire.Response{Success: true, HasResults: true, IsDone: true, ContID: "cursor", Data: []map[string]json.RawMessage{{"ITEM": json.RawMessage(`"PISA061"`), "TIPO_DE_FUENTE": json.RawMessage(`"RPGLE"`), "TIPO_OBJETO": json.RawMessage(`"RPGLE"`), "BIBLIOTECA_FUENTES": json.RawMessage(`"SRCLIB"`), "ARCHIVO_FUENTES": json.RawMessage(`"Q"`)}}}, nil
	default:
		return mapepire.Response{Success: true}, nil
	}
}
func (s *catalogSessionStub) Close() error { s.closed++; return nil }

type fakeAuthorizer struct{}

func (fakeAuthorizer) Authorize(ctx context.Context, sel security.Selector, tgt security.CapabilityTarget) (security.Decision_, error) {
	return security.Decision_{Selector: sel, Target: tgt, Decision: security.DecisionAllow, Reason: "ok"}, nil
}

type fakeResolver struct{}

func (fakeResolver) Resolve(ctx context.Context, q catalog.Search) ([]catalog.Candidate, error) {
	return []catalog.Candidate{{Item: "PISA061", SourceLibrary: "QRPGLESRC", SourceFileBase: "QRPGLESRC", ObjectType: "RPGLE", SourceType: "RPG", Application: "APP", Version: "V1", ProductionLibrary: "PRODLIB"}}, nil
}

type fakeAcquirer struct{}

func (fakeAcquirer) Acquire(ctx context.Context, c catalog.Candidate) (*source.Snapshot, error) {
	return nil, errors.New("unused")
}

type fakeLeaseStore struct{}

func (fakeLeaseStore) Acquire(*source.Snapshot, catalog.Candidate, source.ClientPolicy) (source.Cursor, error) {
	return source.Cursor("test-cursor"), nil
}
func (fakeLeaseStore) Lookup(source.Cursor) (catalog.Candidate, error) {
	return catalog.Candidate{Item: "PISA061"}, nil
}
func (fakeLeaseStore) OpenReader(source.Cursor, catalog.Candidate, source.ClientPolicy) (*source.LeaseReader, error) {
	return nil, errors.New("unused")
}
func (fakeLeaseStore) EvictAll() {}

func fixedClock() func() time.Time { return func() time.Time { return time.Unix(0, 0).UTC() } }

func validDeps() (mainDeps, *runnerStub, *successfulRecovery) {
	rec, r := &successfulRecovery{}, &runnerStub{}
	deps := mainDeps{
		Profile: "test-profile", Credentials: fakeCredentialStore{}, Authorizer: fakeAuthorizer{}, Auditor: audit.NewRecorder(),
		Resolver: fakeResolver{}, Acquirer: fakeAcquirer{}, Recovery: rec, Leases: fakeLeaseStore{},
		LoadProfile: func(string) (profile.Profile, error) { return serveEligibleProfile(), nil },
		CheckEligibility: func(profile.Profile, profile.EligibilityBinding, bool, time.Time) profile.EligibilityRejection {
			return profile.EligibilityApproved
		},
		KeyringAvailable: func() bool { return true },
		OpenAudit: func(context.Context, profile.Profile) (audit.Auditor, io.Closer, error) {
			return audit.NewRecorder(), &closeStub{}, nil
		},
		OpenOwnership: func(context.Context, profile.Profile) (ownershipState, error) {
			return ownershipState{Ledger: &recoveryLedgerStub{}, Recovery: rec, Closer: &closeStub{}}, nil
		},
		ServerFactory: func(*service) (runner, error) { return r, nil },
		Now:           fixedClock(),
	}
	return deps, r, rec
}

func TestProductionCatalogResolverLoadsFreshV3KeyringProfileAndCredential(t *testing.T) {
	previousLoad, previousCredentials, previousOpen := loadCatalogProfile, newCatalogCredentialStore, openCatalogSession
	t.Cleanup(func() {
		loadCatalogProfile, newCatalogCredentialStore, openCatalogSession = previousLoad, previousCredentials, previousOpen
	})
	loaded := serveEligibleProfile()
	loaded.Name, loaded.Username = "approved", "NEXUSUSR"
	credentials := &catalogCredentialStub{secret: []byte("secret")}
	session := &catalogSessionStub{}
	loadCalls := 0
	loadCatalogProfile = func(ctx context.Context, name string) (profile.Profile, error) {
		loadCalls++
		if err := ctx.Err(); err != nil || name != "approved" {
			return profile.Profile{}, errors.New("unexpected profile load")
		}
		return loaded, nil
	}
	newCatalogCredentialStore = func() credential.CredentialStore { return credentials }
	openCatalogSession = func(_ context.Context, got profile.Profile, secret []byte) (catalogados.AuthenticatedSession, error) {
		if got != loaded || string(secret) != "secret" {
			return nil, errors.New("unexpected catalog startup")
		}
		return session, nil
	}
	search, err := catalog.NewSearch("PISA061", "")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := newProductionCatalogResolver("approved").Resolve(context.Background(), search)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || loadCalls != 1 || credentials.name != "approved" || credentials.gets != 1 || session.username != "NEXUSUSR" || string(session.password) != "secret" || session.closed != 1 {
		t.Fatalf("rows=%d loads=%d credential=%q/%d session=%q closed=%d", len(rows), loadCalls, credentials.name, credentials.gets, session.username, session.closed)
	}
	if len(session.requests) != 3 || session.requests[0].Type != mapepire.OperationPrepareSQLExecute || session.requests[0].Rows != catalog.MaxCandidates+1 || session.requests[1].Type != mapepire.OperationSQLClose || session.requests[2].Type != mapepire.OperationExit {
		t.Fatalf("requests=%#v", session.requests)
	}
}

func TestRunWithDepsComposition(t *testing.T) {
	cancelledCtx := func() context.Context {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}
	tests := []struct {
		name    string
		mutate  func(*mainDeps)
		ctxFn   func() context.Context
		wantRec int
		wantRun int
		wanterr bool
	}{
		{"recovery runs before run", nil, func() context.Context { return context.Background() }, 1, 1, false},
		{"recovery failure stops the run", func(d *mainDeps) {
			d.OpenOwnership = func(context.Context, profile.Profile) (ownershipState, error) {
				return ownershipState{Ledger: &recoveryLedgerStub{}, Recovery: &failingRecovery{err: errors.New("simulated recovery failure")}, Closer: &closeStub{}}, nil
			}
		}, func() context.Context { return context.Background() }, 0, 0, true},
		{"pre-cancelled context aborts before recovery", nil, cancelledCtx, 0, 0, true},
		{"whitespace profile is rejected", func(d *mainDeps) { d.Profile = "   \t  " }, func() context.Context { return context.Background() }, 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps, r, rec := validDeps()
			if tt.mutate != nil {
				tt.mutate(&deps)
			}
			err := runWithDeps(tt.ctxFn(), deps)
			if tt.wanterr && err == nil {
				t.Fatalf("runWithDeps error = nil, want non-nil")
			}
			if !tt.wanterr && err != nil {
				t.Fatalf("runWithDeps error = %v, want nil", err)
			}
			if rec.calls != tt.wantRec {
				t.Fatalf("Recovery calls = %d, want %d", rec.calls, tt.wantRec)
			}
			if r.runCalls != tt.wantRun {
				t.Fatalf("Run calls = %d, want %d", r.runCalls, tt.wantRun)
			}
		})
	}
}

func TestRunCommandCLIParsing(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want error
	}{
		{"help returns flag.ErrHelp", []string{"help"}, flag.ErrHelp},
		{"-h returns flag.ErrHelp", []string{"-h"}, flag.ErrHelp},
		{"empty list is rejected", []string{}, nil},
		{"unknown subcommand is rejected", []string{"unknown"}, nil},
		{"root flag is rejected", []string{"--bogus"}, nil},
		{"serve without profile is rejected", []string{"serve"}, nil},
		{"help serve returns flag.ErrHelp", []string{"help", "serve"}, flag.ErrHelp},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runCommand(tt.args, io.Discard)
			if tt.want == nil {
				if err == nil {
					t.Fatalf("runCommand(%v) error = nil, want non-nil", tt.args)
				}
				return
			}
			if err != tt.want {
				t.Fatalf("runCommand(%v) error = %v, want %v", tt.args, err, tt.want)
			}
		})
	}
}

func TestRunCommandRootUsageListsImplementedSubcommands(t *testing.T) {
	out := &strings.Builder{}
	if err := runCommand(nil, out); err == nil {
		t.Fatal("runCommand(nil) error = nil, want explicit-subcommand error")
	}
	if got, want := out.String(), "usage: nexus <subcommand> [flags]\nsubcommands: serve, configure, version, help\n"; got != want {
		t.Fatalf("root usage = %q, want %q", got, want)
	}
}

func TestPrintVersionIsDeterministic(t *testing.T) {
	oldVersion, oldRevision := releaseVersion, vcsRevision
	defer func() { releaseVersion, vcsRevision = oldVersion, oldRevision }()
	releaseVersion, vcsRevision = "v1.0.0", "abc123"
	out := &strings.Builder{}
	if err := printVersion([]string{"--json"}, out); err != nil {
		t.Fatalf("printVersion() error = %v", err)
	}
	if got, want := out.String(), "{\"version\":\"v1.0.0\",\"revision\":\"abc123\"}\n"; got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
}

var forbiddenMainSubstrings = []string{"path", "command", "shell", "exec", "sql", "ssh", "dial", "connect", "remote", "clientinfo", "parent", "argv"}

func TestMainPackageHasNoRemotePathOrShellSurface(t *testing.T) {
	checks := []struct {
		typ   reflect.Type
		names []string
	}{
		{reflect.TypeOf(mainDeps{}), []string{"Profile", "Credentials", "Authorizer", "Auditor", "Resolver", "Acquirer", "Recovery", "Leases", "Admission", "ServerFactory", "Now"}},
		{reflect.TypeOf(service{}), []string{"app", "server", "profile"}},
	}
	for _, c := range checks {
		for _, name := range c.names {
			lower := strings.ToLower(name)
			for _, forbidden := range forbiddenMainSubstrings {
				if strings.Contains(lower, forbidden) {
					t.Fatalf("%s has forbidden field %q (matched %q)", c.typ.String(), name, forbidden)
				}
			}
		}
	}
}

func TestRunCommandServeHelpTextContract(t *testing.T) {
	out := &strings.Builder{}
	if err := runCommand([]string{"help", "serve"}, out); err != flag.ErrHelp {
		t.Fatalf("runCommand(help serve) error = %v, want flag.ErrHelp", err)
	}
	lower := strings.ToLower(out.String())
	for _, want := range []string{"resolve_catalog_candidates", "read_selected_source"} {
		if !strings.Contains(lower, want) {
			t.Fatalf("help text missing required tool %q: %s", want, out.String())
		}
	}
	for _, forbidden := range []string{"ssh", "shell", "exec", "sql", "delete", "remove", "tmp path"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("help text mentions forbidden capability %q: %s", forbidden, out.String())
		}
	}
}

func TestRegisterServeFlagsExposesProfileFlag(t *testing.T) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	registerServeFlags(fs)
	if fs.Lookup("profile") == nil {
		t.Fatal("serve flag set is missing the required -profile flag")
	}
}

func TestRunWithDepsRejectsAdmissionBeforeRecoveryOrServer(t *testing.T) {
	for _, name := range []string{"missing", "legacy", "prompt", "stale", "mismatched", "keyring", "ownership", "audit"} {
		t.Run(name, func(t *testing.T) {
			deps, server, recovery := validDeps()
			admissionCalls := 0
			factoryCalls := 0
			deps.Admission = func(context.Context) error {
				admissionCalls++
				return errors.New("rejected")
			}
			deps.ServerFactory = func(*service) (runner, error) {
				factoryCalls++
				return server, nil
			}
			if err := runWithDeps(context.Background(), deps); err == nil {
				t.Fatal("runWithDeps error = nil, want admission rejection")
			}
			if admissionCalls != 1 || recovery.calls != 0 || factoryCalls != 0 || server.runCalls != 0 {
				t.Fatalf("admission/recovery/factory/run calls = %d/%d/%d/%d, want 1/0/0/0", admissionCalls, recovery.calls, factoryCalls, server.runCalls)
			}
		})
	}
}

func TestRunWithDepsAdmitsOnlyTheCurrentIndependentEligibilityBinding(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	current := serveEligibleProfile()
	binding, err := profile.DeriveEligibilityBinding(current)
	if err != nil {
		t.Fatalf("DeriveEligibilityBinding() error = %v", err)
	}

	configRoot := t.TempDir()
	store := profile.EligibilityStore{
		Root:          filepath.Join(configRoot, "BAC Nexus", "profiles"),
		UserConfigDir: func() (string, error) { return configRoot, nil },
	}
	approved, err := profile.NewEligibility(current, now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("NewEligibility() error = %v", err)
	}
	if err := store.Save(approved); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	tests := []struct {
		name         string
		prepare      func(*profile.Eligibility)
		malformed    bool
		wantRecovery int
	}{
		{name: "valid binding reaches the next injected boundary", wantRecovery: 1},
		{name: "missing eligibility", prepare: func(*profile.Eligibility) { _ = store.Revoke(current.Name) }},
		{name: "malformed eligibility", malformed: true},
		{name: "expired eligibility", prepare: func(e *profile.Eligibility) { e.ExpiresAt = now }},
		{name: "stored target cannot self-compare against a changed current profile", prepare: func(e *profile.Eligibility) { e.TargetDigest = "sha256:" + strings.Repeat("a", 64) }},
		{name: "policy mismatch", prepare: func(e *profile.Eligibility) { e.PolicyID = "other-policy" }},
		{name: "pin mismatch", prepare: func(e *profile.Eligibility) { e.PinDigest = "sha256:" + strings.Repeat("b", 64) }},
		{name: "credential mismatch", prepare: func(e *profile.Eligibility) { e.CredentialRef = "keyring:sha256:" + strings.Repeat("c", 64) }},
		{name: "artifact mismatch", prepare: func(e *profile.Eligibility) { e.ArtifactRef = "sha256:" + strings.Repeat("d", 64) }},
		{name: "proof mismatch", prepare: func(e *profile.Eligibility) { e.ProofDigest = "sha256:" + strings.Repeat("e", 64) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := store.Save(approved); err != nil {
				t.Fatalf("reset Save() error = %v", err)
			}
			if tt.prepare != nil {
				candidate := approved
				tt.prepare(&candidate)
				if tt.name != "missing eligibility" {
					if err := store.Save(candidate); err != nil {
						t.Fatalf("mutated Save() error = %v", err)
					}
				}
			}
			if tt.malformed {
				if err := os.WriteFile(filepath.Join(store.Root, current.Name+".eligibility.json"), []byte("not-json"), 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v", err)
				}
			}

			deps, server, _ := validDeps()
			deps.Now = func() time.Time { return now }
			deps.LoadProfile = func(string) (profile.Profile, error) { return current, nil }
			deps.CheckEligibility = func(p profile.Profile, expected profile.EligibilityBinding, keyringAvailable bool, checkedAt time.Time) profile.EligibilityRejection {
				if p != current || expected != binding || !keyringAvailable || checkedAt != now {
					return profile.EligibilityMismatch
				}
				return store.Check(p, expected, keyringAvailable, checkedAt)
			}
			deps.KeyringAvailable = func() bool { return true }
			operatorRetentionCalls := 0
			deps.Admission = func(context.Context) error { operatorRetentionCalls++; return nil }
			stoppingRecovery := &failingRecovery{err: errors.New("stop after eligibility")}
			deps.OpenOwnership = func(context.Context, profile.Profile) (ownershipState, error) {
				return ownershipState{Ledger: &recoveryLedgerStub{}, Recovery: stoppingRecovery, Closer: &closeStub{}}, nil
			}
			factoryCalls := 0
			deps.ServerFactory = func(*service) (runner, error) {
				factoryCalls++
				return server, nil
			}

			err := runWithDeps(context.Background(), deps)
			if err == nil {
				t.Fatal("runWithDeps() error = nil, want admission result")
			}
			if strings.Contains(strings.ToLower(err.Error()), "secret") {
				t.Fatalf("serve admission error leaks secret material: %q", err)
			}
			if operatorRetentionCalls != 1 {
				t.Fatalf("operator retention calls = %d, want 1", operatorRetentionCalls)
			}
			if stoppingRecovery.calls != tt.wantRecovery || factoryCalls != 0 || server.runCalls != 0 {
				t.Fatalf("recovery/factory/server run calls = %d/%d/%d, want %d/0/0", stoppingRecovery.calls, factoryCalls, server.runCalls, tt.wantRecovery)
			}
		})
	}
}

func TestRunWithDepsRetainsOperatorAdmissionBeforeEligibilityAndFactory(t *testing.T) {
	deps, server, recovery := validDeps()
	events := make([]string, 0, 2)
	deps.Admission = func(context.Context) error {
		events = append(events, "operator-retention")
		return errors.New("operator retention unavailable")
	}
	deps.CheckEligibility = func(profile.Profile, profile.EligibilityBinding, bool, time.Time) profile.EligibilityRejection {
		events = append(events, "eligibility")
		return profile.EligibilityApproved
	}
	factoryCalls := 0
	deps.ServerFactory = func(*service) (runner, error) {
		factoryCalls++
		return server, nil
	}

	if err := runWithDeps(context.Background(), deps); err == nil {
		t.Fatal("runWithDeps() error = nil, want operator-retention rejection")
	}
	if got, want := strings.Join(events, ","), "operator-retention"; got != want {
		t.Fatalf("admission order = %q, want %q", got, want)
	}
	if recovery.calls != 0 || factoryCalls != 0 || server.runCalls != 0 {
		t.Fatalf("recovery/factory/server run calls = %d/%d/%d, want 0/0/0", recovery.calls, factoryCalls, server.runCalls)
	}
}

func TestRunWithDepsDurableLocalStateAndRecoveryBeforeServer(t *testing.T) {
	tests := []struct {
		name          string
		configure     func(*mainDeps, *runnerStub, *successfulRecovery, *[]string, *closeStub, *closeStub)
		wantEvents    string
		wantOwnership int
		wantRecovery  int
		wantFactory   int
		wantRun       int
		wantErr       bool
	}{
		{
			name: "audit open failure blocks ownership recovery and server",
			configure: func(deps *mainDeps, _ *runnerStub, _ *successfulRecovery, events *[]string, _, _ *closeStub) {
				deps.OpenAudit = func(context.Context, profile.Profile) (audit.Auditor, io.Closer, error) {
					*events = append(*events, "audit-open")
					return nil, nil, errors.New("secret audit path")
				}
			},
			wantEvents: "audit-open", wantOwnership: 0, wantRecovery: 0, wantFactory: 0, wantRun: 0, wantErr: true,
		},
		{
			name: "ownership open failure closes audit",
			configure: func(deps *mainDeps, _ *runnerStub, _ *successfulRecovery, events *[]string, _, _ *closeStub) {
				deps.OpenOwnership = func(context.Context, profile.Profile) (ownershipState, error) {
					*events = append(*events, "ownership-open")
					return ownershipState{}, errors.New("secret ownership path")
				}
			},
			wantEvents: "audit-open,ownership-open,audit-close", wantOwnership: 0, wantRecovery: 0, wantFactory: 0, wantRun: 0, wantErr: true,
		},
		{
			name: "recovery failure closes ownership then audit before server",
			configure: func(deps *mainDeps, _ *runnerStub, _ *successfulRecovery, events *[]string, _, ownershipClose *closeStub) {
				deps.OpenOwnership = func(context.Context, profile.Profile) (ownershipState, error) {
					*events = append(*events, "ownership-open")
					return ownershipState{Ledger: &recoveryLedgerStub{}, Recovery: &failingRecovery{err: errors.New("secret remote cleanup")}, Closer: ownershipClose}, nil
				}
			},
			wantEvents: "audit-open,ownership-open,ownership-close,audit-close", wantOwnership: 0, wantRecovery: 0, wantFactory: 0, wantRun: 0, wantErr: true,
		},
		{
			name: "valid path recovers before factory and closes in reverse order",
			configure: func(deps *mainDeps, serverRunner *runnerStub, recovery *successfulRecovery, events *[]string, _, ownershipClose *closeStub) {
				serverRunner.events = events
				recovery.events = events
				deps.ServerFactory = func(*service) (runner, error) {
					*events = append(*events, "factory")
					return serverRunner, nil
				}
				deps.OpenOwnership = func(context.Context, profile.Profile) (ownershipState, error) {
					*events = append(*events, "ownership-open")
					return ownershipState{Ledger: &recoveryLedgerStub{}, Recovery: recovery, Closer: ownershipClose}, nil
				}
			},
			wantEvents: "audit-open,ownership-open,recovery,factory,run,ownership-close,audit-close", wantOwnership: 0, wantRecovery: 1, wantFactory: 0, wantRun: 1, wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps, serverRunner, recovery := validDeps()
			events := make([]string, 0, 7)
			auditClose := &closeStub{name: "audit-close", events: &events}
			ownershipClose := &closeStub{name: "ownership-close", events: &events}
			ownershipCalls, factoryCalls := 0, 0
			deps.OpenAudit = func(context.Context, profile.Profile) (audit.Auditor, io.Closer, error) {
				events = append(events, "audit-open")
				return audit.NewRecorder(), auditClose, nil
			}
			deps.OpenOwnership = func(context.Context, profile.Profile) (ownershipState, error) {
				ownershipCalls++
				events = append(events, "ownership-open")
				return ownershipState{Ledger: &recoveryLedgerStub{}, Recovery: recovery, Closer: ownershipClose}, nil
			}
			deps.ServerFactory = func(*service) (runner, error) {
				factoryCalls++
				return serverRunner, nil
			}
			tt.configure(&deps, serverRunner, recovery, &events, auditClose, ownershipClose)

			err := runWithDeps(context.Background(), deps)
			if tt.wantErr && err == nil {
				t.Fatal("runWithDeps() error = nil, want failure")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("runWithDeps() error = %v, want nil", err)
			}
			if err != nil && strings.Contains(strings.ToLower(err.Error()), "secret") {
				t.Fatalf("runWithDeps() leaked sensitive failure detail: %q", err)
			}
			if got := strings.Join(events, ","); got != tt.wantEvents {
				t.Fatalf("event order = %q, want %q", got, tt.wantEvents)
			}
			if ownershipCalls != tt.wantOwnership || recovery.calls != tt.wantRecovery || factoryCalls != tt.wantFactory || serverRunner.runCalls != tt.wantRun {
				t.Fatalf("ownership/recovery/factory/run = %d/%d/%d/%d, want %d/%d/%d/%d", ownershipCalls, recovery.calls, factoryCalls, serverRunner.runCalls, tt.wantOwnership, tt.wantRecovery, tt.wantFactory, tt.wantRun)
			}
		})
	}
}

func TestRunWithDepsCancellationAfterAuditOpenClosesWithoutRemoteWork(t *testing.T) {
	deps, server, recovery := validDeps()
	events := make([]string, 0, 2)
	auditClose := &closeStub{name: "audit-close", events: &events}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deps.OpenAudit = func(context.Context, profile.Profile) (audit.Auditor, io.Closer, error) {
		events = append(events, "audit-open")
		cancel()
		return audit.NewRecorder(), auditClose, nil
	}
	factoryCalls := 0
	deps.ServerFactory = func(*service) (runner, error) {
		factoryCalls++
		return server, nil
	}

	err := runWithDeps(ctx, deps)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("runWithDeps() error = %v, want context cancellation", err)
	}
	if got, want := strings.Join(events, ","), "audit-open,audit-close"; got != want {
		t.Fatalf("event order = %q, want %q", got, want)
	}
	if recovery.calls != 0 || factoryCalls != 0 || server.runCalls != 0 {
		t.Fatalf("recovery/factory/run = %d/%d/%d, want 0/0/0", recovery.calls, factoryCalls, server.runCalls)
	}
}

func TestRunWithDepsRejectsStoredBindingSelfComparison(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	approvedProfile := serveEligibleProfile()
	stored, err := profile.NewEligibility(approvedProfile, now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("NewEligibility() error = %v", err)
	}
	configRoot := t.TempDir()
	store := profile.EligibilityStore{
		Root:          filepath.Join(configRoot, "BAC Nexus", "profiles"),
		UserConfigDir: func() (string, error) { return configRoot, nil },
	}
	if err := store.Save(stored); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	current := approvedProfile
	current.Host = "127.0.0.2"
	deps, server, recovery := validDeps()
	deps.Now = func() time.Time { return now }
	deps.LoadProfile = func(string) (profile.Profile, error) { return current, nil }
	deps.CheckEligibility = store.Check
	deps.KeyringAvailable = func() bool { return true }
	operatorRetentionCalls := 0
	deps.Admission = func(context.Context) error { operatorRetentionCalls++; return nil }
	factoryCalls := 0
	deps.ServerFactory = func(*service) (runner, error) {
		factoryCalls++
		return server, nil
	}

	if err := runWithDeps(context.Background(), deps); err == nil {
		t.Fatal("runWithDeps() error = nil, want current-profile eligibility rejection")
	}
	if operatorRetentionCalls != 1 || recovery.calls != 0 || factoryCalls != 0 || server.runCalls != 0 {
		t.Fatalf("operator/recovery/factory/server calls = %d/%d/%d/%d, want 1/0/0/0", operatorRetentionCalls, recovery.calls, factoryCalls, server.runCalls)
	}
}

func serveEligibleProfile() profile.Profile {
	return profile.Profile{SchemaVersion: profile.SchemaVersionV3, Name: "production", Host: "127.0.0.1", Port: 22, Username: "operator", HostKeyFingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", HostKeyTrust: profile.HostKeyTrustTOFU, CredentialMode: profile.CredentialModeKeyring}
}

type recoveryLedgerStub struct {
	records     []source.OwnershipRecord
	listCalls   int
	deleted     []source.OwnershipRecord
	closeCalled int
}

func (*recoveryLedgerStub) Admit(context.Context, source.OwnershipRecord) error { return nil }

func (s *recoveryLedgerStub) ListRecovery(context.Context) ([]source.OwnershipRecord, error) {
	s.listCalls++
	return s.records, nil
}

func (s *recoveryLedgerStub) Delete(_ context.Context, record source.OwnershipRecord) error {
	s.deleted = append(s.deleted, record)
	return nil
}

func (s *recoveryLedgerStub) Close() error {
	s.closeCalled++
	return nil
}

type recoveryRemoteStub struct {
	removed string
	closed  int
}

func (s *recoveryRemoteStub) Remove(_ context.Context, path string) error {
	s.removed = path
	return nil
}

func (s *recoveryRemoteStub) Stat(context.Context, string) (os.FileInfo, error) {
	return nil, source.ErrRemoteNotFound
}

func (s *recoveryRemoteStub) Close() error {
	s.closed++
	return nil
}

func TestOpenDurableOwnershipBuildsBoundedExactRecoveryCoordinator(t *testing.T) {
	loaded := serveEligibleProfile()
	record := source.OwnershipRecord{
		Token:        []byte("0123456789abcdef"),
		RemotePath:   "/home/operator/.bac-nexus/tmp/0123456789abcdef.utf8",
		Profile:      loaded.Name,
		TargetDigest: recoveryDigestForTest(loaded),
		CreatedAt:    time.Unix(1_000, 0).UTC(),
	}
	ledger := &recoveryLedgerStub{records: []source.OwnershipRecord{record}}
	remote := &recoveryRemoteStub{}
	credentialStore := &recoveryCredentialStub{secret: []byte("keyring-secret")}

	oldOpenLedger := openDurableOwnershipLedger
	oldLoadProfile := loadRecoveryProfile
	oldCredentials := newRecoveryCredentialStore
	oldOpenCleanup := openRecoveryCleanup
	defer func() {
		openDurableOwnershipLedger = oldOpenLedger
		loadRecoveryProfile = oldLoadProfile
		newRecoveryCredentialStore = oldCredentials
		openRecoveryCleanup = oldOpenCleanup
	}()

	openDurableOwnershipLedger = func(string) (ownershipOpenResult, error) {
		return ownershipOpenResult{Ledger: ledger, RecoveryLedger: ledger, Closer: ledger}, nil
	}
	events := make([]string, 0, 3)
	loadRecoveryProfile = func(ctx context.Context, name string) (profile.Profile, error) {
		if err := ctx.Err(); err != nil {
			return profile.Profile{}, err
		}
		events = append(events, "profile:"+name)
		return loaded, nil
	}
	newRecoveryCredentialStore = func() credential.CredentialStore { return credentialStore }
	openRecoveryCleanup = func(ctx context.Context, got profile.Profile, secret []byte) (source.RecoveryRemote, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if got != loaded || string(secret) != "keyring-secret" {
			t.Fatal("cleanup opener received an unexpected profile or credential")
		}
		events = append(events, "cleanup:"+got.Name)
		return remote, nil
	}

	opened, err := openDurableOwnership(context.Background(), loaded)
	if err != nil || opened.Ledger != ledger || opened.Recovery == nil || opened.Closer == nil {
		t.Fatalf("openDurableOwnership() = (%#v, %v), want opened ledger, coordinator, and closer", opened, err)
	}
	if err := opened.Recovery.Recover(context.Background()); err != nil {
		t.Fatalf("Recover() error = %v", err)
	}
	if got, want := strings.Join(events, ","), "profile:production,cleanup:production"; got != want {
		t.Fatalf("recovery events = %q, want %q", got, want)
	}
	if got, want := credentialStore.requests, []string{"production"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("keyring profile requests = %v, want %v", got, want)
	}
	if remote.removed != record.RemotePath || remote.closed != 1 || len(ledger.deleted) != 1 || !reflect.DeepEqual(ledger.deleted[0], record) {
		t.Fatalf("exact cleanup = removed %q, closed %d, deleted %#v; want exact record cleanup", remote.removed, remote.closed, ledger.deleted)
	}
	if err := opened.Closer.Close(); err != nil || ledger.closeCalled != 1 {
		t.Fatalf("ownership closer = %v with %d calls, want one successful close", err, ledger.closeCalled)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := opened.Recovery.Recover(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Recover() error = %v, want context.Canceled", err)
	}
	if ledger.listCalls != 1 {
		t.Fatalf("ListRecovery calls = %d, want no additional call after cancellation", ledger.listCalls)
	}
}

type recoveryCredentialStub struct {
	secret   []byte
	requests []string
}

func (s *recoveryCredentialStub) Get(name string) ([]byte, error) {
	s.requests = append(s.requests, name)
	return append([]byte(nil), s.secret...), nil
}

func (*recoveryCredentialStub) Set(string, []byte) error { return nil }
func (*recoveryCredentialStub) Delete(string) error      { return nil }

func recoveryDigestForTest(p profile.Profile) []byte {
	// Keep the test record's binding aligned with RecoveryCoordinator's exact
	// profile/target validation without exposing that internal implementation.
	hash := sha256.New()
	_, _ = hash.Write([]byte("BAC Nexus/recovery-target-binding/v1\x00"))
	writeField := func(value string) {
		length := make([]byte, 4)
		binary.BigEndian.PutUint32(length, uint32(len(value)))
		_, _ = hash.Write(length)
		_, _ = hash.Write([]byte(value))
	}
	writeField(p.Host)
	var port [2]byte
	binary.BigEndian.PutUint16(port[:], uint16(p.Port))
	_, _ = hash.Write(port[:])
	writeField(p.Username)
	writeField(p.HostKeyFingerprint)
	writeField(string(p.HostKeyTrust))
	return hash.Sum(nil)
}

var _ = credential.ErrCredentialsUnavailable

func TestRunWithDepsBuildsCompleteProductionGraphAfterRecovery(t *testing.T) {
	deps, server, _ := validDeps()
	loaded := serveEligibleProfile()
	events := make([]string, 0, 9)
	ledger := &recoveryLedgerStub{}
	recovery := &successfulRecovery{events: &events}
	auditClose := &closeStub{name: "audit-close", events: &events}
	ownershipClose := &closeStub{name: "ownership-close", events: &events}
	deps.LoadProfile = func(string) (profile.Profile, error) { return loaded, nil }
	deps.Resolver = nil
	deps.Acquirer = nil
	deps.Leases = nil
	deps.OpenAudit = func(_ context.Context, got profile.Profile) (audit.Auditor, io.Closer, error) {
		if got != loaded {
			t.Fatal("audit did not receive the admitted profile")
		}
		events = append(events, "audit-open")
		return audit.NewRecorder(), auditClose, nil
	}
	deps.OpenOwnership = func(_ context.Context, got profile.Profile) (ownershipState, error) {
		if got != loaded {
			t.Fatal("ownership did not receive the admitted profile")
		}
		events = append(events, "ownership-open")
		return ownershipState{Ledger: ledger, Recovery: recovery, Closer: ownershipClose}, nil
	}
	deps.BuildResolver = func(got profile.Profile) app.CatalogResolver {
		if got != loaded {
			t.Fatal("resolver did not receive the admitted profile")
		}
		events = append(events, "resolver")
		return fakeResolver{}
	}
	deps.BuildAcquirer = func(got profile.Profile, gotLedger source.OwnershipLedger, gotRecovery app.RecoveryCoordinator) app.SnapshotAcquirer {
		if got != loaded || gotLedger != ledger || gotRecovery != recovery {
			t.Fatal("acquirer did not receive the opened ownership state")
		}
		events = append(events, "acquirer")
		return fakeAcquirer{}
	}
	deps.NewLeases = func() app.LeaseStore {
		events = append(events, "leases")
		return fakeLeaseStore{}
	}
	deps.ServerFactory = func(s *service) (runner, error) {
		if s.app == nil {
			t.Fatal("server did not receive the composed service")
		}
		events = append(events, "server")
		server.events = &events
		return server, nil
	}

	if err := runWithDeps(context.Background(), deps); err != nil {
		t.Fatalf("runWithDeps() error = %v", err)
	}
	if got, want := strings.Join(events, ","), "audit-open,ownership-open,recovery,resolver,acquirer,leases,server,run,ownership-close,audit-close"; got != want {
		t.Fatalf("composition order = %q, want %q", got, want)
	}
}

func TestRunWithDepsShutdownEvictsLeasesThenClosesOwnershipAuditsAndClosesAudit(t *testing.T) {
	deps, server, recovery := validDeps()
	events := make([]string, 0, 8)
	auditor := &lifecycleAuditor{events: &events}
	deps.OpenAudit = func(context.Context, profile.Profile) (audit.Auditor, io.Closer, error) {
		events = append(events, "audit-open")
		return auditor, &closeStub{name: "audit-close", events: &events}, nil
	}
	deps.OpenOwnership = func(context.Context, profile.Profile) (ownershipState, error) {
		events = append(events, "ownership-open")
		return ownershipState{Ledger: &recoveryLedgerStub{}, Recovery: recovery, Closer: &closeStub{name: "ownership-close", events: &events}}, nil
	}
	deps.Leases = lifecycleLeaseStore{events: &events}
	deps.ServerFactory = func(*service) (runner, error) {
		server.events = &events
		return server, nil
	}

	if err := runWithDeps(context.Background(), deps); err != nil {
		t.Fatalf("runWithDeps() error = %v", err)
	}
	if got, want := strings.Join(events, ","), "audit-open,ownership-open,run,leases-evict,ownership-close,audit-lifecycle,audit-close"; got != want {
		t.Fatalf("shutdown order = %q, want %q", got, want)
	}
	if auditor.lifecycleEvents != 1 {
		t.Fatalf("lifecycle audit events = %d, want 1", auditor.lifecycleEvents)
	}
}

func TestRunWithDepsSanitizesRunnerLifecycleErrors(t *testing.T) {
	deps, _, _ := validDeps()
	deps.ServerFactory = func(*service) (runner, error) {
		return &runnerStub{runErr: errors.New("peer payload secret=/srv/nexus")}, nil
	}

	err := runWithDeps(context.Background(), deps)
	if !errors.Is(err, errServeMCPUnavailable) {
		t.Fatalf("runWithDeps() error = %v, want %v", err, errServeMCPUnavailable)
	}
	if strings.Contains(err.Error(), "secret=") || strings.Contains(err.Error(), "/srv/nexus") {
		t.Fatalf("runWithDeps() leaked runner detail: %q", err)
	}
}

func TestRunWithDepsPreservesContextLifecycleErrors(t *testing.T) {
	deps, _, _ := validDeps()
	deps.ServerFactory = func(*service) (runner, error) {
		return &runnerStub{runErr: context.DeadlineExceeded}, nil
	}

	if err := runWithDeps(context.Background(), deps); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("runWithDeps() error = %v, want context deadline", err)
	}
}

func TestNexusStdioSubprocessProducesOnlyJSONRPCOnStdout(t *testing.T) {
	if os.Getenv("NEXUS_STDIO_HELPER") == "1" {
		helperServer, err := internalmcp.New(internalmcp.Config{
			Info: internalmcp.Info{Name: "nexus-stdio-helper", Version: "test"}, Service: stdioService{},
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "helper server unavailable")
			os.Exit(2)
		}
		_ = helperServer.Run(context.Background())
		os.Exit(0)
	}

	command := exec.Command(os.Args[0], "-test.run=^TestNexusStdioSubprocessProducesOnlyJSONRPCOnStdout$")
	command.Env = append(os.Environ(), "NEXUS_STDIO_HELPER=1")
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(stdin, "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2025-06-18\",\"capabilities\":{},\"clientInfo\":{\"name\":\"test\",\"version\":\"test\"}}}\n")
	time.Sleep(50 * time.Millisecond)
	_ = stdin.Close()
	waited := make(chan error, 1)
	go func() { waited <- command.Wait() }()
	select {
	case err := <-waited:
		if err != nil {
			t.Fatalf("stdio child error = %v, stderr=%q", err, stderr.String())
		}
	case <-time.After(3 * time.Second):
		_ = command.Process.Kill()
		<-waited
		t.Fatal("stdio child timed out")
	}
	if command.ProcessState == nil || !command.ProcessState.Exited() {
		t.Fatal("stdio helper child was not invoked and reaped")
	}
	if strings.Contains(stderr.String(), "{") {
		t.Fatalf("stderr contains protocol JSON: %q", stderr.String())
	}
	scanner := bufio.NewScanner(bytes.NewReader(stdout.Bytes()))
	frames := 0
	for scanner.Scan() {
		var frame struct {
			JSONRPC string `json:"jsonrpc"`
			ID      any    `json:"id"`
			Method  string `json:"method"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &frame); err != nil || frame.JSONRPC != "2.0" || (frame.ID == nil && frame.Method == "") {
			t.Fatalf("stdout frame is not MCP JSON-RPC: %q (%v)", scanner.Text(), err)
		}
		frames++
	}
	if err := scanner.Err(); err != nil || frames == 0 {
		t.Fatalf("stdout frames=%d scan error=%v", frames, err)
	}
}

type stdioService struct{}

func (stdioService) ResolveCatalog(context.Context, catalog.Search, security.Selector) ([]catalog.Candidate, error) {
	return nil, errors.New("unused")
}
func (stdioService) ReadSelectedSource(context.Context, catalog.Candidate, string, source.Range) (source.Page, error) {
	return source.Page{}, errors.New("unused")
}

type lifecycleLeaseStore struct{ events *[]string }

func (s lifecycleLeaseStore) Acquire(*source.Snapshot, catalog.Candidate, source.ClientPolicy) (source.Cursor, error) {
	return "", errors.New("unused")
}
func (s lifecycleLeaseStore) Lookup(source.Cursor) (catalog.Candidate, error) {
	return catalog.Candidate{}, errors.New("unused")
}
func (s lifecycleLeaseStore) OpenReader(source.Cursor, catalog.Candidate, source.ClientPolicy) (*source.LeaseReader, error) {
	return nil, errors.New("unused")
}
func (s lifecycleLeaseStore) EvictAll() { *s.events = append(*s.events, "leases-evict") }

type lifecycleAuditor struct {
	events          *[]string
	lifecycleEvents int
}

func (a *lifecycleAuditor) Record(_ context.Context, event audit.Event) error {
	if event.Capability == audit.CapabilityLifecycleCompletion {
		a.lifecycleEvents++
		*a.events = append(*a.events, "audit-lifecycle")
	}
	return audit.ValidateEvent(event)
}

func TestNewProductionSourceAcquirerDefersFreshRemoteSetupUntilRequest(t *testing.T) {
	loaded := serveEligibleProfile()
	ledger := &recoveryLedgerStub{}
	recovery := &successfulRecovery{}
	acquirer := newProductionSourceAcquirer(loaded, ledger, recovery)
	if acquirer.Open == nil || acquirer.Recover == nil || acquirer.Ownership != ledger || acquirer.Profile != loaded.Name || !reflect.DeepEqual(acquirer.TargetDigest, recoveryDigestForTest(loaded)) {
		t.Fatalf("production source acquirer = %#v, want owned, recoverable request-scoped source acquisition", acquirer)
	}
}

func TestNewProductionSourceAcquirerUsesFreshProfileAndKeyringForEachRequest(t *testing.T) {
	previousLoad, previousCredentials, previousOpen := loadSourceProfile, newSourceCredentialStore, openSourceAcquisition
	t.Cleanup(func() {
		loadSourceProfile, newSourceCredentialStore, openSourceAcquisition = previousLoad, previousCredentials, previousOpen
	})
	loaded := serveEligibleProfile()
	credentials := &catalogCredentialStub{secret: []byte("source-secret")}
	loadCalls, openCalls := 0, 0
	loadSourceProfile = func(ctx context.Context, name string) (profile.Profile, error) {
		loadCalls++
		if err := ctx.Err(); err != nil || name != loaded.Name {
			return profile.Profile{}, errors.New("unexpected source profile request")
		}
		return loaded, nil
	}
	newSourceCredentialStore = func() credential.CredentialStore { return credentials }
	openSourceAcquisition = func(_ context.Context, got profile.Profile, secret []byte) (source.AcquisitionRemote, io.Closer, error) {
		openCalls++
		if got != loaded || string(secret) != "source-secret" {
			t.Fatal("source opener did not receive the fresh admitted profile and keyring credential")
		}
		return nil, nil, errors.New("fake source opener stops before remote contact")
	}

	acquirer := newProductionSourceAcquirer(loaded, &recoveryLedgerStub{}, &successfulRecovery{})
	for range 2 {
		if _, _, err := acquirer.Open(context.Background()); err == nil {
			t.Fatal("source opener error = nil, want fake factory rejection")
		}
	}
	if loadCalls != 2 || credentials.gets != 2 || credentials.name != loaded.Name || openCalls != 2 {
		t.Fatalf("fresh source setup loads/keyring/name/open = %d/%d/%q/%d, want 2/2/%q/2", loadCalls, credentials.gets, credentials.name, openCalls, loaded.Name)
	}
}

func TestRunWithDepsRejectionAndCancellationDoNotBuildProductionFactories(t *testing.T) {
	for _, tc := range []struct {
		name      string
		cancelled bool
		reject    bool
		wantClose string
	}{
		{name: "admission rejection", reject: true},
		{name: "cancellation after ownership", cancelled: true, wantClose: "audit-open,ownership-open,ownership-close,audit-close"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps, _, _ := validDeps()
			events := make([]string, 0, 3)
			builds := 0
			deps.BuildResolver = func(profile.Profile) app.CatalogResolver { builds++; return fakeResolver{} }
			deps.BuildAcquirer = func(profile.Profile, source.OwnershipLedger, app.RecoveryCoordinator) app.SnapshotAcquirer {
				builds++
				return fakeAcquirer{}
			}
			deps.NewLeases = func() app.LeaseStore { builds++; return fakeLeaseStore{} }
			if tc.reject {
				deps.Admission = func(context.Context) error { return errors.New("rejected") }
			}
			if tc.cancelled {
				ctx, cancel := context.WithCancel(context.Background())
				deps.OpenAudit = func(context.Context, profile.Profile) (audit.Auditor, io.Closer, error) {
					events = append(events, "audit-open")
					return audit.NewRecorder(), &closeStub{name: "audit-close", events: &events}, nil
				}
				deps.OpenOwnership = func(context.Context, profile.Profile) (ownershipState, error) {
					events = append(events, "ownership-open")
					cancel()
					return ownershipState{Ledger: &recoveryLedgerStub{}, Recovery: &successfulRecovery{}, Closer: &closeStub{name: "ownership-close", events: &events}}, nil
				}
				if err := runWithDeps(ctx, deps); !errors.Is(err, context.Canceled) {
					t.Fatalf("runWithDeps() error = %v, want context cancellation", err)
				}
			} else if err := runWithDeps(context.Background(), deps); err == nil {
				t.Fatal("runWithDeps() error = nil, want rejection")
			}
			if builds != 0 {
				t.Fatalf("production factory calls = %d, want 0", builds)
			}
			if tc.wantClose != "" && strings.Join(events, ",") != tc.wantClose {
				t.Fatalf("close order = %q, want %q", strings.Join(events, ","), tc.wantClose)
			}
		})
	}
}
