package main

import (
	"context"
	"io"
	"testing"

	"bac-nexus/internal/audit"
	"bac-nexus/internal/configuration"
	"bac-nexus/internal/credential"
	"bac-nexus/internal/hostidentity"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
	"bac-nexus/internal/security"
	"bac-nexus/internal/tui"
)

func TestRunCommandConfigureIsSeparateFromServe(t *testing.T) {
	old := runConfigureTUI
	defer func() { runConfigureTUI = old }()
	calls := 0
	oldVersion, oldRevision := releaseVersion, vcsRevision
	defer func() { releaseVersion, vcsRevision = oldVersion, oldRevision }()
	releaseVersion, vcsRevision = "v9.8.7", "abc123"
	runConfigureTUI = func(_ context.Context, _ configuration.ProfilesStore, build tui.BuildInfo, inspector hostidentity.Inspector, runner configuration.Step8Runner) error {
		calls++
		if _, ok := inspector.(remote.HostIdentityInspector); !ok {
			t.Fatalf("configure inspector = %T, want remote.HostIdentityInspector", inspector)
		}
		if build != (tui.BuildInfo{Version: "v9.8.7", Revision: "abc123"}) {
			t.Fatalf("configure build identity = %#v", build)
		}
		service, ok := runner.(configuration.Step8Service)
		if !ok {
			t.Fatalf("configure runner = %T, want configuration.Step8Service", runner)
		}
		if _, ok := service.Observe.(configuration.ManagedStep8PreAuth); !ok {
			t.Fatalf("observe adapter = %T, want configuration.ManagedStep8PreAuth", service.Observe)
		}
		if _, ok := service.Credentials.(credential.Step8Provider); !ok {
			t.Fatalf("credential adapter = %T, want credential.Step8Provider", service.Credentials)
		}
		if _, ok := service.WSS.(configuration.ManagedStep8WSS); !ok {
			t.Fatalf("WSS adapter = %T, want configuration.ManagedStep8WSS", service.WSS)
		}
		if _, ok := service.Gate.Policy.(security.Step8SSHPolicy); !ok {
			t.Fatalf("SSH policy adapter = %T, want security.Step8SSHPolicy", service.Gate.Policy)
		}
		if _, ok := service.Gate.Trust.(security.Step8SSHTrustAdapter); !ok {
			t.Fatalf("SSH trust adapter = %T, want security.Step8SSHTrustAdapter", service.Gate.Trust)
		}
		if _, ok := service.SSH.(configuration.SSHRuntimeFactory); !ok {
			t.Fatalf("SSH runtime adapter = %T, want configuration.SSHRuntimeFactory", service.SSH)
		}
		if _, ok := service.Markers.(step8MarkerAdapter); !ok {
			t.Fatalf("marker store = %T, want step8MarkerAdapter", service.Markers)
		}
		if _, ok := service.Audit.(audit.Step8ConfigurationAdapter); !ok {
			t.Fatalf("audit adapter = %T, want audit.Step8ConfigurationAdapter", service.Audit)
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

func TestStep8ProductionRunnerWSSSuccessDoesNotInvokeSSHRuntime(t *testing.T) {
	service, ok := newStep8ProductionRunner(profile.Store{Root: t.TempDir()}).(configuration.Step8Service)
	if !ok {
		t.Fatal("production runner is not a Step8Service")
	}
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
