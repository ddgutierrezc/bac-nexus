package configuration

import (
	"context"
	"errors"
	"testing"
	"time"

	"bac-nexus/internal/connectors/ibmi/mapepirestdio"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
)

type runtimeClientFake struct {
	closes   int
	closeErr error
}

func (f *runtimeClientFake) Close() error                         { f.closes++; return f.closeErr }
func (*runtimeClientFake) RemoteFiles() mapepirestdio.RemoteFiles { return nil }
func (*runtimeClientFake) FixedMapepireProof(context.Context, mapepirestdio.LaunchPolicy, string, []byte) (remote.FixedProofMetadata, error) {
	return remote.FixedProofMetadata{}, nil
}

func admittedSSH() Step8Result {
	return Step8Result{RequestID: "req-6b", Decision: DecisionSSHEligible, Class: ResultProofSuccess}
}

func TestSSHRuntimeFactoryRejectsUnsafeArtifactBeforeDial(t *testing.T) {
	for _, name := range []string{"unpinned", "corrupt", "partial", "changed", "latest", "unverified"} {
		t.Run(name, func(t *testing.T) {
			dials := 0
			factory := SSHRuntimeFactory{
				VerifyArtifact: func(string) error { return errors.New(name) },
				Dial: func(context.Context, profile.Profile, []byte) (SSHRuntimeClient, error) {
					dials++
					return &runtimeClientFake{}, nil
				},
			}
			_, result := factory.Open(context.Background(), admittedSSH(), savedStep8Profile(t), []byte("opaque"))
			if result.Decision != DecisionTerminal || result.Class != ResultArtifactFailure {
				t.Fatalf("result=%+v", result)
			}
			if dials != 0 {
				t.Fatalf("unsafe artifact dialed remote %d times", dials)
			}
		})
	}
}

func TestSSHRuntimeFactoryClosesClientAfterJavaOrUploadFailure(t *testing.T) {
	tests := []struct {
		name   string
		java   error
		upload error
		want   ResultClass
	}{
		{name: "java", java: errors.New("not ready"), want: ResultJavaFailure},
		{name: "upload", upload: errors.New("bounded upload failed"), want: ResultUploadFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &runtimeClientFake{}
			factory := SSHRuntimeFactory{
				VerifyArtifact: func(string) error { return nil },
				Dial: func(ctx context.Context, _ profile.Profile, _ []byte) (SSHRuntimeClient, error) {
					if _, ok := ctx.Deadline(); !ok {
						t.Fatal("dial context is unbounded")
					}
					return client, nil
				},
				JavaReady: func(context.Context, profile.Profile) error { return tt.java },
				Upload: func(ctx context.Context, _ mapepirestdio.RemoteFiles, _ string) (string, error) {
					if _, ok := ctx.Deadline(); !ok {
						t.Fatal("upload is unbounded")
					}
					return "", tt.upload
				},
			}
			_, result := factory.Open(context.Background(), admittedSSH(), savedStep8Profile(t), []byte("opaque"))
			if result.Decision != DecisionTerminal || result.Class != tt.want || !result.Cleanup {
				t.Fatalf("result=%+v", result)
			}
			if client.closes != 1 {
				t.Fatalf("client closes=%d want 1", client.closes)
			}
		})
	}
}

func TestSSHRuntimeFactoryMapsDialTimeoutWithoutRuntime(t *testing.T) {
	factory := SSHRuntimeFactory{
		VerifyArtifact: func(string) error { return nil },
		Dial: func(context.Context, profile.Profile, []byte) (SSHRuntimeClient, error) {
			return nil, context.DeadlineExceeded
		},
	}
	_, result := factory.Open(context.Background(), admittedSSH(), savedStep8Profile(t), []byte("opaque"))
	if result.Decision != DecisionTerminal || result.Class != ResultOperationTimeout || result.Cleanup {
		t.Fatalf("result=%+v", result)
	}
}

func TestSSHRuntimeFactoryDefaultHasBoundedOperationTimeout(t *testing.T) {
	if SSHRuntimeOperationTimeout <= 0 || SSHRuntimeOperationTimeout > time.Minute {
		t.Fatalf("operation timeout=%s", SSHRuntimeOperationTimeout)
	}
}

func TestSSHRuntimeFactoryKeepsPrimaryFailureAndAssignsUniqueTraceIDs(t *testing.T) {
	clients := []*runtimeClientFake{{}, {}, {}, {}}
	index := 0
	factory := SSHRuntimeFactory{
		VerifyArtifact: func(string) error { return nil },
		Dial: func(context.Context, profile.Profile, []byte) (SSHRuntimeClient, error) {
			client := clients[index]
			index++
			return client, nil
		},
		JavaReady: func(context.Context, profile.Profile) error { return errors.New("java") },
		Upload: func(context.Context, mapepirestdio.RemoteFiles, string) (string, error) {
			return "/tmp/pinned.jar", nil
		},
	}
	for i := range clients[:2] {
		clients[i].closeErr = errors.New("cleanup")
		_, result := factory.Open(context.Background(), admittedSSH(), savedStep8Profile(t), []byte("opaque"))
		if result.Class != ResultJavaFailure || result.Cleanup {
			t.Fatalf("result=%+v", result)
		}
		if clients[i].closes != 1 {
			t.Fatalf("client %d closes=%d", i, clients[i].closes)
		}
	}

	factory.JavaReady = func(context.Context, profile.Profile) error { return nil }
	runtimeA, resultA := factory.Open(context.Background(), admittedSSH(), savedStep8Profile(t), []byte("opaque"))
	runtimeB, resultB := factory.Open(context.Background(), admittedSSH(), savedStep8Profile(t), []byte("opaque"))
	if resultA.Class != ResultProofSuccess || resultB.Class != ResultProofSuccess || runtimeA.traceID == 0 || runtimeA.traceID == runtimeB.traceID {
		t.Fatalf("runtime traces=%+v,%+v results=%+v,%+v", runtimeA, runtimeB, resultA, resultB)
	}
}
