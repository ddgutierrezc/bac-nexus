package configuration

import (
	"context"
	"errors"
	"time"

	"bac-nexus/internal/connectors/ibmi/mapepirestdio"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
)

const SSHRuntimeOperationTimeout = 15 * time.Second

// SSHRuntimeClient is the only remote surface the fallback runtime consumes.
type SSHRuntimeClient interface {
	Close() error
	RemoteFiles() mapepirestdio.RemoteFiles
	FixedMapepireProof(context.Context, mapepirestdio.LaunchPolicy, string, []byte) (remote.FixedProofMetadata, error)
}

type SSHRuntime struct {
	client    SSHRuntimeClient
	remoteJAR string
	requestID string
}

func (r *SSHRuntime) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Close()
}

type SSHRuntimeFactory struct {
	Dial           func(context.Context, profile.Profile, []byte) (SSHRuntimeClient, error)
	VerifyArtifact func(string) error
	JavaReady      func(context.Context, profile.Profile) error
	Upload         func(context.Context, mapepirestdio.RemoteFiles, string) (string, error)
}

func NewSSHRuntimeFactory() SSHRuntimeFactory {
	return SSHRuntimeFactory{
		Dial: func(ctx context.Context, p profile.Profile, secret []byte) (SSHRuntimeClient, error) {
			return remote.Dial(ctx, p, secret)
		},
		VerifyArtifact: mapepirestdio.VerifyServerJAR,
		JavaReady: func(ctx context.Context, p profile.Profile) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			return mapepirestdio.ValidateJavaHome(p.JavaHome)
		},
		Upload: func(ctx context.Context, files mapepirestdio.RemoteFiles, localPath string) (string, error) {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			return mapepirestdio.EnsureServerJAR(files, localPath)
		},
	}
}

// Open consumes an already-admitted SSH fallback and retains only its verified runtime handle.
func (f SSHRuntimeFactory) Open(ctx context.Context, admission Step8Result, p profile.Profile, secret []byte) (runtime *SSHRuntime, result Step8Result) {
	result = Step8Result{RequestID: admission.RequestID}
	defer zeroCredential(secret)
	if admission.Decision != DecisionSSHEligible || admission.Class != ResultProofSuccess || admission.RequestID == "" || f.VerifyArtifact == nil {
		return nil, terminalGateResult(result, ResultDowngradeBlocked)
	}
	operation, cancel := context.WithTimeout(ctx, SSHRuntimeOperationTimeout)
	defer cancel()
	if err := operation.Err(); err != nil {
		return nil, terminalGateResult(result, runtimeContextClass(ctx))
	}
	if err := f.VerifyArtifact(p.MapepireJAR); err != nil {
		return nil, terminalGateResult(result, ResultArtifactFailure)
	}
	if f.Dial == nil {
		return nil, terminalGateResult(result, ResultDowngradeBlocked)
	}
	client, err := f.Dial(operation, p, secret)
	if err != nil {
		return nil, terminalGateResult(result, runtimeErrorClass(ctx, err, ResultSessionFailure))
	}
	cleanup := true
	defer func() {
		if cleanup {
			if err := client.Close(); err != nil {
				runtime = nil
				result = terminalGateResult(result, ResultCleanupFailure)
				return
			}
			result.Cleanup = true
		}
	}()
	if f.JavaReady == nil {
		return nil, terminalGateResult(result, ResultDowngradeBlocked)
	}
	if err := f.JavaReady(operation, p); err != nil {
		return nil, terminalGateResult(result, runtimeErrorClass(ctx, err, ResultJavaFailure))
	}
	if f.Upload == nil {
		return nil, terminalGateResult(result, ResultDowngradeBlocked)
	}
	remoteJAR, err := f.Upload(operation, client.RemoteFiles(), p.MapepireJAR)
	if err != nil {
		return nil, terminalGateResult(result, runtimeErrorClass(ctx, err, ResultUploadFailure))
	}
	cleanup = false
	return &SSHRuntime{client: client, remoteJAR: remoteJAR, requestID: admission.RequestID}, Step8Result{RequestID: admission.RequestID, Decision: DecisionSSHEligible, Class: ResultProofSuccess}
}

// Prove starts only the release-owned single-mode process and fixed proof.
func (r *SSHRuntime) Prove(ctx context.Context, p profile.Profile, secret []byte) (ProofMetadata, Step8Result) {
	if r == nil {
		return ProofMetadata{}, terminalGateResult(Step8Result{}, ResultSessionFailure)
	}
	result := Step8Result{RequestID: r.requestID}
	if r.client == nil || r.remoteJAR == "" {
		return ProofMetadata{}, terminalGateResult(result, ResultSessionFailure)
	}
	proof, err := r.client.FixedMapepireProof(ctx, mapepirestdio.LaunchPolicy{JavaHome: p.JavaHome, RemoteJAR: r.remoteJAR, Consented: true}, p.Username, secret)
	if err != nil {
		return ProofMetadata{}, terminalGateResult(result, proofErrorClass(ctx, err))
	}
	metadata := ProofMetadata{Rows: proof.Rows, ProofRevision: proof.Revision}
	if err := ValidateProofMetadata(metadata); err != nil {
		return ProofMetadata{}, terminalGateResult(result, ResultProofFailure)
	}
	result.Decision = DecisionSSHEligible
	result.Class = ResultProofSuccess
	result.ProofRevision = metadata.ProofRevision
	return metadata, result
}

func proofErrorClass(ctx context.Context, err error) ResultClass {
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return ResultCancelled
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ResultProofTimeout
	}
	switch remote.ClassifyFixedProofError(err) {
	case remote.FixedProofLimitFailure:
		return ResultLimitExceeded
	case remote.FixedProofFramingFailure:
		return ResultFramingFailure
	case remote.FixedProofProtocolFailure:
		return ResultProtocolFailure
	}
	switch remote.FixedProofStageFor(err) {
	case remote.FixedProofLaunchStage:
		return ResultLaunchFailure
	case remote.FixedProofSessionStage:
		return ResultSessionFailure
	case remote.FixedProofRunStage:
		return ResultProofFailure
	default:
		return ResultProofFailure
	}
}

func runtimeContextClass(ctx context.Context) ResultClass {
	if errors.Is(ctx.Err(), context.Canceled) {
		return ResultCancelled
	}
	return ResultOperationTimeout
}

func runtimeErrorClass(ctx context.Context, err error, fallback ResultClass) ResultClass {
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return ResultCancelled
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return runtimeContextClass(ctx)
	}
	return fallback
}
