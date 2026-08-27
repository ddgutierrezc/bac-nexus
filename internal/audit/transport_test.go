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

func TestTransportAuditCarriesBoundedPolicyAndTrustMetadata(t *testing.T) {
	e := TransportEvent{Transport: "wss", Reason: "availability", Outcome: "selected", Protocol: "2.3.5", PolicyID: "verified-readonly", TrustOutcome: "verified", Version: "2.3.5"}
	if err := ValidateTransportEvent(e); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []TransportEvent{{PolicyID: "host.example"}, {TrustOutcome: "certificate-bytes"}, {Version: strings.Repeat("x", 65)}} {
		bad.Transport, bad.Outcome = "wss", "failed"
		if ValidateTransportEvent(bad) == nil {
			t.Fatalf("accepted unsafe audit metadata: %+v", bad)
		}
	}
}
