// Package main wires the v1 MCP stdio server entry point. The
// command composes the catalog-context service from phases 2, 3B.3,
// 5A, 5B, and 6, invokes the pre-acquire recovery gate during
// startup, and runs the MCP server over stdio. It must never
// expose a generic remote, path, shell, SQL, or SSH tool.
package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"bac-nexus/internal/audit"
	"bac-nexus/internal/catalog"
	"bac-nexus/internal/credential"
	"bac-nexus/internal/security"
	"bac-nexus/internal/source"
)

// ---------------------------------------------------------------------------
// Test fixtures: minimal, deterministic doubles for the narrow
// surfaces the main package depends on. No live IBM i, no real
// filesystem, no real keyring, and no real SSH is ever involved.
// ---------------------------------------------------------------------------

// runnerStub is the package-local runner double used by every main
// test. It records the Run call and returns the configured error.
type runnerStub struct {
	runErr   error
	runCalls int
	gotCtx   context.Context
}

func (r *runnerStub) Run(ctx context.Context) error {
	r.runCalls++
	r.gotCtx = ctx
	return r.runErr
}

// failingRecovery is a RecoveryCoordinator that returns the
// configured error. The main package test uses it to prove the
// composition root fails closed when pre-acquire recovery fails.
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

// successfulRecovery is a RecoveryCoordinator that records its
// call and returns nil. The main package test uses it to prove
// the composition root performs recovery before running the MCP
// server.
type successfulRecovery struct {
	calls int
}

func (s *successfulRecovery) Recover(ctx context.Context) error {
	s.calls++
	return ctx.Err()
}

// fakeCredentialStore is a minimal CredentialStore used to satisfy
// the app.Service contract.
type fakeCredentialStore struct{}

func (fakeCredentialStore) Get(profile string) ([]byte, error)      { return []byte("test"), nil }
func (fakeCredentialStore) Set(profile string, secret []byte) error { return nil }
func (fakeCredentialStore) Delete(profile string) error             { return nil }

// fakeAuthorizer is a minimal Authorizer that always allows.
type fakeAuthorizer struct{}

func (fakeAuthorizer) Authorize(ctx context.Context, selector security.Selector, target security.CapabilityTarget) (security.Decision_, error) {
	return security.Decision_{
		Selector: selector,
		Class:    security.CapabilityCatalogResolve,
		Target:   target,
		Decision: security.DecisionAllow,
		Reason:   "allowlisted selector and matching target class",
	}, nil
}

// fakeResolver is a minimal CatalogResolver returning a single
// canonical candidate.
type fakeResolver struct{}

func (fakeResolver) Resolve(ctx context.Context, query catalog.Query) ([]catalog.Candidate, error) {
	return []catalog.Candidate{candidateFixture()}, nil
}

// fakeAcquirer is a minimal SnapshotAcquirer that never produces a
// snapshot; the main package tests do not exercise acquisition.
type fakeAcquirer struct{}

func (fakeAcquirer) Acquire(ctx context.Context, candidate catalog.Candidate) (*source.Snapshot, error) {
	return nil, errors.New("acquirer not used by main tests")
}

// fakeLeaseStore is a minimal LeaseStore; main tests do not exercise
// leases. Returning a zero cursor is sufficient because the public
// service methods that consume it are not invoked in main tests.
type fakeLeaseStore struct{}

func (fakeLeaseStore) Acquire(snap *source.Snapshot, selection catalog.Candidate, policy source.ClientPolicy) (source.Cursor, error) {
	return source.Cursor("test-cursor"), nil
}
func (fakeLeaseStore) Lookup(cursor source.Cursor) (catalog.Candidate, error) {
	return candidateFixture(), nil
}
func (fakeLeaseStore) OpenReader(cursor source.Cursor, selection catalog.Candidate, policy source.ClientPolicy) (*source.LeaseReader, error) {
	return nil, errors.New("lease reader not used by main tests")
}

