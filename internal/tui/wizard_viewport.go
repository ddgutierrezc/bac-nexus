package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderWizardShell keeps chrome fixed while the complete panel content lives
// in one persistent Bubbles viewport. The reserved indicator row means a
// narrow terminal never silently clips a functional wizard line.
func (m Model) renderWizardShell(header, footer, panel string) string {
	frameWidth, frameHeight := m.shellFrameDimensions()
	width, height := m.shellInnerWidth(frameWidth), m.shellInnerHeight(frameHeight)
	t := newHomeTheme(m.noColor)
	separator := t.headerSeparator.Width(width).Render(strings.Repeat("─", width))
	footerBlock := t.fieldsetBorder.Render(strings.Repeat("─", width+2)) + "\n" + footer
	rhythm := newWizardRhythm(height)
	bodyHeight := max(height-lipgloss.Height(header)-lipgloss.Height(separator)-rhythm.top-lipgloss.Height(footerBlock)-1, 1)
	// One fixed row is owned by the indicator, so content and indicator together
	// always consume the actual shell body height.
	viewportHeight := max(bodyHeight-1, 1)
	vp := m.wizardViewport
	vp.Width, vp.Height = width, viewportHeight
	vp.SetContent(panel)
	body := strings.TrimRight(vp.View(), "\n") + "\n" + wizardOverflowIndicator(vp, width, t)

	layout := t.shellLayout(width)
	layout.Add(header)
	layout.Add(separator)
	layout.AddGap(rhythm.top)
	layout.Add(body)
	// shellLayout joins slots with a separator newline. Reserve that row before
	// the footer so Lip Gloss does not pad a blank row *after* fixed chrome.
	layout.AddGap(1)
	layout.AddFooter(footerBlock)
	return t.frame.Width(frameWidth).Height(frameHeight).Render(layout.Render(height))
}

func wizardOverflowIndicator(vp interface {
	AtTop() bool
	AtBottom() bool
}, width int, t homeTheme) string {
	indicator := ""
	switch {
	case !vp.AtTop() && !vp.AtBottom():
		indicator = "▲ más  ▼ más"
	case !vp.AtTop():
		indicator = "▲ más"
	case !vp.AtBottom():
		indicator = "▼ más"
	}
	if indicator == "" {
		return ""
	}
	return t.metadata.Width(width).Align(lipgloss.Right).Render(indicator)
}

