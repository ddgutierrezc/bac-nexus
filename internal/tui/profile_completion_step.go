package tui

type profileCompletionOutcome string

const (
	profileCompletionOmitted    profileCompletionOutcome = "omitted"
	profileCompletionCancelled  profileCompletionOutcome = "cancelled"
	profileCompletionFailed     profileCompletionOutcome = "failed"
	profileCompletionSuccessful profileCompletionOutcome = "successful"
)

func (m Model) renderProfileCompletionStep() string {
	fw, fh := m.shellFrameDimensions()
	w, h := m.shellInnerWidth(fw), m.shellInnerHeight(fh)
	t := newHomeTheme(m.noColor)
	return m.renderWizardShell(m.renderProfileConnectionHeader(w, t), renderFooterText(w, t, "Enter: finish", m.buildInfo), m.renderProfileCompletionPanel(w, h, t))
}

func (m Model) renderProfileCompletionPanel(w, h int, t homeTheme) string {
	panel := newWizardPanelLayout(w, h, t)
	feedback := wizardFeedback{kind: wizardFeedbackInfo, message: "Proof was omitted. Local profile configuration is saved."}
	if m.profileCompletion == profileCompletionCancelled {
		feedback = wizardFeedback{kind: wizardFeedbackWarning, message: "Proof was cancelled. Local profile configuration is saved."}
	}
	if m.profileCompletion == profileCompletionFailed {
		feedback = wizardFeedback{kind: wizardFeedbackError, message: "Proof did not complete. Local profile configuration is saved."}
	}
	if m.profileCompletion == profileCompletionSuccessful {
		feedback = wizardFeedback{kind: wizardFeedbackOK, message: "Proof successful; ready for controlled validation."}
	}
	lines := append(renderWizardTitleRow(panel.contentWidth, t, "Completion", "Step 8 of 8"), "", renderWizardDivider(panel.contentWidth, t), "", "Outcome: "+string(m.profileCompletion), "", renderWizardFeedback(panel.contentWidth, t, feedback), "", "▸ [ FINISH ]")
	return panel.render(w, lines)
}
