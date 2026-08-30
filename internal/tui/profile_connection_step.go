package tui

import (
	"strconv"
	"strings"

	"bac-nexus/internal/profile"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type profileConnectionFocus uint8

const (
	profileConnectionFocusHost profileConnectionFocus = iota
	profileConnectionFocusUsername
	profileConnectionFocusPort
	profileConnectionFocusBack
	profileConnectionFocusContinue
)

const (
	profileConnectionStepFooter = "Tab siguiente  •  Enter continuar  •  Esc volver  •  ? ayuda"
	connectionPanelTitle        = "Crear perfil IBM i"
	connectionStepIndicator     = "Paso 2 de 8 — Conexión"
)

func newProfileConnectionInput(limit int) textinput.Model {
	input := textinput.New()
	input.Prompt, input.Placeholder, input.CharLimit, input.Width = "", "", limit, 36
	input.SetCursorMode(textinput.CursorStatic)
	input.Cursor.SetChar("█")
	return input
}

func (m *Model) focusProfileConnectionInput() {
	m.connectionHost.Blur()
	m.connectionUsername.Blur()
	m.connectionPort.Blur()
	switch m.connectionFocus {
	case profileConnectionFocusHost:
		m.connectionHost.Focus()
	case profileConnectionFocusUsername:
		m.connectionUsername.Focus()
	case profileConnectionFocusPort:
		m.connectionPort.Focus()
	}
}

func (m Model) updateProfileConnectionStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.screen, m.status, m.err = screenProfileStep, "", nil
		m.profileFocus = profileFocusName
		m.profileName.Focus()
		return m, nil
	case "?":
		m.status, m.err = m.text("wizard.connection.help", nil), nil
		return m, nil
	case "tab":
		return m.moveProfileConnectionFocus(1)
	case "shift+tab":
		return m.moveProfileConnectionFocus(-1)
	case "enter":
		switch m.connectionFocus {
		case profileConnectionFocusHost, profileConnectionFocusUsername, profileConnectionFocusPort:
			if m.connectionFieldValid(m.connectionFocus) {
				return m.moveProfileConnectionFocus(1)
			}
			guard, focus := m.connectionProgressGuard()
			m.activateWizardProgress(guard, func() { m.connectionFocus = focus; m.connectionValidate = true; m.focusProfileConnectionInput() })
			return m, nil
		case profileConnectionFocusBack:
			m.screen, m.status, m.err = screenProfileStep, "", nil
			m.profileFocus = profileFocusName
			m.profileName.Focus()
			return m, nil
		case profileConnectionFocusContinue:
			guard, focus := m.connectionProgressGuard()
			if !m.activateWizardProgress(guard, func() { m.connectionFocus = focus; m.connectionValidate = true; m.focusProfileConnectionInput() }) {
				return m, nil
			}
			host := strings.TrimSpace(m.connectionHost.Value())
			username := strings.TrimSpace(m.connectionUsername.Value())
			port, _ := strconv.Atoi(m.connectionPort.Value())
			return m, func() tea.Msg { return profileConnectionAcceptedMsg{host: host, username: username, port: port} }
		}
	}

	var cmd tea.Cmd
	switch m.connectionFocus {
	case profileConnectionFocusHost:
		m.connectionHost, cmd = m.connectionHost.Update(msg)
	case profileConnectionFocusUsername:
		m.connectionUsername, cmd = m.connectionUsername.Update(msg)
	case profileConnectionFocusPort:
		m.connectionPort, cmd = m.connectionPort.Update(msg)
	default:
		return m, nil
	}
	m.status = ""
	m.connectionValidate = false
	return m, cmd
}

func (m Model) moveProfileConnectionFocus(delta int) (tea.Model, tea.Cmd) {
	m.status = ""
	count := int(profileConnectionFocusContinue) + 1
	m.connectionFocus = profileConnectionFocus((int(m.connectionFocus) + delta + count) % count)
	m.focusProfileConnectionInput()
	return m, nil
}

func (m Model) connectionFieldValid(focus profileConnectionFocus) bool {
	switch focus {
	case profileConnectionFocusHost:
		return profile.ValidateHost(m.connectionHost.Value()) == nil
	case profileConnectionFocusUsername:
		return profile.ValidateUsername(m.connectionUsername.Value()) == nil
	case profileConnectionFocusPort:
		port, err := strconv.Atoi(m.connectionPort.Value())
		return err == nil && profile.ValidatePort(port) == nil
	default:
		return false
	}
}

func (m Model) profileConnectionValid() bool {
	return m.connectionFieldValid(profileConnectionFocusHost) &&
		m.connectionFieldValid(profileConnectionFocusUsername) &&
		m.connectionFieldValid(profileConnectionFocusPort)
}

func (m Model) connectionProgressGuard() (wizardProgressGuard, profileConnectionFocus) {
	for _, focus := range []profileConnectionFocus{profileConnectionFocusHost, profileConnectionFocusUsername, profileConnectionFocusPort} {
		if feedback, ok := m.connectionFieldFeedback(focus); ok {
			return wizardProgressGuard{state: wizardProgressBlocked, feedback: feedback}, focus
		}
	}
	return wizardProgressGuard{}, profileConnectionFocusContinue
}

