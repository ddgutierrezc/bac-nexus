package audit

import (
	"context"
	"strings"
	"testing"

	"bac-nexus/internal/connectors/ibmi/mapepirestdio"
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

func TestStep8AuditorRecordsTerminalFailureWithoutRevision(t *testing.T) {
	recorder := NewRecorder()
	if err := NewStep8Auditor(recorder).Record(context.Background(), Step8Event{Transport: "ssh", Class: "upload_failure", ArtifactStage: mapepirestdio.ArtifactStageTransfer}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	events := recorder.Events()
	if len(events) != 1 || events[0].Reason != "step8:ssh:upload_failure:artifact_stage:transfer:incomplete" {
		t.Fatalf("failed Step 8 event was not recorded: %+v", events)
	}
}

func TestStep8AuditorRecordsOnlyAllowlistedLaunchFailureStages(t *testing.T) {
	for _, class := range []string{"launch_receipt_binding_invalid", "launch_reverify_stat_failure", "launch_reverify_artifact_invalid", "launch_reverify_open_failure", "launch_reverify_read_failure", "launch_reverify_size_changed", "launch_reverify_hash_mismatch", "launch_command_policy_failure", "launch_new_session_prohibited", "launch_new_session_connection_failed", "launch_new_session_unknown_channel_type", "launch_new_session_resource_shortage", "launch_new_session_failure", "launch_stdin_failure", "launch_stdout_failure", "launch_start_failure"} {
		t.Run(class, func(t *testing.T) {
			recorder := NewRecorder()
			if err := NewStep8Auditor(recorder).Record(context.Background(), Step8Event{Transport: "ssh", Class: class}); err != nil {
				t.Fatalf("Record() error = %v", err)
			}
			events := recorder.Events()
			if len(events) != 1 || events[0].Reason != "step8:ssh:"+class+":incomplete" {
				t.Fatalf("recorded events = %+v, want allowlisted launch stage", events)
			}
		})
	}
}

func TestStep8AuditorRejectsInvalidArtifactStageCombinations(t *testing.T) {
	for _, event := range []Step8Event{
		{Transport: "wss", Class: "upload_failure", ArtifactStage: mapepirestdio.ArtifactStageTransfer},
		{Transport: "ssh", Class: "authentication_failed", ArtifactStage: mapepirestdio.ArtifactStageTransfer},
		{Transport: "ssh", Class: "upload_failure", ArtifactStage: "sftp://user@host/path raw error"},
		{Transport: "ssh", Class: "proof_success"},
	} {
		recorder := NewRecorder()
		if err := NewStep8Auditor(recorder).Record(context.Background(), event); err == nil || len(recorder.Events()) != 0 {
			t.Fatalf("invalid event accepted: %+v", event)
		}
	}
}
