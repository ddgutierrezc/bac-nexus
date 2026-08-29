package configuration

import (
	"context"
	"errors"
	"testing"

	"bac-nexus/internal/connectors/ibmi/mapepirewss"
	"bac-nexus/internal/profile"
)

func TestManagedStep8WSSBindsSavedProfileAndIndependentTLSTrust(t *testing.T) {
	p := serviceSavedProfile()
	p.Host = "ibmi.example.test"
	p.TLSTrust = profile.TrustEvidence{
		Mode:       profile.TrustModePin,
		Pin:        "sha256/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Provenance: "tls-enrollment",
	}

	var gotEndpoint string
	var gotOptions mapepirewss.Options
	adapter := NewManagedStep8WSS()
	adapter.newFactory = func(endpoint string, options mapepirewss.Options) managedStep8WSSOpener {
		gotEndpoint, gotOptions = endpoint, options
		return managedStep8WSSOpenFunc(func(context.Context) (Step8WSSSession, error) {
			return &managedStep8WSSTestSession{}, nil
		})
	}

	session, err := adapter.Open(context.Background(), p)
	if err != nil || session == nil {
		t.Fatalf("Open() session=%v err=%v", session, err)
	}
	if got, want := gotEndpoint, "wss://ibmi.example.test:8076"; got != want {
		t.Fatalf("endpoint=%q, want %q", got, want)
	}
	if gotOptions.Pin != p.TLSTrust.Pin || gotOptions.TOFU {
		t.Fatalf("options=%#v, want independent TLS pin without TOFU", gotOptions)
	}
}

func TestManagedStep8WSSFailuresStayTerminalAndNeverReachSSH(t *testing.T) {
	for _, failure := range []error{
		errors.New("authentication failed"),
		errors.New("authorization denied"),
		errors.New("identity failure"),
		errors.New("trust mismatch"),
		errors.New("protocol failure"),
		errors.New("limit exceeded"),
		context.Canceled,
		context.DeadlineExceeded,
		errors.New("unknown failure"),
	} {
		t.Run(failure.Error(), func(t *testing.T) {
			sshCalls := 0
			adapter := NewManagedStep8WSS()
			adapter.newFactory = func(string, mapepirewss.Options) managedStep8WSSOpener {
				return managedStep8WSSOpenFunc(func(context.Context) (Step8WSSSession, error) {
					return managedStep8WSSTestSession{proveErr: failure}, nil
				})
			}
			service := Step8Service{
				Observe: step8ObserveFunc(func(context.Context, profile.Profile) Observation {
					return Observation{Decision: DecisionWSSSelected, Reason: ReasonWSSSelected}
				}),
				Credentials: step8CredentialsFunc(func(context.Context, string, profile.CredentialMode) ([]byte, error) {
					return []byte("opaque"), nil
				}),
				WSS: adapter,
				SSH: step8SSHRuntimeFunc(func(context.Context, Step8Result, profile.Profile, []byte) (*SSHRuntime, Step8Result) {
					sshCalls++
					return nil, Step8Result{}
				}),
			}

			result := service.Run(context.Background(), Step8Request{RequestID: "request-1", Profile: serviceSavedProfile()})
			if result.Decision != DecisionTerminal || !IsTerminalResult(result.Class) {
				t.Fatalf("result=%#v, want terminal classification", result)
			}
			if sshCalls != 0 {
				t.Fatalf("SSH calls=%d, want 0", sshCalls)
			}
		})
	}
}

type step8SSHRuntimeFunc func(context.Context, Step8Result, profile.Profile, []byte) (*SSHRuntime, Step8Result)

func (f step8SSHRuntimeFunc) Open(ctx context.Context, result Step8Result, p profile.Profile, secret []byte) (*SSHRuntime, Step8Result) {
	return f(ctx, result, p, secret)
}

type managedStep8WSSTestSession struct{ proveErr error }

func (s managedStep8WSSTestSession) Prove(context.Context, string, []byte) (ProofMetadata, error) {
	return ProofMetadata{}, s.proveErr
}

func (managedStep8WSSTestSession) Close() error { return nil }
