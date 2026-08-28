package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"bac-nexus/internal/connectors/ibmi/mapepirestdio"
	"bac-nexus/internal/mapepire"
	"bac-nexus/internal/remote"
)

type proofRuntimeClientFake struct {
	transport mapepire.MessageTransport
	launchErr error
	policy    mapepirestdio.LaunchPolicy
	closes    int
	closeErr  error
}

func (f *proofRuntimeClientFake) Close() error {
	f.closes++
	return f.closeErr
}
func (*proofRuntimeClientFake) RemoteFiles() mapepirestdio.RemoteFiles { return nil }
func (f *proofRuntimeClientFake) FixedMapepireProof(_ context.Context, policy mapepirestdio.LaunchPolicy, username string, secret []byte) (remote.FixedProofMetadata, error) {
	f.policy = policy
	if f.launchErr != nil {
		return remote.FixedProofMetadata{}, &remote.FixedProofError{Stage: remote.FixedProofLaunchStage, Err: f.launchErr}
	}
	if f.transport == nil {
		return remote.FixedProofMetadata{}, remote.ErrFixedProofSession
	}
	proof, err := mapepire.NewMessageSession(f.transport).FixedProof(context.Background(), username, secret)
	if err != nil {
		return remote.FixedProofMetadata{}, &remote.FixedProofError{Stage: remote.FixedProofRunStage, Err: err}
	}
	return remote.FixedProofMetadata{Rows: proof.Rows, Revision: proof.Revision}, nil
}

type proofTransportFake struct {
	mu       sync.Mutex
	requests []mapepire.Request
	response chan []byte
	closed   bool
}

func newProofTransportFake() *proofTransportFake {
	return &proofTransportFake{response: make(chan []byte, 4)}
}

func (f *proofTransportFake) Send(_ context.Context, frame []byte) error {
	var request mapepire.Request
	if err := json.Unmarshal(frame, &request); err != nil {
		return err
	}
	f.mu.Lock()
	f.requests = append(f.requests, request)
	f.mu.Unlock()
	response := mapepire.Response{ID: request.ID, Success: true}
	switch request.Type {
	case mapepire.OperationConnect:
		response.Job = "QPADEV0001"
	case mapepire.OperationPrepareSQLExecute:
		response.HasResults = true
		response.IsDone = true
		response.Data = []map[string]json.RawMessage{{"1": json.RawMessage("1")}}
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return err
	}
	f.response <- encoded
	return nil
}

func (f *proofTransportFake) Receive(context.Context) ([]byte, error) { return <-f.response, nil }
func (f *proofTransportFake) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *proofTransportFake) snapshot() []mapepire.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]mapepire.Request(nil), f.requests...)
}

func TestSSHRuntimeProveUsesOnlyFixedSingleSessionAndMetadata(t *testing.T) {
	transport := newProofTransportFake()
	client := &proofRuntimeClientFake{transport: transport}
	runtime := &SSHRuntime{client: client, remoteJAR: "/tmp/pinned.jar"}
	metadata, result := runtime.Prove(context.Background(), savedStep8Profile(t), []byte("opaque"))
	if result.Decision != DecisionSSHEligible || result.Class != ResultProofSuccess || result.ProofRevision != ProofRevision {
		t.Fatalf("result=%+v", result)
	}
	if metadata != (ProofMetadata{Rows: 1, ProofRevision: ProofRevision}) {
		t.Fatalf("metadata=%+v", metadata)
	}
	if client.policy.RemoteJAR != runtime.remoteJAR || !client.policy.Consented {
		t.Fatalf("launch policy=%+v", client.policy)
	}
	requests := transport.snapshot()
	if len(requests) != 4 {
		t.Fatalf("requests=%d want 4", len(requests))
	}
	if requests[0].Type != mapepire.OperationConnect || requests[0].Username == "" || requests[0].Password == "" {
		t.Fatalf("connect=%+v", requests[0])
	}
	if requests[1].Type != mapepire.OperationPrepareSQLExecute || requests[1].SQL != mapepire.FixedProofSQL || requests[1].Rows != 1 || requests[1].Username != "" || requests[1].Password != "" {
		t.Fatalf("proof=%+v", requests[1])
	}
	for _, request := range requests[2:] {
		if request.Username != "" || request.Password != "" {
			t.Fatalf("credentials leaked to %s: %+v", request.Type, request)
		}
	}
}

