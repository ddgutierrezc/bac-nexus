package audit

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"bac-nexus/internal/configuration"
)

func TestStep8AuditEvidenceRejectsProhibitedValues(t *testing.T) {
	recorder := NewRecorder()
	auditor := NewStep8Auditor(recorder)

	for _, event := range []Step8Event{
		{Transport: "ibmi.example.test", Class: "proof_success", Revision: configuration.ProofRevision, Cleanup: true},
		{Transport: "wss", Class: "password=opaque", Revision: configuration.ProofRevision, Cleanup: true},
		{Transport: "wss", Class: "proof_success", Revision: "VALUES 1", Cleanup: true},
	} {
		if err := auditor.Record(context.Background(), event); err == nil {
			t.Fatalf("Record(%+v) succeeded", event)
		}
	}
	if got := recorder.Events(); len(got) != 0 {
		t.Fatalf("rejected events were recorded: %+v", got)
	}
}

func TestStep8AuditEvidenceHasBoundedCleanupAndNoReadinessMarker(t *testing.T) {
	recorder := NewRecorder()
	auditor := NewStep8Auditor(recorder)
	for _, event := range []Step8Event{
		{Transport: "wss", Class: "proof_success", Revision: configuration.ProofRevision, Cleanup: true},
		{Transport: "ssh", Class: "cleanup_failure", Revision: configuration.ProofRevision, Cleanup: false},
	} {
		if err := auditor.Record(context.Background(), event); err != nil {
			t.Fatalf("Record(%+v): %v", event, err)
		}
	}

	events := recorder.Events()
	if len(events) != 2 {
		t.Fatalf("recorded events=%d, want 2", len(events))
	}
	if events[0].Result != ResultClassAllow || events[1].Result != ResultClassError {
		t.Fatalf("results=%q/%q, want allow/error", events[0].Result, events[1].Result)
	}
	if !strings.HasSuffix(events[0].Reason, ":cleanup") || !strings.HasSuffix(events[1].Reason, ":incomplete") {
		t.Fatalf("cleanup lifecycle reasons=%q/%q", events[0].Reason, events[1].Reason)
	}
	for _, event := range events {
		if strings.Contains(event.Reason, "marker") || strings.Contains(event.Reason, "VALUES 1") {
			t.Fatalf("audit reason exposed prohibited state: %q", event.Reason)
		}
	}

	fields := reflect.VisibleFields(reflect.TypeOf(Step8Event{}))
	gotNames := make([]string, len(fields))
	for i, field := range fields {
		gotNames[i] = field.Name
	}
	if want := []string{"Transport", "Class", "Revision", "Cleanup"}; !reflect.DeepEqual(gotNames, want) {
		t.Fatalf("Step8Event fields=%v, want %v", gotNames, want)
	}
	marker := configuration.Marker{SchemaVersion: configuration.MarkerSchemaVersion, AtUnixMs: 1, Outcome: configuration.ResultProofSuccess, ProofRevision: configuration.ProofRevision}
	if configuration.MarkerIsReadiness(marker) {
		t.Fatal("historical marker established current readiness")
	}
}
