package configuration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type diagnosticFunc func(context.Context) error

func (f diagnosticFunc) Run(ctx context.Context) error { return f(ctx) }

type diagnosticAuditStub struct{ events []DiagnosticAuditEvent }

func (a *diagnosticAuditStub) Record(_ context.Context, event DiagnosticAuditEvent) error {
	a.events = append(a.events, event)
	return nil
}

func TestLocalReadinessIsOfflineAndExposesServeCompositionGap(t *testing.T) {
	report := CheckLocalReadiness()
	if report.ProductStatus != ReadyForControlledIBMiValidation || report.ValidationStatus != NotValidatedOnIBMi {
		t.Fatalf("statuses = %q/%q", report.ProductStatus, report.ValidationStatus)
	}
	for _, component := range []string{"recovery", "resolver", "acquirer", "lease"} {
		if !strings.Contains(report.Summary(), component) {
			t.Fatalf("readiness summary omits missing %s: %q", component, report.Summary())
		}
	}
	if report.RemoteContacted {
		t.Fatal("local readiness must not contact a remote system")
	}
}

func TestRemoteDiagnosticTimeoutIsSanitizedAndPreservesStatuses(t *testing.T) {
	audit := &diagnosticAuditStub{}
	result := RunRemoteDiagnostic(context.Background(), diagnosticFunc(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}), 5*time.Millisecond, audit)
	if result.Classification != DiagnosticTimedOut || result.ProductStatus != ReadyForControlledIBMiValidation || result.ValidationStatus != NotValidatedOnIBMi {
		t.Fatalf("result = %+v", result)
	}
	if len(audit.events) != 1 || audit.events[0].Classification != DiagnosticTimedOut || audit.events[0].Detail != "diagnostic timed out" {
		t.Fatalf("audit = %+v", audit.events)
	}
}

func TestRemoteDiagnosticCancellationDoesNotClaimValidation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := RunRemoteDiagnostic(ctx, diagnosticFunc(func(context.Context) error { return errors.New("secret host password") }), time.Second, nil)
	if result.Classification != DiagnosticCancelled || result.ValidationStatus != NotValidatedOnIBMi {
		t.Fatalf("result = %+v", result)
	}
	if strings.Contains(result.Detail, "secret") || strings.Contains(result.Detail, "password") {
		t.Fatalf("unsanitized detail = %q", result.Detail)
	}
}
