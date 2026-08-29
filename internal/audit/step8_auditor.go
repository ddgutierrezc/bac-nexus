package audit

import (
	"context"
	"time"
)

// Step8Auditor records the fixed Step 8 taxonomy through the existing Recorder.
type Step8Auditor struct{ recorder *Recorder }

// NewStep8Auditor binds Step 8 audit events to the existing allowlisted recorder.
func NewStep8Auditor(recorder *Recorder) *Step8Auditor { return &Step8Auditor{recorder: recorder} }

// Record validates fixed Step 8 metadata before constructing a Recorder event.
func (a *Step8Auditor) Record(ctx context.Context, event Step8Event) error {
	if a == nil || a.recorder == nil {
		return ErrStep8EventRejected
	}
	if err := ValidateStep8Event(event); err != nil {
		return err
	}
	result := ResultClassError
	if event.Class == "proof_success" {
		result = ResultClassAllow
	}
	return a.recorder.Record(ctx, Event{
		Capability:  CapabilityCatalogResolve,
		Connector:   ConnectorIBMi,
		TargetClass: TargetClassIBMiCatalog,
		PolicyID:    PolicyIDVerifiedReadOnly,
		Result:      result,
		Timestamp:   time.Now().UTC(),
		Reason:      "step8:" + event.Transport + ":" + event.Class + ":" + event.Revision + cleanupSuffix(event.Cleanup),
	})
}

func cleanupSuffix(cleanup bool) string {
	if cleanup {
		return ":cleanup"
	}
	return ":incomplete"
}