func TestSSHRuntimeProveMapsLaunchSessionAndProofFailures(t *testing.T) {
	tests := []struct {
		name      string
		runtime   *SSHRuntime
		wantClass ResultClass
	}{
		{name: "launch", runtime: &SSHRuntime{client: &proofRuntimeClientFake{launchErr: errors.New("launch")}, remoteJAR: "/tmp/pinned.jar"}, wantClass: ResultLaunchFailure},
		{name: "session", runtime: &SSHRuntime{client: &proofRuntimeClientFake{}, remoteJAR: "/tmp/pinned.jar"}, wantClass: ResultSessionFailure},
		{name: "proof", runtime: &SSHRuntime{client: &proofRuntimeClientFake{transport: failingProofTransport{}}, remoteJAR: "/tmp/pinned.jar"}, wantClass: ResultProofFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, result := tt.runtime.Prove(context.Background(), savedStep8Profile(t), []byte("opaque"))
			if result.Decision != DecisionTerminal || result.Class != tt.wantClass {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}

func TestProofErrorClassMapsTypedTerminalFailures(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name string
		ctx  context.Context
		err  error
		want ResultClass
	}{
		{name: "cancelled", ctx: cancelled, err: context.Canceled, want: ResultCancelled},
		{name: "limit", ctx: context.Background(), err: &remote.FixedProofError{Stage: remote.FixedProofRunStage, Err: mapepire.ErrLimitExceeded}, want: ResultLimitExceeded},
		{name: "protocol", ctx: context.Background(), err: &remote.FixedProofError{Stage: remote.FixedProofRunStage, Err: mapepire.ErrProtocolViolation}, want: ResultProtocolFailure},
		{name: "unknown", ctx: context.Background(), err: errors.New("opaque"), want: ResultProofFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := proofErrorClass(tt.ctx, tt.err); got != tt.want {
				t.Fatalf("class=%q want %q", got, tt.want)
			}
		})
	}
}

func TestSSHRuntimeProveSettlesClientBeforeZeroingCredentials(t *testing.T) {
	tests := []struct {
		name      string
		client    *proofRuntimeClientFake
		ctx       context.Context
		wantClass ResultClass
		cleanup   bool
	}{
		{name: "success", client: &proofRuntimeClientFake{transport: newProofTransportFake()}, ctx: context.Background(), wantClass: ResultProofSuccess, cleanup: true},
		{name: "proof failure", client: &proofRuntimeClientFake{transport: failingProofTransport{}}, ctx: context.Background(), wantClass: ResultProofFailure, cleanup: true},
		{name: "cancelled", client: &proofRuntimeClientFake{transport: failingProofTransport{}}, ctx: cancelledContext(), wantClass: ResultCancelled, cleanup: true},
		{name: "cleanup failure preserves primary result", client: &proofRuntimeClientFake{transport: failingProofTransport{}, closeErr: errors.New("close")}, ctx: context.Background(), wantClass: ResultProofFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := []byte("opaque")
			runtime := &SSHRuntime{client: tt.client, remoteJAR: "/tmp/pinned.jar", requestID: "req-6d"}
			_, result := runtime.Prove(tt.ctx, savedStep8Profile(t), secret)
			if result.Class != tt.wantClass || result.Cleanup != tt.cleanup {
				t.Fatalf("result=%+v", result)
			}
			if tt.client.closes != 1 {
				t.Fatalf("client closes=%d want 1", tt.client.closes)
			}
			if err := runtime.Close(); err != nil && tt.cleanup {
				t.Fatalf("second close: %v", err)
			}
			if tt.client.closes != 1 {
				t.Fatalf("client settled more than once: %d", tt.client.closes)
			}
			for _, b := range secret {
				if b != 0 {
					t.Fatalf("credential was not zeroed: %q", secret)
				}
			}
		})
	}
}

func cancelledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

type failingProofTransport struct{}

func (failingProofTransport) Send(context.Context, []byte) error { return errors.New("proof failed") }
func (failingProofTransport) Receive(context.Context) ([]byte, error) {
	return nil, errors.New("proof failed")
}
func (failingProofTransport) Close() error { return nil }
