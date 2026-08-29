package audit

import (
	"context"
	"errors"
)

var ErrStep8EventRejected = errors.New("step8 audit event rejected")

// Step8Event is the complete allowlisted metadata surface for proof audit.
type Step8Event struct {
	Transport, Class, Revision string
	Cleanup                    bool
}

// Step8Recorder is a deterministic sink for sanitized proof events.
type Step8Recorder struct{ events []Step8Event }

func NewStep8Recorder() *Step8Recorder { return &Step8Recorder{} }
func (r *Step8Recorder) Record(ctx context.Context, e Step8Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ValidateStep8Event(e); err != nil {
		return err
	}
	r.events = append(r.events, e)
	return nil
}
func (r *Step8Recorder) Events() []Step8Event { return append([]Step8Event(nil), r.events...) }

func ValidateStep8Event(e Step8Event) error {
	if (e.Transport != "wss" && e.Transport != "ssh") || e.Revision != "values-1-v1" || !step8Class(e.Class) {
		return ErrStep8EventRejected
	}
	return nil
}
func step8Class(c string) bool {
	switch c {
	case "proof_success", "identity_failure", "trust_mismatch", "protocol_failure", "framing_failure", "malformed_response", "downgrade_blocked", "credentials_unavailable", "authentication_failed", "authorization_denied", "cancelled", "operation_timeout", "proof_timeout", "cleanup_timeout", "cleanup_failure", "limit_exceeded", "consent_declined_or_absent", "artifact_failure", "java_failure", "upload_failure", "launch_failure", "session_failure", "proof_failure":
		return true
	}
	return false
}
