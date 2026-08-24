package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type profileIdentityFocus uint8

const (
	profileIdentityFocusKnown profileIdentityFocus = iota
	profileIdentityFocusObserved
	profileIdentityFocusBack
	profileIdentityFocusContinue
)
const profileIdentityFooter = "Tab siguiente  •  Espacio seleccionar  •  Enter continuar  •  Esc volver  •  ? ayuda"

func (m Model) updateProfileIdentityStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.screen = screenProfileConnection
		return m, nil
	case "?":
		m.status = "Ayuda: Tab cambia el foco; Espacio selecciona una opción."
		return m, nil
	case "tab":
		m.identityFocus = profileIdentityFocus((int(m.identityFocus) + 1) % 4)
		return m, nil
	case "shift+tab":
		m.identityFocus = profileIdentityFocus((int(m.identityFocus) + 3) % 4)
		return m, nil
	case " ":
		if m.identityFocus == profileIdentityFocusKnown {
			m.identityDecision = profileIdentityKnownFingerprint
			m.status = ""
		}
		if m.identityFocus == profileIdentityFocusObserved {
			m.identityDecision = profileIdentityObservedKey
			m.status = ""
		}
		return m, nil
	case "enter":
		if m.identityFocus == profileIdentityFocusBack {
			m.screen = screenProfileConnection
			return m, nil
		}
		if m.identityFocus == profileIdentityFocusContinue {
			guard := wizardProgressGuard{}
			if m.identityDecision == profileIdentityNone {
				guard = wizardProgressGuard{state: wizardProgressBlocked, feedback: wizardFeedback{kind: wizardFeedbackWarning, message: "Selecciona una opción antes de continuar"}}
			}
			if !m.activateWizardProgress(guard, nil) {
				return m, nil
			}
			d := m.identityDecision
			return m, func() tea.Msg { return profileIdentityAcceptedMsg{d} }
		}
	}
	return m, nil
}

func (m Model) renderProfileIdentityStep() string {
	fw, fh := m.shellFrameDimensions()
	w, h := m.shellInnerWidth(fw), m.shellInnerHeight(fh)
	t := newHomeTheme(m.noColor)
	_ = fw
	return m.renderWizardShell(m.renderProfileConnectionHeader(w, t), renderFooterText(w, t, profileIdentityFooter, m.buildInfo), m.renderProfileIdentityPanel(w, h, t))
}
func (m Model) renderProfileIdentityPanel(w, h int, t homeTheme) string {
	return m.renderProfileIdentityPanelContent(w, h, t).text
}

type wizardPanel struct {
	text   string
	ranges map[profileIdentityFocus]wizardLineRange
}

type wizardLineRange struct{ start, end int }

func (m Model) renderProfileIdentityPanelContent(w, h int, t homeTheme) wizardPanel {
	panel := newWizardPanelLayout(w, h, t)
	cw := panel.contentWidth
	rhythm := newWizardRhythm(h)
	lines := renderWizardTitleRow(cw, t, "Crear perfil IBM i", "Paso 3 de 9 — Identidad")
	ranges := make(map[profileIdentityFocus]wizardLineRange)
	appendBlock := func(focus profileIdentityFocus, block wizardRenderedBlock) {
		start := len(lines) + block.start
		lines = append(lines, strings.Split(block.text, "\n")...)
		ranges[focus] = wizardLineRange{start: start, end: start + block.end - block.start}
	}
	lines = appendWizardGap(lines, rhythm.titleDivider)
	lines = append(lines, renderWizardDivider(cw, t))
	lines = append(lines, t.wizardContentHeading.Render("Identidad del servidor"))
	lines = appendWizardGap(lines, rhythm.sectionDescription)
	for _, x := range wrapWizardText("¿Cómo quieres comprobar que este IBM i es el servidor correcto?", cw, "") {
		lines = append(lines, t.fieldsetContent.Render(x))
	}
	lines = appendWizardGap(lines, rhythm.questionSupporting)
	for _, x := range wrapWizardText("Elige cómo Nexus debe establecer la confianza SSH de este perfil.", cw, "") {
		lines = append(lines, t.metadata.Render(x))
	}
	for _, x := range wrapWizardText("Esta decisión solo se registra localmente en el asistente; no conecta con el servidor ni guarda credenciales o perfiles todavía.", cw, "") {
		lines = append(lines, t.metadata.Render(x))
	}
	lines = appendWizardGap(lines, rhythm.descriptionControl)
	appendBlock(profileIdentityFocusKnown, renderWizardChoiceBlock(cw, t, WizardChoice{ID: "known-fingerprint", Label: "Verificar un fingerprint conocido", Description: "Ya tengo un fingerprint obtenido mediante una fuente oficial o independiente."}, m.identityFocus == profileIdentityFocusKnown, m.identityDecision == profileIdentityKnownFingerprint))
	lines = appendWizardGap(lines, rhythm.controls)
	appendBlock(profileIdentityFocusObserved, renderWizardChoiceBlock(cw, t, WizardChoice{ID: "observed-key", Label: "Confiar en la clave observada ahora", Description: "Nexus inspeccionará la clave presentada por el servidor y mostrará su fingerprint antes de guardarlo.", Note: "Esta primera observación no verifica por sí sola que el servidor sea legítimo."}, m.identityFocus == profileIdentityFocusObserved, m.identityDecision == profileIdentityObservedKey))
	feedbackStart := -1
	if feedback, ok := m.wizardFeedbackFor(""); ok {
		lines = appendWizardGap(lines, rhythm.feedback)
		feedbackStart = len(lines)
		lines = append(lines, renderWizardFeedback(cw, t, feedback))
	}
	lines = appendWizardGap(lines, rhythm.actions)
	rightState := wizardProgressReady
	if m.identityDecision == profileIdentityNone {
		rightState = wizardProgressBlocked
	}
	actions := renderWizardActionsBlock(cw, t, "< VOLVER >", "[ CONTINUAR ]", m.identityFocus == profileIdentityFocusBack, m.identityFocus == profileIdentityFocusContinue, m.noColor, wizardActionOptions{rightState: rightState})
	actionStart := len(lines) + actions.start
	lines = append(lines, strings.Split(actions.text, "\n")...)
	// A split action block exposes deterministic ranges for both controls. They
	// share one line at wide widths and receive their own lines when narrow.
	ranges[profileIdentityFocusBack] = wizardLineRange{start: actionStart, end: actionStart}
	ranges[profileIdentityFocusContinue] = wizardLineRange{start: actionStart + actions.end, end: actionStart + actions.end}
	if m.identityFocus == profileIdentityFocusContinue && feedbackStart >= 0 {
		ranges[profileIdentityFocusContinue] = wizardLineRange{start: feedbackStart, end: actionStart + actions.end}
	}
	for focus, r := range ranges {
		ranges[focus] = wizardLineRange{start: r.start + panel.contentTopOffset, end: r.end + panel.contentTopOffset}
	}
	return wizardPanel{text: panel.render(w, lines), ranges: ranges}
}
