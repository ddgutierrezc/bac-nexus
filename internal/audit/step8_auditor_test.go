package audit

import (
	"context"
	"strings"
	"testing"
)

func TestStep8AuditorRecordsOnlyAllowlistedMetadata(t *testing.T) {
	recorder := NewRecorder()
	auditor := NewStep8Auditor(recorder)

	if err := auditor.Record(context.Background(), Step8Event{
		Transport: "wss", Class: "proof_success", Revision: "values-1-v1", Cleanup: true,
	}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	events := recorder.Events()
	if len(events) != 1 {
		t.Fatalf("recorded events = %d, want 1", len(events))
	}
	if got, want := events[0], (Event{
		Capability: CapabilityCatalogResolve, Connector: ConnectorIBMi, TargetClass: TargetClassIBMiCatalog,
		PolicyID: PolicyIDVerifiedReadOnly, Result: ResultClassAllow, Reason: "step8:wss:proof_success:values-1-v1:cleanup",
	}); got.Capability != want.Capability || got.Connector != want.Connector || got.TargetClass != want.TargetClass || got.PolicyID != want.PolicyID || got.Result != want.Result || got.Reason != want.Reason || got.Requested != 0 || got.Returned != 0 || got.Timestamp.IsZero() || got.Duration != 0 {
		t.Fatalf("recorded event = %+v, want bounded Step 8 metadata", got)
	}
}

func TestStep8AuditorRejectsForbiddenValuesWithoutRecording(t *testing.T) {
	sentinels := []string{
		"https://endpoint.invalid:8076", "host=ibmi.internal", "user=QPGMR", "path=/tmp/proof",
		"error=connection refused", "SQL=VALUES 1", "rows=[secret]", "password=super-secret",
	}
	for _, sentinel := range sentinels {
		for _, field := range []string{"transport", "class", "revision"} {
			t.Run(field+"/"+sentinel, func(t *testing.T) {
				recorder := NewRecorder()
				auditor := NewStep8Auditor(recorder)
				event := Step8Event{Transport: "wss", Class: "proof_success", Revision: "values-1-v1", Cleanup: true}
				switch field {
				case "transport":
					event.Transport = sentinel
				case "class":
					event.Class = sentinel
				case "revision":
					event.Revision = sentinel
				}

				if err := auditor.Record(context.Background(), event); err == nil {
					t.Fatal("Record() error = nil, want rejection")
				}
				if got := recorder.Events(); len(got) != 0 {
					t.Fatalf("rejected event recorded: %+v", got)
				}
				for _, recorded := range recorder.Events() {
					if strings.Contains(recorded.Reason, sentinel) {
						t.Fatalf("forbidden value appeared in audit output: %q", recorded.Reason)
					}
				}
			})
		}
	}
}

func TestStep8AuditorRecordsFailureAndCleanupState(t *testing.T) {
	recorder := NewRecorder()
	auditor := NewStep8Auditor(recorder)
	if err := auditor.Record(context.Background(), Step8Event{
		Transport: "ssh", Class: "trust_mismatch", Revision: "values-1-v1",
	}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	events := recorder.Events()
	if len(events) != 1 || events[0].Result != ResultClassError || events[0].Reason != "step8:ssh:trust_mismatch:values-1-v1:incomplete" {
		t.Fatalf("recorded events = %+v, want bounded failed Step 8 audit", events)
	}
}
