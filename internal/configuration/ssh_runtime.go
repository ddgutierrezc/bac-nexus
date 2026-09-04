package configuration

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"bac-nexus/internal/connectors/ibmi/mapepirestdio"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
)

const (
	SSHRuntimeOperationTimeout = 15 * time.Second
	SSHRuntimeProofTimeout     = 60 * time.Second
)

var sshRuntimeTraceSequence atomic.Uint64

// SSHRuntimeClient is the only remote surface the fallback runtime consumes.
type SSHRuntimeClient interface {
	Close() error
	EnsureMapepireServerJAR(context.Context, string) (mapepirestdio.VerifiedMapepireArtifactReceipt, error)
	FixedMapepireProof(context.Context, mapepirestdio.VerifiedMapepireArtifactReceipt, string, []byte) (remote.FixedProofMetadata, error)
}

type SSHRuntime struct {
	mu        sync.Mutex
	client    SSHRuntimeClient
	receipt   mapepirestdio.VerifiedMapepireArtifactReceipt
	requestID string
	traceID   uint64
	settled   bool
}

func (r *SSHRuntime) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	if r.client == nil || r.settled {
		r.mu.Unlock()
		return nil
	}
	client := r.client
	r.settled = true
	r.mu.Unlock()
	return client.Close()
}

func (r *SSHRuntime) activeClient() SSHRuntimeClient {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.settled {
		return nil
	}
	return r.client
}

type SSHRuntimeFactory struct {
	Dial            func(context.Context, profile.Profile, []byte) (SSHRuntimeClient, error)
	ResolveArtifact func() (string, error)
	VerifyArtifact  func(string) error
	JavaReady       func(context.Context, profile.Profile) error
}

func NewSSHRuntimeFactory() SSHRuntimeFactory {
	return SSHRuntimeFactory{
		Dial: func(ctx context.Context, p profile.Profile, secret []byte) (SSHRuntimeClient, error) {
			return remote.Dial(ctx, p, secret)
		},
		ResolveArtifact: newBundleArtifactResolver().Resolve,
		VerifyArtifact:  mapepirestdio.VerifyServerJAR,
		JavaReady: func(ctx context.Context, p profile.Profile) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			return mapepirestdio.ValidateJavaHome(p.JavaHome)
		},
	}
}

// Open consumes an already-admitted SSH fallback and retains only its verified runtime handle.
func (f SSHRuntimeFactory) Open(ctx context.Context, admission Step8Result, p profile.Profile, secret []byte) (runtime *SSHRuntime, result Step8Result) {
	result = Step8Result{RequestID: admission.RequestID}
	if admission.Decision != DecisionSSHEligible || admission.Class != ResultProofSuccess || admission.RequestID == "" || f.ResolveArtifact == nil || f.VerifyArtifact == nil {
		return nil, terminalGateResult(result, ResultDowngradeBlocked)
	}
	operation, cancel := context.WithTimeout(ctx, SSHRuntimeOperationTimeout)
	defer cancel()
	if err := operation.Err(); err != nil {
		return nil, terminalGateResult(result, runtimeContextClass(ctx))
	}
	artifact, err := f.ResolveArtifact()
	if err != nil || f.VerifyArtifact(artifact) != nil {
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
			result.Cleanup = client.Close() == nil
		}
	}()
	if f.JavaReady == nil {
		return nil, terminalGateResult(result, ResultDowngradeBlocked)
	}
	if err := f.JavaReady(operation, p); err != nil {
		return nil, terminalGateResult(result, runtimeErrorClass(ctx, err, ResultJavaFailure))
	}
	receipt, err := client.EnsureMapepireServerJAR(operation, artifact)
	if err != nil {
		return nil, terminalGateResult(result, runtimeErrorClass(ctx, err, ResultUploadFailure))
	}
	cleanup = false
	return &SSHRuntime{client: client, receipt: receipt, requestID: admission.RequestID, traceID: sshRuntimeTraceSequence.Add(1)}, Step8Result{RequestID: admission.RequestID, Decision: DecisionSSHEligible, Class: ResultProofSuccess}
}

// Prove starts only the release-owned single-mode process and fixed proof.
func (r *SSHRuntime) Prove(ctx context.Context, p profile.Profile, secret []byte) (metadata ProofMetadata, result Step8Result) {
	defer zeroCredential(secret)
	if r == nil {
		return ProofMetadata{}, terminalGateResult(Step8Result{}, ResultSessionFailure)
	}
	result = Step8Result{RequestID: r.requestID}
	client := r.activeClient()
	if client == nil {
		return ProofMetadata{}, terminalGateResult(result, ResultSessionFailure)
	}
	proofContext, cancel := context.WithTimeout(ctx, SSHRuntimeProofTimeout)
	defer cancel()
	defer func() {
		if r.Close() == nil {
			result.Cleanup = true
		}
	}()
	proof, err := client.FixedMapepireProof(proofContext, r.receipt, p.Username, secret)
	if err != nil {
		return ProofMetadata{}, terminalGateResult(result, proofErrorClass(proofContext, err))
	}
	metadata = ProofMetadata{Rows: proof.Rows, ProofRevision: proof.Revision}
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
