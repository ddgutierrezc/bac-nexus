package configuration

import (
	"context"
	"errors"
	"testing"
	"time"

	"bac-nexus/internal/connectors/ibmi/mapepirestdio"
	"bac-nexus/internal/profile"
)

type runtimeClientFake struct{ closes int }

func (f *runtimeClientFake) Close() error                         { f.closes++; return nil }
func (*runtimeClientFake) RemoteFiles() mapepirestdio.RemoteFiles { return nil }

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
