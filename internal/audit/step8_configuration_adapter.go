package audit

import (
	"context"

	"bac-nexus/internal/configuration"
)

// Step8ConfigurationAdapter adapts bounded configuration events to Step8Auditor.
type Step8ConfigurationAdapter struct{ auditor *Step8Auditor }

func NewStep8ConfigurationAdapter(auditor *Step8Auditor) Step8ConfigurationAdapter {
	return Step8ConfigurationAdapter{auditor: auditor}
}

func (a Step8ConfigurationAdapter) Record(ctx context.Context, event configuration.Step8AuditEvent) error {
	return a.auditor.Record(ctx, Step8Event{
		Transport: string(event.Transport),
		Class:     string(event.Class),
		Revision:  event.Revision,
		Cleanup:   event.Cleanup,
	})
}
