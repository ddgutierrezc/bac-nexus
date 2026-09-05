package audit

import (
	"context"
	"errors"

	"bac-nexus/internal/connectors/ibmi/mapepirestdio"
)

var ErrStep8EventRejected = errors.New("step8 audit event rejected")

// Step8Event is the complete allowlisted metadata surface for proof audit.
type Step8Event struct {
	Transport, Class, Revision string
	Cleanup                    bool
	ArtifactStage              mapepirestdio.ArtifactStage
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
	if (e.Transport != "wss" && e.Transport != "ssh") || !step8Class(e.Class) {
		return ErrStep8EventRejected
	}
	if e.Class == "proof_success" && e.Revision != "values-1-v1" {
		return ErrStep8EventRejected
	}
	if e.Revision != "" && e.Revision != "values-1-v1" {
		return ErrStep8EventRejected
	}
	if e.ArtifactStage != "" && (e.Transport != "ssh" || e.Class != "upload_failure" || !mapepirestdio.ValidArtifactStage(e.ArtifactStage)) {
		return ErrStep8EventRejected
	}
	return nil
}
func step8Class(c string) bool {
	switch c {
	case "proof_success", "identity_failure", "trust_mismatch", "protocol_failure", "framing_failure", "malformed_response", "downgrade_blocked", "credentials_unavailable", "authentication_failed", "authorization_denied", "cancelled", "operation_timeout", "proof_timeout", "cleanup_timeout", "cleanup_failure", "limit_exceeded", "consent_declined_or_absent", "artifact_failure", "java_failure", "upload_failure", "launch_failure", "launch_receipt_binding_invalid", "launch_reverify_stat_failure", "launch_reverify_artifact_invalid", "launch_reverify_open_failure", "launch_reverify_read_failure", "launch_reverify_size_changed", "launch_reverify_hash_mismatch", "launch_command_policy_failure", "launch_new_session_prohibited", "launch_new_session_connection_failed", "launch_new_session_unknown_channel_type", "launch_new_session_resource_shortage", "launch_new_session_failure", "launch_session_failure", "launch_stdin_failure", "launch_stdout_failure", "launch_start_failure", "session_failure", "proof_failure":
		return true
	}
	return false
}
