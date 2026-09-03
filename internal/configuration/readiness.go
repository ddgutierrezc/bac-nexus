package configuration

import (
	"context"
	"errors"
	"time"
)

type ProductStatus string

const ReadyForControlledIBMiValidation ProductStatus = "ready_for_controlled_ibmi_validation"

type ValidationStatus string

const NotValidatedOnIBMi ValidationStatus = "not_validated_on_ibmi"

type ReadinessCheck struct {
	Name   string
	Status string
}

type ReadinessReport struct {
	ProductStatus         ProductStatus
	ValidationStatus      ValidationStatus
	Checks                []ReadinessCheck
	RemoteContacted       bool
	Transport             Transport
	AuthenticationPending bool
}

func CheckLocalReadiness() ReadinessReport {
	return ReadinessReport{
		ProductStatus:    ReadyForControlledIBMiValidation,
		ValidationStatus: NotValidatedOnIBMi,
		Checks: []ReadinessCheck{
			{Name: "recovery", Status: "missing from nexus serve composition"},
			{Name: "resolver", Status: "missing from nexus serve composition"},
			{Name: "acquirer", Status: "missing from nexus serve composition"},
			{Name: "lease", Status: "missing from nexus serve composition"},
		},
	}
}

func (r ReadinessReport) Summary() string {
	result := "local readiness: nexus serve composition gap"
	for _, check := range r.Checks {
		result += "; " + check.Name + "=" + check.Status
	}
	return result
}

type DiagnosticClassification string

const (
	DiagnosticSucceeded DiagnosticClassification = "diagnostic_succeeded"
	DiagnosticCancelled DiagnosticClassification = "diagnostic_cancelled"
	DiagnosticTimedOut  DiagnosticClassification = "diagnostic_timed_out"
	DiagnosticFailed    DiagnosticClassification = "diagnostic_failed"
)

type DiagnosticResult struct {
	Classification   DiagnosticClassification
	ProductStatus    ProductStatus
	ValidationStatus ValidationStatus
	Detail           string
}

type DiagnosticAuditEvent struct {
	Classification DiagnosticClassification
	Detail         string
	Duration       time.Duration
}

type DiagnosticAuditor interface {
	Record(context.Context, DiagnosticAuditEvent) error
}

type DiagnosticRunner interface {
	Run(context.Context) error
}

func RunRemoteDiagnostic(parent context.Context, runner DiagnosticRunner, timeout time.Duration, auditor DiagnosticAuditor) DiagnosticResult {
	base := DiagnosticResult{ProductStatus: ReadyForControlledIBMiValidation, ValidationStatus: NotValidatedOnIBMi}
	if parent == nil {
		parent = context.Background()
	}
	if runner == nil {
		return finishDiagnostic(base, DiagnosticFailed, "diagnostic unavailable", 0, auditor)
	}
	if err := parent.Err(); err != nil {
		return finishDiagnostic(base, DiagnosticCancelled, "diagnostic cancelled", 0, auditor)
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	started := time.Now()
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()
	select {
	case err := <-done:
		if err == nil {
			return finishDiagnostic(base, DiagnosticSucceeded, "diagnostic completed", time.Since(started), auditor)
		}
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return finishDiagnostic(base, DiagnosticCancelled, "diagnostic cancelled", time.Since(started), auditor)
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return finishDiagnostic(base, DiagnosticTimedOut, "diagnostic timed out", time.Since(started), auditor)
		}
		return finishDiagnostic(base, DiagnosticFailed, "diagnostic failed", time.Since(started), auditor)
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return finishDiagnostic(base, DiagnosticTimedOut, "diagnostic timed out", time.Since(started), auditor)
		}
		return finishDiagnostic(base, DiagnosticCancelled, "diagnostic cancelled", time.Since(started), auditor)
	}
}

func finishDiagnostic(result DiagnosticResult, classification DiagnosticClassification, detail string, duration time.Duration, auditor DiagnosticAuditor) DiagnosticResult {
	result.Classification, result.Detail = classification, detail
	if auditor != nil {
		if auditor.Record(context.Background(), DiagnosticAuditEvent{Classification: classification, Detail: detail, Duration: duration}) != nil {
			result.Classification, result.Detail = DiagnosticFailed, "diagnostic evidence unavailable"
		}
	}
	return result
}
