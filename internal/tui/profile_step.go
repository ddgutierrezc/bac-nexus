package tui

import (
	"strings"

	"bac-nexus/internal/profile"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type profileStepFocus uint8

const (
	profileFocusName profileStepFocus = iota
	profileFocusCancel
	profileFocusContinue
)

const profileStepFooter = "Tab siguiente  •  Enter continuar  •  Esc volver  •  ? ayuda"

const (
	profileStepPanelMaxWidth = 72
	profileInputLabel        = "Nombre"
)

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
func (m Model) profileNameState() string {
	name := m.profileName.Value()
	if name == "" || !m.profilesLoaded || m.profilesLoadFailed {
		return ""
	}
	if profile.ValidateName(name) != nil {
		return "[ERR] Nombre de perfil inválido"
	}
	if m.profileNameDuplicate(name) {
		return "[ERR] Ya existe un perfil con ese nombre"
	}
	return "[OK] Nombre disponible"
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
	return m.profileNameState() == "[OK] Nombre disponible"
}

func (m Model) updateProfileStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc", "ctrl+c":
		m.screen, m.status, m.err = screenHome, "", nil
		return m, nil
	case "?":
		m.status, m.err = "Ayuda: escribe un nombre; Tab cambia el foco.", nil
		return m, nil
	case "tab":
		return m.moveProfileFocus(1)
	case "shift+tab":
		return m.moveProfileFocus(-1)
	case "enter":
		switch m.profileFocus {
		case profileFocusName:
			if m.profileNameValid() {
				return m.moveProfileFocus(1 + int(profileFocusContinue-profileFocusCancel))
			}
			return m, nil
		case profileFocusCancel:
			m.screen, m.status, m.err = screenHome, "", nil
			return m, nil
		case profileFocusContinue:
			if !m.profileNameValid() {
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
	header := m.renderProfileStepHeader(inner, t)
	separator := t.headerSeparator.Width(inner).Render(strings.Repeat("─", inner))
	footer := renderFooterText(inner, t, profileStepFooter, m.buildInfo)
	footerSeparator := t.fieldsetBorder.Render(strings.Repeat("─", inner+2))

	layout := t.shellLayout(inner)
	layout.Add(header)
	layout.Add(separator)
	layout.AddGap(profileStepTopGap(height))
	layout.Add(m.renderProfileStepPanel(inner, height, t))
	layout.AddStretch()
	layout.AddFooter(footerSeparator + "\n" + footer)
	return t.frame.Width(frameWidth).Height(frameHeight).Render(layout.Render(height))
}

func (m Model) renderProfileStepHeader(width int, t homeTheme) string {
	brand := t.headerBrand.Render("BAC NEXUS")
	profile := t.headerProfile.Render("PERFIL: NUEVO")
	status := t.headerStatus.Render("ESTADO: CONFIGURANDO")
	text := strings.Repeat(" ", headerLeftPadding) + strings.Join([]string{brand, "│", profile, "│", status}, "  ")
	return t.header.Width(width).Align(lipgloss.Left).Render(fitHomeLine(text, width))
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
	if width < 16 || height < 15 {
		return m.renderMinimalProfileStep(width, t)
	}
	panelWidth := min(max(width-12, 34), profileStepPanelMaxWidth)
	if panelWidth > width {
		panelWidth = width
	}
	panelStyle, contentInset := profileStepPanelStyle(t, height)
	contentWidth := max(panelWidth-contentInset, 1)
	state := m.profileNameState()
	inputGap, actionGap := profileStepInputGap(height), profileStepActionGap(height)
	if height < 24 && (state != "" || m.status != "" || m.err != nil) {
		inputGap, actionGap = 0, 0
	}
	lines := renderProfileStepTitleRow(contentWidth, t)
	lines = appendProfileStepGap(lines, profileStepSmallGap(height))
	lines = append(lines, t.fieldsetBorder.Render(strings.Repeat("─", contentWidth)))
	lines = appendProfileStepGap(lines, profileStepSmallGap(height))
	lines = append(lines, m.renderProfileNameLabel(contentWidth, t))
	lines = appendProfileStepGap(lines, profileStepSmallGap(height))
	lines = append(lines, profileStepGuidance(contentWidth, height, t)...)
	lines = appendProfileStepGap(lines, profileStepSmallGap(height))
	lines = append(lines, t.metadata.Render(fitHomeLine("Ej: CRI400F, CRI400FDev, CRI400FProd", contentWidth)))
	lines = appendProfileStepGap(lines, inputGap)
	lines = append(lines, m.renderProfileInput(contentWidth, t))
	if state != "" {
		lines = append(lines, m.renderProfileStatus(state, contentWidth, t))
	}
	if m.status != "" || m.err != nil {
		lines = append(lines, m.renderFeedback(contentWidth, t))
	}
	lines = appendProfileStepGap(lines, actionGap)
	lines = append(lines, m.renderProfileActions(contentWidth, t))
	panel := panelStyle.Width(panelWidth - 2).Render(strings.Join(lines, "\n"))
	return centerHomeBlock(width, panel)
}

func profileStepGuidance(width, height int, t homeTheme) []string {
	rules := []string{
		"Usa 1–64 caracteres ASCII; inicia con letra o número.",
		"Luego usa solo letras, números, guion (-) o guion bajo (_).",
		"Sin espacios, puntos, tildes ni otros símbolos.",
	}
	if height < 30 {
		rules = rules[:1]
	}
	lines := make([]string, len(rules))
	for i, rule := range rules {
		lines[i] = t.metadata.Render(fitHomeLine(rule, width))
	}
	return lines
}

// renderProfileStepTitleRow keeps the wizard title and step indicator aligned
// to opposite edges whenever their display widths leave a readable middle gap.
func renderProfileStepTitleRow(width int, t homeTheme) []string {
	const title = "Crear perfil IBM i"
	const step = "Paso 1 de 9 — Perfil"
	const minimumGap = 2
	if width >= lipgloss.Width(title)+minimumGap+lipgloss.Width(step) {
		return []string{t.panelTitle.Render(title) + strings.Repeat(" ", width-lipgloss.Width(title)-lipgloss.Width(step)) + t.metadata.Render(step)}
	}
	titleLine := t.panelTitle.Render(fitHomeLine(title, width))
	stepLine := fitHomeLine(step, width)
	if lipgloss.Width(step) <= width {
		stepLine = lipgloss.PlaceHorizontal(width, lipgloss.Right, stepLine)
	}
	return []string{titleLine, t.metadata.Render(stepLine)}
}

// profileStepPanelStyle derives the wizard surface from the shared panel and
// adds breathing room only where the shell has enough height to retain all
// controls and its pinned footer.
func profileStepPanelStyle(t homeTheme, height int) (lipgloss.Style, int) {
	if height >= 30 {
		return t.panel.Padding(1, 2), 6
	}
	return t.panel, 4
}

func (m Model) renderMinimalProfileStep(width int, t homeTheme) string {
	lineWidth := max(width, 1)
	lines := []string{
		t.panelTitle.Render(fitHomeLine("Crear perfil IBM i — Paso 1/9", lineWidth)),
	}
	if m.status != "" || m.err != nil {
		lines = append(lines, m.renderFeedback(lineWidth, t))
	}
	lines = append(lines, m.renderProfileInput(lineWidth, t))
	if state := m.profileNameState(); state != "" && width >= 24 {
		lines = append(lines, m.renderProfileStatus(state, lineWidth, t))
	}
	lines = append(lines, m.renderProfileActions(lineWidth, t))
	return strings.Join(lines, "\n")
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
	label := "Nombre del perfil"
	if m.profileFocus == profileFocusName {
		label = "▸ " + label
	}
	return t.fieldsetContent.Render(fitHomeLine(label, width))
}

func (m Model) renderProfileInput(width int, t homeTheme) string {
	input := m.profileName
	focusMarker := "  "
	if m.profileFocus == profileFocusName {
		focusMarker = "▸ "
	}
	prefix := focusMarker + profileInputLabel + "  [ "
	// Keep the Bubbles viewport inside terminal-native brackets and reserve the
	// closing glyph, so editing always has a visible trailing surface.
	editableWidth := max(width-lipgloss.Width(prefix)-lipgloss.Width(" ]"), 1)
	// Bubbles reserves a cursor cell beyond Width, so leave one cell for it
	// inside the bracketed surface rather than letting the closing bracket wrap.
	input.Width = max(editableWidth-1, 1)
	view := input.View()
	padding := max(editableWidth-lipgloss.Width(view), 0)
	row := prefix + view + strings.Repeat(" ", padding) + " ]"
	if m.profileFocus == profileFocusName {
		return t.selectedRow.Width(width).Render(row)
	}
	return t.fieldsetContent.Width(width).Render(row)
}

func (m Model) renderProfileStatus(state string, width int, t homeTheme) string {
	if strings.HasPrefix(state, "[OK]") {
		return t.statusOK.Render(fitHomeLine(state, width))
	}
	return t.statusError.Render(fitHomeLine(state, width))
}

func (m Model) renderProfileActions(width int, t homeTheme) string {
	cancel := "< CANCELAR >"
	continueAction := "[ CONTINUAR ]"
	if m.profileFocus == profileFocusCancel {
		cancel = "▸ " + cancel
	}
	if m.profileFocus == profileFocusContinue {
		continueAction = "▸ " + continueAction
		if !m.noColor {
			continueAction = lipgloss.NewStyle().Background(lipgloss.Color(bacRed)).Foreground(lipgloss.Color(white)).Bold(true).Render(continueAction)
		}
	}
	line := cancel + "    " + continueAction
	if lipgloss.Width(line) > width {
		if m.profileFocus == profileFocusCancel {
			return fitHomeLine(cancel, width)
		}
		return fitHomeLine(continueAction, width)
	}
	return lipgloss.PlaceHorizontal(width, lipgloss.Right, line)
}
