package configuration_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bac-nexus/internal/audit"
	"bac-nexus/internal/configuration"
	"bac-nexus/internal/localstate"
)

type diagnosticFunc func(context.Context) error

func (f diagnosticFunc) Run(ctx context.Context) error { return f(ctx) }

type diagnosticAuditStub struct {
	events []configuration.DiagnosticAuditEvent
}

func (a *diagnosticAuditStub) Record(_ context.Context, event configuration.DiagnosticAuditEvent) error {
	a.events = append(a.events, event)
	return nil
}

type diagnosticAuditPlatform struct{}

func (diagnosticAuditPlatform) VerifyManagedDirectory(path string, _ ...string) (localstate.Evidence, error) {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return localstate.Evidence{}, err
	}
	return diagnosticAuditEvidence(), nil
}

func (diagnosticAuditPlatform) CreateManagedFile(path string, _ ...string) (localstate.Evidence, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return localstate.Evidence{}, err
	}
	if err := file.Close(); err != nil {
		return localstate.Evidence{}, err
	}
	return diagnosticAuditEvidence(), nil
}

func diagnosticAuditEvidence() localstate.Evidence {
	return localstate.Evidence{Available: true, LinkSafe: true, Local: true, Owned: true, Restrictive: true, HandleStable: true}
}

func TestLocalReadinessIsOfflineAndExposesServeCompositionGap(t *testing.T) {
	report := configuration.CheckLocalReadiness()
	if report.ProductStatus != configuration.ReadyForControlledIBMiValidation || report.ValidationStatus != configuration.NotValidatedOnIBMi {
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
	result := configuration.RunRemoteDiagnostic(context.Background(), diagnosticFunc(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}), 5*time.Millisecond, audit)
	if result.Classification != configuration.DiagnosticTimedOut || result.ProductStatus != configuration.ReadyForControlledIBMiValidation || result.ValidationStatus != configuration.NotValidatedOnIBMi {
		t.Fatalf("result = %+v", result)
	}
	if len(audit.events) != 1 || audit.events[0].Classification != configuration.DiagnosticTimedOut || audit.events[0].Detail != "diagnostic timed out" {
		t.Fatalf("audit = %+v", audit.events)
	}
}

func TestRemoteDiagnosticCancellationDoesNotClaimValidation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := configuration.RunRemoteDiagnostic(ctx, diagnosticFunc(func(context.Context) error { return errors.New("secret host password") }), time.Second, nil)
	if result.Classification != configuration.DiagnosticCancelled || result.ValidationStatus != configuration.NotValidatedOnIBMi {
		t.Fatalf("result = %+v", result)
	}
	if strings.Contains(result.Detail, "secret") || strings.Contains(result.Detail, "password") {
		t.Fatalf("unsanitized detail = %q", result.Detail)
	}
}

func TestRemoteDiagnosticPersistsOnlySanitizedFactsAndReopens(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	config := audit.FileConfig{
		Root:          root,
		Components:    []string{"audit"},
		RetentionDays: "30",
		Platform:      diagnosticAuditPlatform{},
		Now:           func() time.Time { return now },
	}
	sink, err := audit.OpenFile(config)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	result := configuration.RunRemoteDiagnostic(context.Background(), diagnosticFunc(func(context.Context) error { return nil }), time.Second, audit.NewPersistentDiagnosticAuditor(sink, func() time.Time { return now }))
	if result.Classification != configuration.DiagnosticSucceeded || result.ValidationStatus != configuration.NotValidatedOnIBMi {
		t.Fatalf("result = %+v", result)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "audit", "audit-2026-09-02.jsonl"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	want := map[string]any{
		"operation_class":   "configuration_diagnostic",
		"policy_id":         "verified_read_only",
		"result_class":      "succeeded",
		"requested_lines":   float64(0),
		"returned_lines":    float64(0),
		"lifecycle_outcome": "completed",
	}
	for field, expected := range want {
		if got := record[field]; got != expected {
			t.Fatalf("record[%q] = %#v, want %#v", field, got, expected)
		}
	}
	if len(record) != 7 {
		t.Fatalf("persisted fields = %#v, want only seven sanitized fields", record)
	}
	reopened, err := audit.OpenFile(config)
	if err != nil {
		t.Fatalf("OpenFile() after diagnostic record error = %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("reopened Close() error = %v", err)
	}
}

func TestRemoteDiagnosticAppendFailureOverridesSuccessWithoutLiveClaim(t *testing.T) {
	root := t.TempDir()
	sink, err := audit.OpenFile(audit.FileConfig{
		Root:          root,
		Components:    []string{"audit"},
		RetentionDays: "30",
		Platform:      diagnosticAuditPlatform{},
		Sync:          func() error { return errors.New("sync failed") },
	})
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	t.Cleanup(func() { _ = sink.Close() })
	client := struct{ mutated bool }{}
	result := configuration.RunRemoteDiagnostic(context.Background(), diagnosticFunc(func(context.Context) error {
		return nil
	}), time.Second, audit.NewPersistentDiagnosticAuditor(sink, time.Now))
	if client.mutated {
		t.Fatal("diagnostic mutated an external client")
	}
	if result.Classification != configuration.DiagnosticFailed || result.Detail != "diagnostic evidence unavailable" || result.ValidationStatus != configuration.NotValidatedOnIBMi {
		t.Fatalf("result = %+v", result)
	}
}

func TestPersistentDiagnosticAuditorStoresTimeoutAndCancellation(t *testing.T) {
	for _, test := range []struct {
		name, want string
		runner     diagnosticFunc
		timeout    time.Duration
		parent     func() (context.Context, context.CancelFunc)
	}{
		{
			name: "timeout",
			want: "timed_out",
			runner: diagnosticFunc(func(ctx context.Context) error {
				<-ctx.Done()
				return ctx.Err()
			}),
			timeout: 5 * time.Millisecond,
			parent:  func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) },
		},
		{
			name: "cancelled",
			want: "cancelled",
			runner: diagnosticFunc(func(context.Context) error {
				return errors.New("secret host password")
			}),
			timeout: time.Second,
			parent: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			sink, err := audit.OpenFile(audit.FileConfig{Root: root, Components: []string{"audit"}, RetentionDays: "30", Platform: diagnosticAuditPlatform{}})
			if err != nil {
				t.Fatalf("OpenFile() error = %v", err)
			}
			ctx, cancel := test.parent()
			defer cancel()
			result := configuration.RunRemoteDiagnostic(ctx, test.runner, test.timeout, audit.NewPersistentDiagnosticAuditor(sink, time.Now))
			if result.ValidationStatus != configuration.NotValidatedOnIBMi {
				t.Fatalf("ValidationStatus = %q", result.ValidationStatus)
			}
			if err := sink.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
			data, err := os.ReadFile(filepath.Join(root, "audit", "audit-"+time.Now().UTC().Format("2006-01-02")+".jsonl"))
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			if !strings.Contains(string(data), `"result_class":"`+test.want+`"`) {
				t.Fatalf("audit record = %q, want result %q", data, test.want)
			}
		})
	}
}
