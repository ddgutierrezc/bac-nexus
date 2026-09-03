package tui

import "testing"

func TestActivateWizardProgressAllowsReadyAndBlocksInvalidContinuation(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		m := Model{}
		focused := false
		if !m.activateWizardProgress(wizardProgressGuard{state: wizardProgressReady}, func() { focused = true }) {
			t.Fatal("ready progress action was blocked")
		}
		if focused || m.status != "" {
			t.Fatalf("ready progress action changed focus or status: focused=%t status=%q", focused, m.status)
		}
	})

	t.Run("blocked", func(t *testing.T) {
		m := Model{}
		focused := false
		guard := wizardProgressGuard{state: wizardProgressBlocked, feedback: wizardFeedback{kind: wizardFeedbackError, message: "Complete the required field."}}
		if m.activateWizardProgress(guard, func() { focused = true }) {
			t.Fatal("blocked progress action was allowed")
		}
		if !focused || m.status != "[ERR] Complete the required field." {
			t.Fatalf("blocked progress action did not expose feedback and focus: focused=%t status=%q", focused, m.status)
		}
	})
}
