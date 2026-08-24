package tui

import (
	"strings"

	"bac-nexus/internal/profile"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type profileStepFocus uint8

const (
	profileFocusName profileStepFocus = iota
	profileFocusCancel
	profileFocusContinue
)

const profileStepFooter = "Tab siguiente  •  Enter continuar  •  Esc volver  •  ? ayuda"

func newProfileNameInput() textinput.Model {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = ""
	input.CharLimit = 64
	input.Width = 36
	input.Focus()
	// A static block cursor keeps the real Bubbles editing position visible in
	// every terminal profile; focus itself remains the blinking-capable state.
	input.SetCursorMode(textinput.CursorStatic)
	input.Cursor.SetChar("█")
	return input
}

// profileNameState has exactly one non-empty result for non-empty input.
type profileNameState uint8

const (
	profileNameUnknown profileNameState = iota
	profileNameInvalid
	profileNameDuplicate
	profileNameAvailable
)

func (m Model) profileNameValidation() profileNameState {
	name := m.profileName.Value()
	if name == "" || !m.profilesLoaded || m.profilesLoadFailed {
		return profileNameUnknown
	}
	if profile.ValidateName(name) != nil {
		return profileNameInvalid
	}
	if m.profileNameDuplicate(name) {
		return profileNameDuplicate
	}
	return profileNameAvailable
}

// profileNameDuplicate compares strictly valid names case-insensitively.
// Profile file names may be case-sensitive, but presenting a case variant as
// available would create operationally confusing identities.
func (m Model) profileNameDuplicate(name string) bool {
	for _, existing := range m.profiles {
		if strings.EqualFold(existing.Name, name) {
			return true
		}
	}
	return false
}

func (m Model) profileNameValid() bool {
	return m.profileNameValidation() == profileNameAvailable
}

func (m Model) profileNameState() string {
	feedback, ok := m.profileNameFeedback()
	if !ok {
		return ""
	}
	prefix := "[ERR] "
	if feedback.kind == wizardFeedbackOK {
		prefix = "[OK] "
	}
	return prefix + feedback.message
}

func (m Model) profileNameFeedback() (wizardFeedback, bool) {
	switch m.profileNameValidation() {
	case profileNameInvalid:
		return wizardFeedback{kind: wizardFeedbackError, message: m.text("wizard.profile.invalid", nil)}, true
	case profileNameDuplicate:
		return wizardFeedback{kind: wizardFeedbackError, message: m.text("wizard.profile.duplicate", nil)}, true
	case profileNameAvailable:
		return wizardFeedback{kind: wizardFeedbackOK, message: m.text("wizard.profile.available", nil)}, true
	default:
		return wizardFeedback{}, false
	}
}

func (m Model) profileProgressGuard() wizardProgressGuard {
	if m.profileName.Value() == "" {
		return wizardProgressGuard{state: wizardProgressBlocked, feedback: wizardFeedback{kind: wizardFeedbackWarning, message: m.text("wizard.profile.required", nil)}}
	}
	if !m.profilesLoaded || m.profilesLoadFailed {
		return wizardProgressGuard{state: wizardProgressBlocked, feedback: wizardFeedback{kind: wizardFeedbackInfo, message: m.text("wizard.profile.loading", nil)}}
	}
	if feedback, ok := m.profileNameFeedback(); ok && feedback.kind == wizardFeedbackError {
		return wizardProgressGuard{state: wizardProgressBlocked, feedback: feedback}
	}
	return wizardProgressGuard{}
}

func (m *Model) activateProfileProgress() bool {
	return m.activateWizardProgress(m.profileProgressGuard(), func() { m.profileFocus = profileFocusName; m.profileName.Focus() })
}

func (m Model) updateProfileStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc", "ctrl+c":
		m.screen, m.status, m.err = screenHome, "", nil
		return m, nil
	case "?":
		m.status, m.err = m.text("wizard.profile.help", nil), nil
		return m, nil
	case "tab":
		return m.moveProfileFocus(1)
	case "shift+tab":
		return m.moveProfileFocus(-1)
	case "enter":
		switch m.profileFocus {
		case profileFocusName:
			if m.activateProfileProgress() {
				return m.moveProfileFocus(1 + int(profileFocusContinue-profileFocusCancel))
			}
			return m, nil
		case profileFocusCancel:
			m.screen, m.status, m.err = screenHome, "", nil
			return m, nil
		case profileFocusContinue:
			if !m.activateProfileProgress() {
				return m, nil
			}
			// This defensive normalization is a no-op after strict raw validation;
			// it must remain at the accepted-draft seam, never before validation.
			name := strings.TrimSpace(m.profileName.Value())
			return m, func() tea.Msg { return profileStepAcceptedMsg{name: name} }
		}
	}
	if m.profileFocus == profileFocusName {
		var cmd tea.Cmd
		m.profileName, cmd = m.profileName.Update(msg)
		m.status = ""
		return m, cmd
	}
	return m, nil
}

func (m Model) moveProfileFocus(delta int) (tea.Model, tea.Cmd) {
	m.status = ""
	count := int(profileFocusContinue) + 1
	next := (int(m.profileFocus) + delta + count) % count
	m.profileFocus = profileStepFocus(next)
	if m.profileFocus == profileFocusName {
		return m, m.profileName.Focus()
	}
	m.profileName.Blur()
	return m, nil
}

func (m Model) renderProfileStep() string {
	frameWidth, frameHeight := m.shellFrameDimensions()
	inner, height := m.shellInnerWidth(frameWidth), m.shellInnerHeight(frameHeight)
	t := newHomeTheme(m.noColor)
	footer := renderFooterText(inner, t, m.text("wizard.footer.profile", nil), m.buildInfo)
	return m.renderWizardShell(m.renderProfileStepHeader(inner, t), footer, m.renderProfileStepPanel(inner, height, t))
}

