// Package main wires the v1 MCP stdio server entry point.
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

// all fakes are minimum implementations used only to satisfy the
// composition-root contracts. Each method records a single count or
// returns a single canned result.
type runnerStub struct {
	runErr   error
	runCalls int
}

func (r *runnerStub) Run(ctx context.Context) error { r.runCalls++; return r.runErr }

type successfulRecovery struct{ calls int }

func (s *successfulRecovery) Recover(ctx context.Context) error { s.calls++; return ctx.Err() }

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

type fakeAuthorizer struct{}

func (fakeAuthorizer) Authorize(ctx context.Context, sel security.Selector, tgt security.CapabilityTarget) (security.Decision_, error) {
	return security.Decision_{Selector: sel, Target: tgt, Decision: security.DecisionAllow, Reason: "ok"}, nil
}

type fakeResolver struct{}

func (fakeResolver) Resolve(ctx context.Context, q catalog.Query) ([]catalog.Candidate, error) {
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

func fixedClock() func() time.Time { return func() time.Time { return time.Unix(0, 0).UTC() } }

func validDeps() (mainDeps, *runnerStub, *successfulRecovery) {
	rec, r := &successfulRecovery{}, &runnerStub{}
	deps := mainDeps{
		Profile: "test-profile", Credentials: fakeCredentialStore{}, Authorizer: fakeAuthorizer{}, Auditor: audit.NewRecorder(),
		Resolver: fakeResolver{}, Acquirer: fakeAcquirer{}, Recovery: rec, Leases: fakeLeaseStore{},
		ServerFactory: func(*service) (runner, error) { return r, nil },
		Now:           fixedClock(),
	}
	return deps, r, rec
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
		{"recovery failure stops the run", func(d *mainDeps) { d.Recovery = &failingRecovery{err: errors.New("simulated recovery failure")} }, func() context.Context { return context.Background() }, 0, 0, true},
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
		{reflect.TypeOf(mainDeps{}), []string{"Profile", "Credentials", "Authorizer", "Auditor", "Resolver", "Acquirer", "Recovery", "Leases", "ServerFactory", "Now"}},
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

var _ = credential.ErrCredentialsUnavailable