func (m *Model) refreshWizardViewport() {
	if m.screen != screenProfileStep && m.screen != screenProfileConnection && m.screen != screenProfileIdentity {
		return
	}
	frameWidth, frameHeight := m.shellFrameDimensions()
	width, height := m.shellInnerWidth(frameWidth), m.shellInnerHeight(frameHeight)
	t := newHomeTheme(m.noColor)
	footer := profileStepFooter
	panel := ""
	anchor := ""
	var focusRange wizardLineRange
	hasFocusRange := false
	switch m.screen {
	case screenProfileStep:
		panel = m.renderProfileStepPanel(width, height, t)
		focusRange, hasFocusRange = m.profileStepFocusRange(width, height, t), true
		if m.profileFocus == profileFocusName {
			anchor = "▸ " + profileInputLabel
		} else if m.profileFocus == profileFocusCancel {
			anchor = "< CANCELAR >"
		} else {
			anchor = "[ CONTINUAR ]"
		}
	case screenProfileConnection:
		footer = profileConnectionStepFooter
		panel = m.renderProfileConnectionPanel(width, height, t)
		focusRange, hasFocusRange = m.profileConnectionFocusRange(width, height, t), true
		switch m.connectionFocus {
		case profileConnectionFocusHost:
			anchor = "Host"
		case profileConnectionFocusUsername:
			anchor = "Usuario"
		case profileConnectionFocusPort:
			anchor = "Puerto SSH"
		case profileConnectionFocusBack:
			anchor = "< VOLVER >"
		default:
			anchor = "[ CONTINUAR ]"
		}
	case screenProfileIdentity:
		footer = profileIdentityFooter
		identityPanel := m.renderProfileIdentityPanelContent(width, height, t)
		panel = identityPanel.text
		focusRange, hasFocusRange = identityPanel.ranges[m.identityFocus]
		switch m.identityFocus {
		case profileIdentityFocusKnown:
			anchor = "Verificar"
		case profileIdentityFocusObserved:
			anchor = "Confiar"
		case profileIdentityFocusBack:
			anchor = "< VOLVER >"
		default:
			anchor = "[ CONTINUAR ]"
		}
	}
	if m.status != "" {
		if feedback, ok := wizardFeedbackFromRow(m.status); ok {
			switch feedback.kind {
			case wizardFeedbackOK:
				anchor = "[OK]"
			case wizardFeedbackInfo:
				anchor = "[INFO]"
			case wizardFeedbackWarning:
				anchor = "[WARN]"
			case wizardFeedbackError:
				anchor = "[ERR]"
			default:
				anchor = "[--]"
			}
		} else {
			anchor = "Ayuda:"
		}
	}
	if m.err != nil {
		anchor = "[ERR]"
		hasFocusRange = false
	}
	footerText := renderFooterText(width, t, footer, m.buildInfo)
	bodyHeight := max(height-lipgloss.Height(m.renderProfileConnectionHeader(width, t))-1-newWizardRhythm(height).top-lipgloss.Height(t.fieldsetBorder.Render(strings.Repeat("─", width+2))+"\n"+footerText)-1, 1)
	m.wizardViewport.Width, m.wizardViewport.Height = width, max(bodyHeight-1, 1)
	m.wizardViewport.SetContent(panel)
	if hasFocusRange && m.status == "" {
		m.wizardFocusStart, m.wizardFocusEnd = focusRange.start, focusRange.end
	} else {
		m.setWizardFocusRange(panel, anchor)
	}
	m.revealWizardFocusRange()
}

func (m *Model) setWizardFocusRange(panel, anchor string) {
	m.wizardFocusStart, m.wizardFocusEnd = 0, 0
	if anchor == "" {
		return
	}
	lines := strings.Split(panel, "\n")
	for i, line := range lines {
		if strings.Contains(line, anchor) {
			m.wizardFocusStart, m.wizardFocusEnd = i, i
			// A focused choice is a complete visual block. Its extent continues
			// through its wrapped description/note until the next blank row.
			if m.screen == screenProfileIdentity && (m.identityFocus == profileIdentityFocusKnown || m.identityFocus == profileIdentityFocusObserved) {
				for j := i + 1; j < len(lines) && strings.TrimSpace(lines[j]) != ""; j++ {
					m.wizardFocusEnd = j
				}
			}
			return
		}
	}
}

func (m *Model) revealWizardFocusRange() {
	if m.wizardFocusEnd < m.wizardFocusStart {
		return
	}
	start, end, height := m.wizardFocusStart, m.wizardFocusEnd, m.wizardViewport.Height
	if end-start+1 > height {
		// A block taller than the viewport cannot be wholly revealed. Its first
		// line identifies the focused control, while ordinary viewport movement
		// keeps every continuation reachable.
		m.wizardViewport.SetYOffset(start)
		return
	}
	if start < m.wizardViewport.YOffset {
		m.wizardViewport.SetYOffset(start)
		return
	}
	if end >= m.wizardViewport.YOffset+height {
		if height == 1 {
			m.wizardViewport.SetYOffset(end)
			return
		}
		// Bubbles reserves its final row while rendering a viewport. Keep one
		// conservative row of headroom so a focused terminal line is visible.
		m.wizardViewport.SetYOffset(max(end-height+2, 0))
	}
}

func (m Model) wizardPanelContent() string {
	frameWidth, frameHeight := m.shellFrameDimensions()
	width, height := m.shellInnerWidth(frameWidth), m.shellInnerHeight(frameHeight)
	t := newHomeTheme(m.noColor)
	switch m.screen {
	case screenProfileStep:
		return m.renderProfileStepPanel(width, height, t)
	case screenProfileConnection:
		return m.renderProfileConnectionPanel(width, height, t)
	case screenProfileIdentity:
		return m.renderProfileIdentityPanel(width, height, t)
	}
	return ""
}