// profileConnectionState returns one relevant deterministic status, favoring
// the focused field and otherwise the first invalid field in form order.
type connectionValidation uint8

const (
	connectionValid connectionValidation = iota
	connectionHostRequired
	connectionHostInvalid
	connectionUsernameRequired
	connectionUsernameInvalid
	connectionPortNumber
	connectionPortInvalid
)

func (m Model) profileConnectionValidation() connectionValidation {
	order := []profileConnectionFocus{profileConnectionFocusHost, profileConnectionFocusUsername, profileConnectionFocusPort}
	for _, focus := range order {
		if focus == m.connectionFocus {
			if state := m.connectionFieldState(focus); state != connectionValid && (m.connectionValidate || m.connectionFieldValue(focus) != "") {
				return state
			}
			break
		}
	}
	for _, focus := range order {
		if state := m.connectionFieldState(focus); state != connectionValid && (m.connectionValidate || m.connectionFieldValue(focus) != "") {
			return state
		}
	}
	return connectionValid
}

// profileConnectionState is retained as a display seam for existing callers;
// control flow uses profileConnectionValidation instead.
func (m Model) profileConnectionState() string {
	feedback, ok := m.connectionFeedback(m.profileConnectionValidation())
	if !ok {
		return ""
	}
	return "[ERR] " + feedback.message
}

func (m Model) connectionFieldValue(focus profileConnectionFocus) string {
	switch focus {
	case profileConnectionFocusHost:
		return m.connectionHost.Value()
	case profileConnectionFocusUsername:
		return m.connectionUsername.Value()
	case profileConnectionFocusPort:
		return m.connectionPort.Value()
	default:
		return ""
	}
}

func (m Model) connectionFieldState(focus profileConnectionFocus) connectionValidation {
	var value string
	switch focus {
	case profileConnectionFocusHost:
		value = m.connectionHost.Value()
		if value == "" {
			return connectionHostRequired
		}
		if profile.ValidateHost(value) != nil {
			return connectionHostInvalid
		}
	case profileConnectionFocusUsername:
		value = m.connectionUsername.Value()
		if value == "" {
			return connectionUsernameRequired
		}
		if profile.ValidateUsername(value) != nil {
			return connectionUsernameInvalid
		}
	case profileConnectionFocusPort:
		value = m.connectionPort.Value()
		port, err := strconv.Atoi(value)
		if value == "" || err != nil {
			return connectionPortNumber
		}
		if profile.ValidatePort(port) != nil {
			return connectionPortInvalid
		}
	}
	return connectionValid
}

func (m Model) connectionFieldFeedback(focus profileConnectionFocus) (wizardFeedback, bool) {
	return m.connectionFeedback(m.connectionFieldState(focus))
}

func (m Model) connectionFeedback(state connectionValidation) (wizardFeedback, bool) {
	ids := map[connectionValidation]string{connectionHostRequired: "wizard.validation.host_required", connectionHostInvalid: "wizard.validation.host_invalid", connectionUsernameRequired: "wizard.validation.username_required", connectionUsernameInvalid: "wizard.validation.username_invalid", connectionPortNumber: "wizard.validation.port_number", connectionPortInvalid: "wizard.validation.port_invalid"}
	id, ok := ids[state]
	if !ok {
		return wizardFeedback{}, false
	}
	return wizardFeedback{kind: wizardFeedbackError, message: m.text(id, nil)}, true
}

func (m Model) renderProfileConnectionStep() string {
	frameWidth, frameHeight := m.shellFrameDimensions()
	inner, height := m.shellInnerWidth(frameWidth), m.shellInnerHeight(frameHeight)
	t := newHomeTheme(m.noColor)
	footer := renderFooterText(inner, t, m.text("wizard.footer.connection", nil), m.buildInfo)
	return m.renderWizardShell(m.renderProfileConnectionHeader(inner, t), footer, m.renderProfileConnectionPanel(inner, height, t))
}

func (m Model) renderProfileConnectionHeader(width int, t homeTheme) string {
	return m.renderWizardHeader(width, t, m.profileDraftName)
}

