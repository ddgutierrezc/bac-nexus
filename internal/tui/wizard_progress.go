package tui

// wizardProgressState distinguishes executable, actionable-but-blocked, and
// genuinely unavailable controls without coupling guards to a specific step.
type wizardProgressState uint8

const (
	wizardProgressReady wizardProgressState = iota
	wizardProgressBlocked
	wizardProgressDisabled
)

type wizardProgressGuard struct {
	state    wizardProgressState
	feedback wizardFeedback
}

func (m *Model) activateWizardProgress(guard wizardProgressGuard, focus func()) bool {
	if guard.state == wizardProgressReady {
		return true
	}
	if guard.state == wizardProgressBlocked {
		m.status = wizardFeedbackRow(guard.feedback)
		if focus != nil {
			focus()
		}
	}
	return false
}

func wizardFeedbackRow(feedback wizardFeedback) string {
	marker := "[--]"
	switch feedback.kind {
	case wizardFeedbackOK:
		marker = "[OK]"
	case wizardFeedbackInfo:
		marker = "[INFO]"
	case wizardFeedbackWarning:
		marker = "[WARN]"
	case wizardFeedbackError:
		marker = "[ERR]"
	}
	return marker + " " + feedback.message
}
