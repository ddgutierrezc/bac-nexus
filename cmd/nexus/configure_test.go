package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"path/filepath"
	"runtime"
	"testing"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
	"bac-nexus/internal/tui"
)

func TestRunCommandConfigureIsSeparateFromServe(t *testing.T) {
	old := runConfigureTUI
	defer func() { runConfigureTUI = old }()
	calls := 0
	oldVersion, oldRevision := releaseVersion, vcsRevision
	defer func() { releaseVersion, vcsRevision = oldVersion, oldRevision }()
	releaseVersion, vcsRevision = "v9.8.7", "abc123"
	runConfigureTUI = func(_ context.Context, _ configuration.ProfilesStore, build tui.BuildInfo, operations tui.OnboardingOperations, prompt remote.SecretPrompt) error {
		calls++
		if build != (tui.BuildInfo{Version: "v9.8.7", Revision: "abc123"}) {
			t.Fatalf("configure build identity = %#v", build)
		}
		if _, ok := operations.(*configuration.OnboardingService); !ok {
			t.Fatalf("configure operations = %T, want onboarding service", operations)
		}
		if prompt.Input == nil || prompt.Output == nil || prompt.IsTerminal == nil || prompt.Read == nil {
			t.Fatal("configure prompt is not fully injected")
		}
		return nil
	}
	if err := runCommand([]string{"configure"}, io.Discard); err != nil {
		t.Fatalf("configure returned error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("configure TUI calls = %d, want 1", calls)
	}
}

func TestDirectOnboardingServiceBindsEligibilityToFixedStep8Proof(t *testing.T) {
	service := newDirectOnboardingService(profile.Store{Root: t.TempDir()})
	if service == nil {
		t.Fatal("newDirectOnboardingService returned nil")
	}

	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test source location is unavailable")
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(filepath.Dir(testFile), "main.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse production constructor: %v", err)
	}
	constructor := findFunction(parsed, "newDirectOnboardingService")
	if constructor == nil {
		t.Fatal("production direct onboarding constructor is unavailable")
	}
	if got := constructor.Type.Params.NumFields(); got != 1 {
		t.Fatalf("constructor parameters = %d, want only profile store", got)
	}
	if constructor.Type.Results == nil || constructor.Type.Results.NumFields() != 1 {
		t.Fatal("constructor must return one onboarding service")
	}
	if !constructorContainsFixedProofGate(constructor) {
		t.Fatal("direct onboarding eligibility is not bound to the fixed Step 8 proof gate")
	}
}

func findFunction(file *ast.File, name string) *ast.FuncDecl {
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Name.Name == name {
			return function
		}
	}
	return nil
}

func constructorContainsFixedProofGate(constructor *ast.FuncDecl) bool {
	var buildsProductionRunner, checksFixedRevision, installsCommit bool
	ast.Inspect(constructor.Body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.CallExpr:
			if identifier, ok := node.Fun.(*ast.Ident); ok && identifier.Name == "newStep8ProductionRunnerWithCredentials" {
				buildsProductionRunner = true
			}
		case *ast.SelectorExpr:
			if packageName, ok := node.X.(*ast.Ident); ok && packageName.Name == "mapepire" && node.Sel.Name == "FixedProofRevision" {
				checksFixedRevision = true
			}
		case *ast.KeyValueExpr:
			if field, ok := node.Key.(*ast.Ident); ok && field.Name == "Commit" {
				installsCommit = true
			}
		}
		return true
	})
	return buildsProductionRunner && checksFixedRevision && installsCommit
}

func TestStep8ProductionRunnerWSSSuccessDoesNotInvokeSSHRuntime(t *testing.T) {
	service := newStep8ProductionRunnerWithCredentials(profile.Store{Root: t.TempDir()}, configureCredentialsFunc(func(context.Context, string, profile.CredentialMode) ([]byte, error) { return []byte("opaque"), nil }))
	ssh := &configureSSHFactory{}
	session := &configureWSSSession{}
	service.Observe = configureObserveFunc(func(context.Context, profile.Profile) configuration.Observation {
		return configuration.Observation{Decision: configuration.DecisionWSSSelected, Reason: configuration.ReasonWSSSelected}
	})
	service.Credentials = configureCredentialsFunc(func(context.Context, string, profile.CredentialMode) ([]byte, error) {
		return []byte("opaque"), nil
	})
	service.WSS = configureWSSFunc(func(context.Context, profile.Profile) (configuration.Step8WSSSession, error) {
		return session, nil
	})
	service.SSH = ssh
	service.Markers, service.Audit = nil, nil

	result := service.Run(context.Background(), configuration.Step8Request{
		RequestID:  "wss-success",
		WSSConsent: true,
		Profile:    profile.Profile{SchemaVersion: profile.SchemaVersionV3, Name: "profile-1", Host: "host.example", Port: 22, Username: "user", CredentialMode: profile.CredentialModePrompt},
	})
	if result.Decision != configuration.DecisionWSSSelected || result.Class != configuration.ResultProofSuccess || !result.Cleanup {
		t.Fatalf("WSS result = %+v", result)
	}
	if session.proveCalls != 1 || session.closeCalls != 1 {
		t.Fatalf("WSS proof/close calls = %d/%d, want 1/1", session.proveCalls, session.closeCalls)
	}
	if ssh.calls != 0 {
		t.Fatalf("SSH runtime calls = %d, want 0", ssh.calls)
	}
}

type configureObserveFunc func(context.Context, profile.Profile) configuration.Observation

func (f configureObserveFunc) Observe(ctx context.Context, p profile.Profile) configuration.Observation {
	return f(ctx, p)
}

type configureCredentialsFunc func(context.Context, string, profile.CredentialMode) ([]byte, error)

func (f configureCredentialsFunc) Get(ctx context.Context, key string, mode profile.CredentialMode) ([]byte, error) {
	return f(ctx, key, mode)
}

type configureWSSFunc func(context.Context, profile.Profile) (configuration.Step8WSSSession, error)

func (f configureWSSFunc) Open(ctx context.Context, p profile.Profile) (configuration.Step8WSSSession, error) {
	return f(ctx, p)
}

type configureWSSSession struct{ proveCalls, closeCalls int }

func (s *configureWSSSession) Prove(context.Context, string, []byte) (configuration.ProofMetadata, error) {
	s.proveCalls++
	return configuration.ProofMetadata{Rows: 1, ProofRevision: configuration.ProofRevision}, nil
}

func (s *configureWSSSession) Close() error {
	s.closeCalls++
	return nil
}

type configureSSHFactory struct{ calls int }

func (f *configureSSHFactory) Open(context.Context, configuration.Step8Result, profile.Profile, []byte) (*configuration.SSHRuntime, configuration.Step8Result) {
	f.calls++
	return nil, configuration.Step8Result{}
}
