package audit

import (
	"context"
	"strings"
	"testing"
)

func TestTransportAuditAcceptsMetadataAndRejectsSensitivePayload(t *testing.T) {
	r := NewRecorder()
	if err := r.RecordTransport(context.Background(), TransportEvent{Transport: "ssh", Reason: "availability", Outcome: "selected", Protocol: "2.3.5"}); err != nil {
		t.Fatal(err)
	}
	if got := r.TransportEvents(); len(got) != 1 || got[0].Transport != "ssh" {
		t.Fatalf("events=%+v", got)
	}
	if err := r.RecordTransport(context.Background(), TransportEvent{Transport: "wss", Reason: "credential=" + strings.Repeat("x", 3), Outcome: "failed"}); err == nil {
		t.Fatal("sensitive reason accepted")
	}
}
