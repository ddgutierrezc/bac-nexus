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
	connectionStepIndicator     = "Paso 2 de 9 — Conexión"
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
		m.status, m.err = "Ayuda: completa Host, Usuario y Puerto SSH; Tab cambia el foco.", nil
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
			m.connectionValidate = true
			return m, nil
		case profileConnectionFocusBack:
			m.screen, m.status, m.err = screenProfileStep, "", nil
			m.profileFocus = profileFocusName
			m.profileName.Focus()
			return m, nil
		case profileConnectionFocusContinue:
			if !m.profileConnectionValid() {
				m.connectionValidate = true
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

// profileConnectionState returns one relevant deterministic status, favoring
// the focused field and otherwise the first invalid field in form order.
func (m Model) profileConnectionState() string {
	order := []profileConnectionFocus{profileConnectionFocusHost, profileConnectionFocusUsername, profileConnectionFocusPort}
	for _, focus := range order {
		if focus == m.connectionFocus {
			if state := m.connectionFieldState(focus); state != "" && (m.connectionValidate || m.connectionFieldValue(focus) != "") {
				return state
			}
			break
		}
	}
	for _, focus := range order {
		if state := m.connectionFieldState(focus); state != "" && (m.connectionValidate || m.connectionFieldValue(focus) != "") {
			return state
		}
	}
	return ""
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

func (m Model) connectionFieldState(focus profileConnectionFocus) string {
	var value string
	switch focus {
	case profileConnectionFocusHost:
		value = m.connectionHost.Value()
		if value == "" {
			return "[ERR] Host requerido"
		}
		if profile.ValidateHost(value) != nil {
			return "[ERR] Host inválido"
		}
	case profileConnectionFocusUsername:
		value = m.connectionUsername.Value()
		if value == "" {
			return "[ERR] Usuario requerido"
		}
		if profile.ValidateUsername(value) != nil {
			return "[ERR] Usuario inválido"
		}
	case profileConnectionFocusPort:
		value = m.connectionPort.Value()
		port, err := strconv.Atoi(value)
		if value == "" || err != nil {
			return "[ERR] Puerto SSH debe ser un número"
		}
		if profile.ValidatePort(port) != nil {
			return "[ERR] Puerto SSH debe estar entre 1 y 65535"
		}
	}
	return ""
}

func (m Model) renderProfileConnectionStep() string {
	frameWidth, frameHeight := m.shellFrameDimensions()
	inner, height := m.shellInnerWidth(frameWidth), m.shellInnerHeight(frameHeight)
	t := newHomeTheme(m.noColor)
	footer := renderFooterText(inner, t, profileConnectionStepFooter, m.buildInfo)
	return m.renderWizardShell(m.renderProfileConnectionHeader(inner, t), footer, m.renderProfileConnectionPanel(inner, height, t))
}

func (m Model) renderProfileConnectionHeader(width int, t homeTheme) string {
	return renderWizardHeader(width, t, m.profileDraftName)
}

func (m Model) renderProfileConnectionPanel(width, height int, t homeTheme) string {
	panelWidth := min(max(width-12, 34), profileStepPanelMaxWidth)
	if panelWidth > width {
		panelWidth = width
	}
	panelStyle, contentInset := profileStepPanelStyle(t, height)
	contentWidth := max(panelWidth-contentInset, 1)
	rhythm := newWizardRhythm(height)
	lines := renderWizardTitleRow(contentWidth, t, connectionPanelTitle, connectionStepIndicator)
	lines = appendWizardGap(lines, rhythm.titleDivider)
	lines = append(lines, renderWizardDivider(contentWidth, t))
	lines = appendWizardGap(lines, rhythm.sectionDescription)
	lines = append(lines, t.wizardContentHeading.Render(wrapWizardText("Conexión con IBM i", contentWidth, "")[0]))
	for _, text := range []string{"Indica cómo localizar el IBM i y qué usuario utilizará Nexus.", "Nexus todavía no se conectará al servidor en este paso."} {
		for _, line := range wrapWizardText(text, contentWidth, "") {
			lines = append(lines, t.metadata.Render(line))
		}
	}
	lines = appendWizardGap(lines, rhythm.descriptionControl)
	lines = append(lines, m.renderConnectionInputRow("Host", m.connectionHost, profileConnectionFocusHost, contentWidth, t))
	lines = appendWizardGap(lines, rhythm.controls)
	lines = append(lines, m.renderConnectionInputRow("Usuario", m.connectionUsername, profileConnectionFocusUsername, contentWidth, t))
	lines = appendWizardGap(lines, rhythm.controls)
	lines = append(lines, m.renderConnectionPortRow(contentWidth, t))
	if state := m.profileConnectionState(); state != "" {
		lines = append(lines, m.renderProfileStatus(state, contentWidth, t))
	}
	if m.status != "" || m.err != nil {
		lines = appendWizardGap(lines, rhythm.feedback)
		lines = append(lines, m.renderFeedback(contentWidth, t))
	}
	lines = appendWizardGap(lines, rhythm.actions)
	lines = append(lines, m.renderProfileConnectionActions(contentWidth, t))
	return centerHomeBlock(width, panelStyle.Width(panelWidth-2).Render(strings.Join(lines, "\n")))
}

// profileConnectionFocusRange mirrors panel composition using line counts,
// keeping input and action focus independent of display-string matching.
func (m Model) profileConnectionFocusRange(width, height int, t homeTheme) wizardLineRange {
	pw := min(max(width-12, 34), profileStepPanelMaxWidth)
	if pw > width {
		pw = width
	}
	_, inset := profileStepPanelStyle(t, height)
	cw, rhythm := max(pw-inset, 1), newWizardRhythm(height)
	lines := len(renderWizardTitleRow(cw, t, connectionPanelTitle, connectionStepIndicator)) + rhythm.titleDivider + 1 + rhythm.sectionDescription + 1
	for _, text := range []string{"Indica cómo localizar el IBM i y qué usuario utilizará Nexus.", "Nexus todavía no se conectará al servidor en este paso."} {
		lines += len(wrapWizardText(text, cw, ""))
	}
	lines += rhythm.descriptionControl
	rows := []string{m.renderConnectionInputRow("Host", m.connectionHost, profileConnectionFocusHost, cw, t), m.renderConnectionInputRow("Usuario", m.connectionUsername, profileConnectionFocusUsername, cw, t), m.renderConnectionPortRow(cw, t)}
	index := int(m.connectionFocus)
	if index <= int(profileConnectionFocusPort) {
		for i := 0; i < index; i++ {
			lines += len(strings.Split(rows[i], "\n")) + rhythm.controls
		}
		start := lines + 1
		if height >= 30 {
			start++
		}
		return wizardLineRange{start: start, end: start + len(strings.Split(rows[index], "\n")) - 1}
	}
	for _, row := range rows {
		lines += len(strings.Split(row, "\n")) + rhythm.controls
	}
	if state := m.profileConnectionState(); state != "" {
		lines += len(strings.Split(m.renderProfileStatus(state, cw, t), "\n"))
	}
	if m.status != "" || m.err != nil {
		lines += rhythm.feedback + len(strings.Split(m.renderFeedback(cw, t), "\n"))
	}
	start := lines + rhythm.actions + 1
	if height >= 30 {
		start++
	}
	return wizardLineRange{start: start, end: start}
}

func renderProfileConnectionTitleRow(width int, t homeTheme) []string {
	return renderWizardTitleRow(width, t, connectionPanelTitle, connectionStepIndicator)
}

func (m Model) renderConnectionInputRow(label string, input textinput.Model, focus profileConnectionFocus, width int, t homeTheme) string {
	return renderWizardInputRow(label, input, m.connectionFocus == focus, width, t, wizardInputOptions{labelWidth: len("Puerto SSH")})
}

func (m Model) renderConnectionPortRow(width int, t homeTheme) string {
	const label = "Puerto SSH"
	const defaultPort = "Predeterminado: 22"
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
	return renderWizardActions(width, t, "< VOLVER >", "[ CONTINUAR ]", m.connectionFocus == profileConnectionFocusBack, m.connectionFocus == profileConnectionFocusContinue, m.noColor)
}
