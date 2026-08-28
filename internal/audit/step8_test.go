package audit

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestStep8RecorderAllowsOnlyBoundedMetadata(t *testing.T) {
	r := NewStep8Recorder()
	e := Step8Event{Transport: "wss", Class: "proof_success", Revision: "values-1-v1", Cleanup: true}
	if err := r.Record(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	got := r.Events()
	if len(got) != 1 || got[0] != e {
		t.Fatalf("events = %#v", got)
	}
}

func TestStep8RecorderRejectsSensitiveOrUnboundedValues(t *testing.T) {
	for _, e := range []Step8Event{
		{Transport: "https://secret", Class: "proof_success", Revision: "values-1-v1"},
		{Transport: "wss", Class: "password=secret", Revision: "values-1-v1"},
		{Transport: "wss", Class: "proof_success", Revision: strings.Repeat("x", 65)},
	} {
		if err := ValidateStep8Event(e); !errors.Is(err, ErrStep8EventRejected) {
			t.Fatalf("event %#v = %v", e, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := NewStep8Recorder().Record(ctx, Step8Event{Transport: "ssh", Class: "cleanup_failure", Revision: "values-1-v1"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled audit = %v", err)
	}
}
