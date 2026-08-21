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

type runnerStub struct{ runErr error; runCalls int }

func (r *runnerStub) Run(ctx context.Context) error { r.runCalls++; return r.runErr }

type successfulRecovery struct{ calls int }

func (s *successfulRecovery) Recover(ctx context.Context) error { s.calls++; return ctx.Err() }

type failingRecovery struct{ err error; calls int }

func (f *failingRecovery) Recover(ctx context.Context) error {
	f.calls++
	if err := ctx.Err(); err != nil {
		return err
	}
	return f.err
}

type fakeCredentialStore struct{}

func (fakeCredentialStore) Get(profile string) ([]byte, error) { return []byte("test"), nil }
func (fakeCredentialStore) Set(profile string, secret []byte) error { return nil }
func (fakeCredentialStore) Delete(profile string) error { return nil }

type fakeAuthorizer struct{}

func (fakeAuthorizer) Authorize(ctx context.Context, selector security.Selector, target security.CapabilityTarget) (security.Decision_, error) {
	return security.Decision_{Selector: selector, Target: target, Decision: security.DecisionAllow, Reason: "ok"}, nil
}

type fakeResolver struct{}

func (fakeResolver) Resolve(ctx context.Context, query catalog.Query) ([]catalog.Candidate, error) {
	return []catalog.Candidate{candidateFixture()}, nil
}

type fakeAcquirer struct{}

func (fakeAcquirer) Acquire(ctx context.Context, candidate catalog.Candidate) (*source.Snapshot, error) {
	return nil, errors.New("unused")
}

type fakeLeaseStore struct{}

func (fakeLeaseStore) Acquire(snap *source.Snapshot, selection catalog.Candidate, policy source.ClientPolicy) (source.Cursor, error) {
	return source.Cursor("test-cursor"), nil
}
func (fakeLeaseStore) Lookup(cursor source.Cursor) (catalog.Candidate, error) { return candidateFixture(), nil }
func (fakeLeaseStore) OpenReader(cursor source.Cursor, selection catalog.Candidate, policy source.ClientPolicy) (*source.LeaseReader, error) {
	return nil, errors.New("unused")
}

func candidateFixture() catalog.Candidate {
	return catalog.Candidate{Item: "PISA061", SourceLibrary: "QRPGLESRC", SourceFileBase: "QRPGLESRC", ObjectType: "RPGLE", SourceType: "RPG", Application: "APP", Version: "V1", ProductionLibrary: "PRODLIB", Description: "test"}
}

func fixedClock() func() time.Time { return func() time.Time { return time.Unix(0, 0).UTC() } }

func validDeps() (mainDeps, *runnerStub, *successfulRecovery) {
	rec := &successfulRecovery{}
	r := &runnerStub{}
	deps := mainDeps{
		Profile: "test-profile", Credentials: fakeCredentialStore{}, Authorizer: fakeAuthorizer{}, Auditor: audit.NewRecorder(),
		Resolver: fakeResolver{}, Acquirer: fakeAcquirer{}, Recovery: rec, Leases: fakeLeaseStore{},
		ServerFactory: func(s *service) (runner, error) { return r, nil },
		Now:           fixedClock(),
	}
	return deps, r, rec
}

func TestRunWithDepsComposition(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(d *mainDeps)
		ctxFn     func() context.Context
		wantRec   int
		wantRun   int
		wantErrIs error
	}{
		{
			name:    "recovery runs before run",
			ctxFn:   func() context.Context { return context.Background() },
			wantRec: 1, wantRun: 1,
		},
		{
			name:      "recovery failure stops the run",
			mutate:    func(d *mainDeps) { d.Recovery = &failingRecovery{err: errors.New("simulated recovery failure")} },
			ctxFn:     func() context.Context { return context.Background() },
			wantRec:   0, wantRun: 0,
			wantErrIs: errors.New(""),
		},
		{
			name: "pre-cancelled context aborts before recovery",
			ctxFn: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			wantRec: 0, wantRun: 0, wantErrIs: context.Canceled,
		},
		{
			name:    "empty profile is rejected",
			mutate:  func(d *mainDeps) { d.Profile = "" },
			ctxFn:   func() context.Context { return context.Background() },
			wantRec: 0, wantRun: 0, wantErrIs: errors.New(""),
		},
		{
			name:    "whitespace profile is rejected",
			mutate:  func(d *mainDeps) { d.Profile = "   \t  " },
			ctxFn:   func() context.Context { return context.Background() },
			wantRec: 0, wantRun: 0, wantErrIs: errors.New(""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps, r, rec := validDeps()
			if tt.mutate != nil {
				tt.mutate(&deps)
			}
			err := runWithDeps(tt.ctxFn(), deps)
			if tt.wantErrIs != nil {
				if err == nil {
					t.Fatalf("runWithDeps error = nil, want non-nil")
				}
				if tt.wantErrIs == context.Canceled && !errors.Is(err, context.Canceled) {
					t.Fatalf("runWithDeps error = %v, want context.Canceled", err)
				}
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

var forbiddenMainSubstrings = []string{"path", "command", "shell", "exec", "sql", "ssh", "dial", "connect", "remote", "clientinfo", "parent", "argv"}

func TestMainPackageHasNoRemotePathOrShellSurface(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeOf(mainDeps{}), reflect.TypeOf(service{})} {
		for _, forbidden := range forbiddenMainSubstrings {
			if found, name := fieldContains(typ, forbidden); found {
				t.Fatalf("%s has forbidden field %q (matched %q)", typ.String(), name, forbidden)
			}
		}
	}
}

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
