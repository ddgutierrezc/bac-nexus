package configuration

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"bac-nexus/internal/connectors/ibmi/mapepirestdio"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
)

type runtimeClientFake struct {
	closes    int
	closeErr  error
	ensureErr error
	artifact  string
}

func (f *runtimeClientFake) Close() error { f.closes++; return f.closeErr }
func (f *runtimeClientFake) EnsureMapepireServerJAR(_ context.Context, artifact string) (mapepirestdio.VerifiedMapepireArtifactReceipt, error) {
	f.artifact = artifact
	return mapepirestdio.VerifiedMapepireArtifactReceipt{}, f.ensureErr
}
func (*runtimeClientFake) FixedMapepireProof(context.Context, mapepirestdio.VerifiedMapepireArtifactReceipt, string, []byte) (remote.FixedProofMetadata, error) {
	return remote.FixedProofMetadata{}, nil
}

func admittedSSH() Step8Result {
	return Step8Result{RequestID: "req-6b", Decision: DecisionSSHEligible, Class: ResultProofSuccess}
}

func testArtifactResolver() func() (string, error) {
	return func() (string, error) { return "/bundle/mapepire-server.jar", nil }
}

func TestSSHRuntimeFactoryRejectsUnsafeArtifactBeforeDial(t *testing.T) {
	for _, name := range []string{"unpinned", "corrupt", "partial", "changed", "latest", "unverified"} {
		t.Run(name, func(t *testing.T) {
			dials := 0
			factory := SSHRuntimeFactory{
				ResolveArtifact: testArtifactResolver(),
				VerifyArtifact:  func(string) error { return errors.New(name) },
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

func TestSSHRuntimeFactoryRejectsUnresolvableArtifactBeforeDial(t *testing.T) {
	dials := 0
	factory := SSHRuntimeFactory{
		ResolveArtifact: func() (string, error) { return "", errors.New("unsafe bundle") },
		VerifyArtifact:  func(string) error { t.Fatal("verifier was called"); return nil },
		Dial: func(context.Context, profile.Profile, []byte) (SSHRuntimeClient, error) {
			dials++
			return &runtimeClientFake{}, nil
		},
	}
	_, result := factory.Open(context.Background(), admittedSSH(), savedStep8Profile(t), []byte("opaque"))
	if result.Decision != DecisionTerminal || result.Class != ResultArtifactFailure || dials != 0 {
		t.Fatalf("result=%+v dials=%d", result, dials)
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
			client := &runtimeClientFake{ensureErr: tt.upload}
			factory := SSHRuntimeFactory{
				ResolveArtifact: testArtifactResolver(),
				VerifyArtifact:  func(string) error { return nil },
				Dial: func(ctx context.Context, _ profile.Profile, _ []byte) (SSHRuntimeClient, error) {
					if _, ok := ctx.Deadline(); !ok {
						t.Fatal("dial context is unbounded")
					}
					return client, nil
				},
				JavaReady: func(context.Context, profile.Profile) error { return tt.java },
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

func TestSSHRuntimeFactoryPropagatesOnlySafeUploadStages(t *testing.T) {
	for _, tt := range []struct {
		name, want string
		err        error
		class      ResultClass
	}{
		{"artifact stage", string(mapepirestdio.ArtifactStageDirectoryPrepare), mapepirestdio.NewArtifactError(mapepirestdio.ArtifactStageDirectoryPrepare), ResultUploadFailure},
		{"cancelled", "", context.Canceled, ResultCancelled},
		{"timeout", "", context.DeadlineExceeded, ResultOperationTimeout},
		{"invalid stage", "", mapepirestdio.NewArtifactError("arbitrary"), ResultUploadFailure},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := &runtimeClientFake{ensureErr: tt.err}
			factory := SSHRuntimeFactory{ResolveArtifact: testArtifactResolver(), VerifyArtifact: func(string) error { return nil }, Dial: func(context.Context, profile.Profile, []byte) (SSHRuntimeClient, error) { return client, nil }, JavaReady: func(context.Context, profile.Profile) error { return nil }}
			_, result := factory.Open(context.Background(), admittedSSH(), savedStep8Profile(t), []byte("opaque"))
			if result.Class != tt.class || string(result.ArtifactStage) != tt.want || result.Validate() != nil {
				t.Fatalf("result=%+v", result)
			}
		})
	}
	if (Step8Result{Decision: DecisionTerminal, Class: ResultUploadFailure, ArtifactStage: "arbitrary"}).Validate() == nil {
		t.Fatal("invalid upload-stage combination was accepted")
	}
}

func TestSSHRuntimeFactoryMapsDialTimeoutWithoutRuntime(t *testing.T) {
	factory := SSHRuntimeFactory{
		ResolveArtifact: testArtifactResolver(),
		VerifyArtifact:  func(string) error { return nil },
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

func TestSSHRuntimeFactoryRetainsGateCredentialUntilProofSettlement(t *testing.T) {
	secret := []byte("opaque")
	client := &runtimeClientFake{}
	factory := SSHRuntimeFactory{
		ResolveArtifact: testArtifactResolver(),
		VerifyArtifact:  func(string) error { return nil },
		Dial:            func(context.Context, profile.Profile, []byte) (SSHRuntimeClient, error) { return client, nil },
		JavaReady:       func(context.Context, profile.Profile) error { return nil },
	}
	runtime, result := factory.Open(context.Background(), admittedSSH(), savedStep8Profile(t), secret)
	if result.Class != ResultProofSuccess || runtime == nil {
		t.Fatalf("open result=%+v runtime=%v", result, runtime)
	}
	for _, b := range secret {
		if b == 0 {
			t.Fatal("runtime zeroed the gate credential before fixed proof")
		}
	}
	_, _ = runtime.Prove(context.Background(), savedStep8Profile(t), secret)
	for _, b := range secret {
		if b != 0 {
			t.Fatal("proof did not zero the credential after settlement")
		}
	}
}

func TestSSHRuntimeFactoryKeepsPrimaryFailureAndAssignsUniqueTraceIDs(t *testing.T) {
	clients := []*runtimeClientFake{{}, {}, {}, {}}
	index := 0
	factory := SSHRuntimeFactory{
		ResolveArtifact: testArtifactResolver(),
		VerifyArtifact:  func(string) error { return nil },
		Dial: func(context.Context, profile.Profile, []byte) (SSHRuntimeClient, error) {
			client := clients[index]
			index++
			return client, nil
		},
		JavaReady: func(context.Context, profile.Profile) error { return errors.New("java") },
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

func TestSSHRuntimeFactoryUsesResolvedArtifactInsteadOfProfilePath(t *testing.T) {
	client := &runtimeClientFake{}
	p := savedStep8Profile(t)
	p.MapepireJAR = "/caller/controlled.jar"
	t.Setenv("PATH", "/caller/controlled")
	workingDirectory := t.TempDir()
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workingDirectory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousDirectory) })
	factory := SSHRuntimeFactory{
		ResolveArtifact: testArtifactResolver(),
		VerifyArtifact: func(path string) error {
			if path != "/bundle/mapepire-server.jar" {
				t.Fatalf("verified path=%q", path)
			}
			return nil
		},
		Dial:      func(context.Context, profile.Profile, []byte) (SSHRuntimeClient, error) { return client, nil },
		JavaReady: func(context.Context, profile.Profile) error { return nil },
	}
	if _, result := factory.Open(context.Background(), admittedSSH(), p, []byte("opaque")); result.Class != ResultProofSuccess {
		t.Fatalf("result=%+v", result)
	}
	if client.artifact != "/bundle/mapepire-server.jar" {
		t.Fatalf("uploaded path=%q", client.artifact)
	}
}