func (m Model) renderProfileStepHeader(width int, t homeTheme) string {
	return m.renderWizardHeader(width, t, m.text("wizard.profile.new", nil))
}

func profileStepTopGap(height int) int {
	if height >= 28 {
		return 3
	}
	if height >= 18 {
		return 1
	}
	return 0
}

func (m Model) renderProfileStepPanel(width, height int, t homeTheme) string {
	panel := newWizardPanelLayout(width, height, t)
	contentWidth := panel.contentWidth
	rhythm := newWizardRhythm(height)
	lines := renderWizardTitleRow(contentWidth, t, m.text("wizard.profile.title", nil), m.text("wizard.step.profile", nil))
	lines = appendWizardGap(lines, rhythm.titleDivider)
	lines = append(lines, renderWizardDivider(contentWidth, t))
	lines = append(lines, m.renderProfileNameLabel(contentWidth, t))
	lines = appendWizardGap(lines, rhythm.sectionDescription)
	lines = append(lines, m.profileStepGuidance(contentWidth, height, t)...)
	for _, line := range wrapWizardText(m.text("wizard.profile.example", nil), contentWidth, "") {
		lines = append(lines, t.metadata.Render(line))
	}
	lines = appendWizardGap(lines, rhythm.descriptionControl)
	lines = append(lines, m.renderProfileInput(contentWidth, t))
	if feedback, ok := m.wizardFeedbackForFeedback(m.profileNameFeedback()); ok {
		lines = appendWizardGap(lines, rhythm.feedback)
		lines = append(lines, renderWizardFeedback(contentWidth, t, feedback))
	}
	lines = appendWizardGap(lines, rhythm.actions)
	lines = append(lines, m.renderProfileActions(contentWidth, t))
	return panel.render(width, lines)
}

// profileStepFocusRange is derived from the same structured sections used to
// render the panel. It deliberately never searches rendered/ANSI text.
func (m Model) profileStepFocusRange(width, height int, t homeTheme) wizardLineRange {
	panel := newWizardPanelLayout(width, height, t)
	cw, rhythm := panel.contentWidth, newWizardRhythm(height)
	lines := len(renderWizardTitleRow(cw, t, m.text("wizard.profile.title", nil), m.text("wizard.step.profile", nil))) + rhythm.titleDivider + 1 + 1 + rhythm.sectionDescription + len(m.profileStepGuidance(cw, height, t)) + len(wrapWizardText(m.text("wizard.profile.example", nil), cw, "")) + rhythm.descriptionControl
	start := lines + 1
	start += panel.contentTopOffset - 1
	end := start + len(strings.Split(m.renderProfileInput(cw, t), "\n")) - 1
	if m.profileFocus == profileFocusCancel || m.profileFocus == profileFocusContinue {
		start = lines + len(strings.Split(m.renderProfileInput(cw, t), "\n"))
		if feedback, ok := m.wizardFeedbackForFeedback(m.profileNameFeedback()); ok {
			start += rhythm.feedback + len(strings.Split(renderWizardFeedback(cw, t, feedback), "\n"))
		}
		start += rhythm.actions + 1
		start += panel.contentTopOffset - 1
		if m.profileFocus == profileFocusContinue && len(strings.Split(m.renderProfileActions(cw, t), "\n")) > 1 {
			start++
		}
		end = start
	}
	return wizardLineRange{start: start, end: end}
}

func (m Model) profileStepGuidance(width, height int, t homeTheme) []string {
	_ = height
	rules := []string{
		m.text("wizard.profile.guidance_1", nil),
		m.text("wizard.profile.guidance_2", nil),
		m.text("wizard.profile.guidance_3", nil),
	}
	lines := make([]string, 0, len(rules))
	for _, rule := range rules {
		for _, line := range wrapWizardText(rule, width, "") {
			lines = append(lines, t.metadata.Render(line))
		}
	}
	return lines
}

// renderProfileStepTitleRow keeps the wizard title and step indicator aligned
// to opposite edges whenever their display widths leave a readable middle gap.
func renderProfileStepTitleRow(width int, t homeTheme) []string {
	return renderWizardTitleRow(width, t, "Crear perfil IBM i", "Paso 1 de 9 — Perfil")
}

func appendProfileStepGap(lines []string, size int) []string {
	for range size {
		lines = append(lines, "")
	}
	return lines
}

func profileStepSmallGap(height int) int {
	if height >= 30 {
		return 1
	}
	return 0
}

func profileStepInputGap(height int) int {
	if height >= 30 {
		return 2
	}
	if height >= 18 {
		return 1
	}
	return 0
}

func profileStepActionGap(height int) int {
	if height >= 30 {
		return 2
	}
	if height >= 18 {
		return 1
	}
	return 0
}

func (m Model) renderProfileNameLabel(width int, t homeTheme) string {
	return t.fieldsetContent.Render(wrapWizardText(m.text("wizard.profile.name_label", nil), width, "")[0])
}

func (m Model) renderProfileInput(width int, t homeTheme) string {
	return renderWizardInputRow(m.text("wizard.profile.label", nil), m.profileName, m.profileFocus == profileFocusName, width, t, wizardInputOptions{})
}

func (m Model) renderProfileActions(width int, t homeTheme) string {
	return renderWizardActions(width, t, m.text("action.cancel", nil), m.text("action.continue", nil), m.profileFocus == profileFocusCancel, m.profileFocus == profileFocusContinue, m.noColor)
}