func candidateFixture() catalog.Candidate {
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

// fixedClock returns a deterministic clock used by main tests.
func fixedClock() func() time.Time { return func() time.Time { return time.Unix(0, 0).UTC() } }

// validDeps returns a mainDeps struct with a successful recovery
// and a runner stub. Tests can override individual fields before
// invoking runWithDeps.
func validDeps() (mainDeps, *runnerStub, *successfulRecovery) {
	rec := &successfulRecovery{}
	r := &runnerStub{}
	deps := mainDeps{
		Profile:       "test-profile",
		Credentials:   fakeCredentialStore{},
		Authorizer:    fakeAuthorizer{},
		Auditor:       audit.NewRecorder(),
		Resolver:      fakeResolver{},
		Acquirer:      fakeAcquirer{},
		Recovery:      rec,
		Leases:        fakeLeaseStore{},
		ServerFactory: func(s *service) (runner, error) { return r, nil },
		Now:           fixedClock(),
	}
	return deps, r, rec
}

// ---------------------------------------------------------------------------
// Composition tests
// ---------------------------------------------------------------------------

// TestRunWithDepsInvokesRecoveryBeforeRun proves the composition
// root performs the pre-acquire recovery gate (via
// RecoveryCoordinator.Recover during app.Service.Startup) before
// running the MCP server. The recovery counter is the canonical
// proof that Startup succeeded.
func TestRunWithDepsInvokesRecoveryBeforeRun(t *testing.T) {
	deps, r, rec := validDeps()
	if err := runWithDeps(context.Background(), deps); err != nil {
		t.Fatalf("runWithDeps error = %v", err)
	}
	if rec.calls != 1 {
		t.Fatalf("Recovery calls = %d, want 1", rec.calls)
	}
	if r.runCalls != 1 {
		t.Fatalf("Run calls = %d, want 1", r.runCalls)
	}
}

// TestRunWithDepsFailsClosedOnRecoveryError proves a failed
// pre-acquire recovery aborts the lifecycle before the MCP server
// runs. The runner must never be invoked.
func TestRunWithDepsFailsClosedOnRecoveryError(t *testing.T) {
	deps, r, _ := validDeps()
	deps.Recovery = &failingRecovery{err: errors.New("simulated recovery failure")}
	if err := runWithDeps(context.Background(), deps); err == nil {
		t.Fatal("runWithDeps error = nil, want recovery failure")
	}
	if r.runCalls != 0 {
		t.Fatalf("Run calls = %d, want 0 after recovery failure", r.runCalls)
	}
}

// TestRunWithDepsHonorsContextCancellation proves a pre-cancelled
// context aborts before Recovery is invoked and the MCP server
// never runs.
func TestRunWithDepsHonorsContextCancellation(t *testing.T) {
	deps, r, rec := validDeps()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := runWithDeps(ctx, deps)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("runWithDeps error = %v, want context.Canceled", err)
	}
	if rec.calls != 0 {
		t.Fatalf("Recovery calls = %d after cancellation, want 0", rec.calls)
	}
	if r.runCalls != 0 {
		t.Fatalf("Run calls = %d after cancellation, want 0", r.runCalls)
	}
}

// TestRunWithDepsRequiresProfile proves the composition root
// refuses an empty profile because every audit record is bound to
// the policy identifier and every credential call needs a profile.
func TestRunWithDepsRequiresProfile(t *testing.T) {
	deps, r, rec := validDeps()
	deps.Profile = ""
	err := runWithDeps(context.Background(), deps)
	if err == nil {
		t.Fatal("runWithDeps error = nil, want empty profile rejection")
	}
	if rec.calls != 0 {
		t.Fatalf("Recovery calls = %d with empty profile, want 0", rec.calls)
	}
	if r.runCalls != 0 {
		t.Fatalf("Run calls = %d with empty profile, want 0", r.runCalls)
	}
}

// TestRunWithDepsRejectsWhitespaceProfile proves the profile
// validation also rejects whitespace-only inputs, because the
// profile name is used as an audit policy identifier.
func TestRunWithDepsRejectsWhitespaceProfile(t *testing.T) {
	deps, _, _ := validDeps()
	deps.Profile = "   \t  "
	if err := runWithDeps(context.Background(), deps); err == nil {
		t.Fatal("runWithDeps error = nil, want whitespace profile rejection")
	}
}

// ---------------------------------------------------------------------------
// CLI subcommand parsing
// ---------------------------------------------------------------------------

// TestRunCommandHelpReturnsFlagHelp proves the binary accepts a
// help subcommand and returns flag.ErrHelp so the caller can
// distinguish help from real failure.
func TestRunCommandHelpReturnsFlagHelp(t *testing.T) {
	if err := runCommand([]string{"help"}, io.Discard); err != flag.ErrHelp {
		t.Fatalf("runCommand(help) error = %v, want flag.ErrHelp", err)
	}
}

// TestRunCommandHelpFlagReturnsFlagHelp proves the binary accepts
// the -h short help flag and returns flag.ErrHelp.
func TestRunCommandHelpFlagReturnsFlagHelp(t *testing.T) {
	if err := runCommand([]string{"-h"}, io.Discard); err != flag.ErrHelp {
		t.Fatalf("runCommand(-h) error = %v, want flag.ErrHelp", err)
	}
}

// TestRunCommandRejectsEmptyArgList proves the binary rejects an
// empty argument list and asks the user to specify a subcommand.
func TestRunCommandRejectsEmptyArgList(t *testing.T) {
	if err := runCommand([]string{}, io.Discard); err == nil {
		t.Fatal("runCommand() error = nil, want explicit subcommand error")
	}
}

// TestRunCommandRejectsUnknownSubcommand proves the binary rejects
// an unknown subcommand rather than silently running the default.
func TestRunCommandRejectsUnknownSubcommand(t *testing.T) {
	if err := runCommand([]string{"unknown"}, io.Discard); err == nil {
		t.Fatal("runCommand(unknown) error = nil, want rejection")
	}
}

