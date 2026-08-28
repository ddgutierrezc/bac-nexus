package tui

func (m Model) renderStep8Action() string {
	frameWidth, frameHeight := m.shellFrameDimensions()
	width, height := m.shellInnerWidth(frameWidth), m.shellInnerHeight(frameHeight)
	t := newHomeTheme(m.noColor)
	footer := renderFooterText(width, t, "Enter run  •  R retry  •  Esc back", m.buildInfo)
	return m.renderWizardShell(m.renderProfileConnectionHeader(width, t), footer, m.renderStep8ActionPanel(width, height, t))
}

func (m Model) renderStep8ActionPanel(width, height int, t homeTheme) string {
	panel := newWizardPanelLayout(width, height, t)
	feedback, action := step8ActionPresentation(m.step8Action.phase)
	if height < 18 {
		lines := []string{renderWizardFeedback(panel.contentWidth, t, feedback), action}
		return panel.render(width, lines)
	}
	lines := renderWizardTitleRow(panel.contentWidth, t, "Step 8 validation", "Step 8 of 9 — Validate")
	lines = append(lines, "", renderWizardDivider(panel.contentWidth, t), t.wizardContentHeading.Render("Authenticated proof"), "")
	lines = append(lines, renderWizardFeedback(panel.contentWidth, t, feedback), action)
	return panel.render(width, lines)
}

func step8ActionPresentation(phase step8ActionPhase) (wizardFeedback, string) {
	switch phase {
	case step8ActionPhaseRunning:
		return wizardFeedback{kind: wizardFeedbackInfo, message: "Validation is running. You can cancel safely."}, "▸ [ CANCEL ]"
	case step8ActionPhaseSuccess:
		return wizardFeedback{kind: wizardFeedbackOK, message: "Validation complete."}, "▸ [ BACK ]"
	case step8ActionPhaseCancelled:
		return wizardFeedback{kind: wizardFeedbackWarning, message: "Cancelled validation. Retry or go back."}, "▸ [ RETRY ]  < BACK >"
	case step8ActionPhaseTerminal:
		return wizardFeedback{kind: wizardFeedbackError, message: "Validation could not be completed. Retry or go back."}, "▸ [ RETRY ]  < BACK >"
	default:
		return wizardFeedback{kind: wizardFeedbackNeutral, message: "Ready to validate the saved profile."}, "▸ [ RUN VALIDATION ]  < BACK >"
	}
}