func (m Model) renderProfileConnectionPanel(width, height int, t homeTheme) string {
	panel := newWizardPanelLayout(width, height, t)
	contentWidth := panel.contentWidth
	rhythm := newWizardRhythm(height)
	lines := renderWizardTitleRow(contentWidth, t, m.text("wizard.connection.title", nil), m.text("wizard.step.connection", nil))
	lines = appendWizardGap(lines, rhythm.titleDivider)
	lines = append(lines, renderWizardDivider(contentWidth, t))
	lines = append(lines, t.wizardContentHeading.Render(wrapWizardText(m.text("wizard.connection.section", nil), contentWidth, "")[0]))
	lines = appendWizardGap(lines, rhythm.sectionDescription)
	for _, text := range strings.Split(m.text("wizard.connection.description", nil), "\n") {
		for _, line := range wrapWizardText(text, contentWidth, "") {
			lines = append(lines, t.metadata.Render(line))
		}
	}
	lines = appendWizardGap(lines, rhythm.descriptionControl)
	lines = append(lines, m.renderConnectionInputRow(m.text("wizard.connection.host", nil), m.connectionHost, profileConnectionFocusHost, contentWidth, t))
	lines = appendWizardGap(lines, rhythm.controls)
	lines = append(lines, m.renderConnectionInputRow(m.text("wizard.connection.username", nil), m.connectionUsername, profileConnectionFocusUsername, contentWidth, t))
	lines = appendWizardGap(lines, rhythm.controls)
	lines = append(lines, m.renderConnectionPortRow(contentWidth, t))
	if feedback, ok := m.wizardFeedbackForFeedback(m.connectionFeedback(m.profileConnectionValidation())); ok {
		lines = appendWizardGap(lines, rhythm.feedback)
		lines = append(lines, renderWizardFeedback(contentWidth, t, feedback))
	}
	lines = appendWizardGap(lines, rhythm.actions)
	lines = append(lines, m.renderProfileConnectionActions(contentWidth, t))
	return panel.render(width, lines)
}

// profileConnectionFocusRange mirrors panel composition using line counts,
// keeping input and action focus independent of display-string matching.
func (m Model) profileConnectionFocusRange(width, height int, t homeTheme) wizardLineRange {
	panel := newWizardPanelLayout(width, height, t)
	cw, rhythm := panel.contentWidth, newWizardRhythm(height)
	lines := len(renderWizardTitleRow(cw, t, m.text("wizard.connection.title", nil), m.text("wizard.step.connection", nil))) + rhythm.titleDivider + 1 + 1 + rhythm.sectionDescription
	for _, text := range strings.Split(m.text("wizard.connection.description", nil), "\n") {
		lines += len(wrapWizardText(text, cw, ""))
	}
	lines += rhythm.descriptionControl
	rows := []string{m.renderConnectionInputRow(m.text("wizard.connection.host", nil), m.connectionHost, profileConnectionFocusHost, cw, t), m.renderConnectionInputRow(m.text("wizard.connection.username", nil), m.connectionUsername, profileConnectionFocusUsername, cw, t), m.renderConnectionPortRow(cw, t)}
	index := int(m.connectionFocus)
	if index <= int(profileConnectionFocusPort) {
		for i := 0; i < index; i++ {
			lines += len(strings.Split(rows[i], "\n")) + rhythm.controls
		}
		start := lines + 1
		start += panel.contentTopOffset - 1
		return wizardLineRange{start: start, end: start + len(strings.Split(rows[index], "\n")) - 1}
	}
	for _, row := range rows {
		lines += len(strings.Split(row, "\n")) + rhythm.controls
	}
	if feedback, ok := m.wizardFeedbackForFeedback(m.connectionFeedback(m.profileConnectionValidation())); ok {
		lines += rhythm.feedback + len(strings.Split(renderWizardFeedback(cw, t, feedback), "\n"))
	}
	start := lines + rhythm.actions + 1
	start += panel.contentTopOffset - 1
	if m.connectionFocus == profileConnectionFocusContinue && len(strings.Split(m.renderProfileConnectionActions(cw, t), "\n")) > 1 {
		start++
	}
	return wizardLineRange{start: start, end: start}
}

func renderProfileConnectionTitleRow(width int, t homeTheme) []string {
	return renderWizardTitleRow(width, t, "Crear perfil IBM i", "Paso 2 de 8 — Conexión")
}

func (m Model) renderConnectionInputRow(label string, input textinput.Model, focus profileConnectionFocus, width int, t homeTheme) string {
	return renderWizardInputRow(label, input, m.connectionFocus == focus, width, t, wizardInputOptions{labelWidth: len("Puerto SSH")})
}

func (m Model) renderConnectionPortRow(width int, t homeTheme) string {
	label := m.text("wizard.connection.port", nil)
	defaultPort := m.text("wizard.connection.default_port", nil)
	if width < 34 {
		lines := []string{m.renderConnectionInputRow(label, m.connectionPort, profileConnectionFocusPort, width, t)}
		for _, line := range wrapWizardText(defaultPort, width, "") {
			lines = append(lines, t.metadata.Render(line))
		}
		return strings.Join(lines, "\n")
	}
	return renderWizardInputRow(label, m.connectionPort, m.connectionFocus == profileConnectionFocusPort, width, t, wizardInputOptions{compactEditableWidth: 7, labelWidth: len(label), suffix: t.metadata.Render("  " + defaultPort)})
}

func profileConnectionTextBlockMargin(height int) int {
	if height >= 30 {
		return 1
	}
	return 0
}

func profileConnectionFieldGap(height int) int {
	if height >= 30 {
		return 1
	}
	return 0
}

func (m Model) renderProfileConnectionActions(width int, t homeTheme) string {
	return renderWizardActions(width, t, m.text("action.back", nil), m.text("action.continue", nil), m.connectionFocus == profileConnectionFocusBack, m.connectionFocus == profileConnectionFocusContinue, m.noColor)
}