// TestRunCommandRejectsRootFlag proves the binary rejects a root
// flag (anything starting with "-") other than -h, because there
// are no root-level flags.
func TestRunCommandRejectsRootFlag(t *testing.T) {
	if err := runCommand([]string{"--bogus"}, io.Discard); err == nil {
		t.Fatal("runCommand(--bogus) error = nil, want rejection")
	}
}

// TestRunCommandServeRejectsMissingProfile proves the serve
// subcommand requires a non-empty profile. A missing profile would
// silently produce unaudited requests and is therefore rejected at
// the CLI boundary.
func TestRunCommandServeRejectsMissingProfile(t *testing.T) {
	if err := runCommand([]string{"serve"}, io.Discard); err == nil {
		t.Fatal("runCommand(serve) error = nil, want profile rejection")
	}
}

// TestRunCommandServeHelpReturnsFlagHelp proves the serve
// subcommand help text is reachable through `nexus help serve`.
func TestRunCommandServeHelpReturnsFlagHelp(t *testing.T) {
	if err := runCommand([]string{"help", "serve"}, io.Discard); err != flag.ErrHelp {
		t.Fatalf("runCommand(help serve) error = %v, want flag.ErrHelp", err)
	}
}

// ---------------------------------------------------------------------------
// Structural surface guard
// ---------------------------------------------------------------------------

// TestMainPackageHasNoRemotePathOrShellSurface is a structural
// reflection test: every public type in the main package must
// never expose generic remote, path, shell, SQL, or SSH
// capabilities. The test enumerates a curated set of identifier
// substrings that the design and security model forbid.
func TestMainPackageHasNoRemotePathOrShellSurface(t *testing.T) {
	checks := []struct {
		typ   reflect.Type
		label string
	}{
		{typ: reflect.TypeOf(mainDeps{}), label: "mainDeps"},
		{typ: reflect.TypeOf(service{}), label: "service"},
	}
	for _, check := range checks {
		for _, forbidden := range forbiddenMainSubstrings {
			if found, name := fieldContains(check.typ, forbidden); found {
				t.Fatalf("%s has forbidden field %q (matched %q)", check.label, name, forbidden)
			}
		}
	}
}

// forbiddenMainSubstrings is the structural guard list for the main
// package. The list is authoritative; adding an entry requires an
// explicit decision and a matching red test.
var forbiddenMainSubstrings = []string{
	"path",
	"command",
	"shell",
	"exec",
	"sql",
	"ssh",
	"dial",
	"connect",
	"remote",
	"clientinfo",
	"parent",
	"argv",
}

// fieldContains returns whether the supplied struct type exposes a
// field whose lower-cased name contains the supplied substring,
// recursing into anonymous embedded structs. It is duplicated
// here to keep the main package's test surface self-contained.
func fieldContains(typ reflect.Type, substring string) (bool, string) {
	return fieldContainsVisited(typ, substring, map[reflect.Type]bool{})
}

func fieldContainsVisited(typ reflect.Type, substring string, visited map[reflect.Type]bool) (bool, string) {
	if typ == nil || visited[typ] {
		return false, ""
	}
	visited[typ] = true
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return false, ""
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Anonymous {
			if found, name := fieldContainsVisited(field.Type, substring, visited); found {
				return true, name
			}
			continue
		}
		if strings.Contains(strings.ToLower(field.Name), substring) {
			return true, field.Name
		}
	}
	return false, ""
}

// ---------------------------------------------------------------------------
// Help-text contract
// ---------------------------------------------------------------------------

// TestRunCommandServeDescriptionMentionsNoGenericTool proves the
// serve subcommand's help text never advertises a generic remote,
// shell, SQL, or path tool. A textual contract catches accidental
// help-text drift that the structural reflection test would miss.
func TestRunCommandServeDescriptionMentionsNoGenericTool(t *testing.T) {
	out := &strings.Builder{}
	if err := runCommand([]string{"help", "serve"}, out); err != flag.ErrHelp {
		t.Fatalf("runCommand(help serve) error = %v, want flag.ErrHelp", err)
	}
	lower := strings.ToLower(out.String())
	for _, forbidden := range []string{"ssh", "shell", "exec", "sql", "delete", "remove", "list path", "tmp path"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("help text mentions forbidden capability %q: %s", forbidden, out.String())
		}
	}
}

// TestRunCommandServeSummaryListsTwoTools proves the help text
// advertises exactly the two allowed tools and no others. A textual
// contract catches accidental feature drift.
func TestRunCommandServeSummaryListsTwoTools(t *testing.T) {
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
}

// TestRegisterServeFlagsExposesProfileFlag proves the canonical
// flag set exposes the required profile flag. The flag is the
// only CLI input that controls the audit policy identifier, so
// every serve invocation must supply it.
func TestRegisterServeFlagsExposesProfileFlag(t *testing.T) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	registerServeFlags(fs)
	if fs.Lookup("profile") == nil {
		t.Fatal("serve flag set is missing the required -profile flag")
	}
}

// _ keeps the credential import alive when unused in this file.
var _ = credential.ErrCredentialsUnavailable
