package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// profileScreen renders the direct profile flow inside the existing BAC chrome.
// Its viewport is owned by Model so scrolling and focus reveal survive updates.
func (m Model) profileScreen(title, body, footer string, focus string) string {
	width, height := m.profileFrameDimensions()
	inner := max(width-2, 1)
	t := newHomeTheme(m.noColor)
	header := t.header.Width(inner).Render(t.headerBrand.Render("BAC NEXUS"))
	separator := t.headerSeparator.Width(inner).Render(strings.Repeat("─", inner))
	footerText := t.footer.Width(inner).Align(lipgloss.Center).Render(footer)
	panelWidth := min(max(inner-4, 1), 72)
	content := m.profilePanelContent(title, body, panelWidth, t)
	// The outer frame, header, separator, panel borders, footer, and layout
	// joins consume ten terminal rows before the viewport can safely grow.
	viewportHeight := max(height-11, 1)
	if m.profileViewport.Width != panelWidth || m.profileViewport.Height != viewportHeight || m.profileViewportText != content {
		m.profileViewport.Width, m.profileViewport.Height = panelWidth, viewportHeight
		m.profileViewportText = content
		m.profileViewport.SetContent(content)
		m.revealProfileFocus(focus)
	}
	panel := t.panel.Width(panelWidth).Render(strings.TrimRight(m.profileViewport.View(), "\n"))
	panel = lipgloss.PlaceHorizontal(inner, lipgloss.Center, panel)
	overflow := wizardOverflowIndicator(m.profileViewport, inner, t, m.text("overflow.above", nil), m.text("overflow.below", nil))
	lines := []string{header, separator, panel}
	if overflow != "" {
		lines = append(lines, overflow)
	}
	layout := t.shellLayout(inner)
	for _, line := range lines {
		layout.Add(line)
	}
	layout.AddStretch()
	layout.AddFooter(footerText)
	return t.frame.Width(width).Height(height).Render(layout.Render(max(height-2, 1)))
}

func (m *Model) refreshProfileViewport(title, body, focus string) {
	width, height := m.profileFrameDimensions()
	inner := max(width-2, 1)
	panelWidth := min(max(inner-4, 1), 72)
	content := m.profilePanelContent(title, body, panelWidth, newHomeTheme(m.noColor))
	m.profileViewport.Width, m.profileViewport.Height = panelWidth, max(height-11, 1)
	m.profileViewportText = content
	m.profileViewport.SetContent(content)
	m.revealProfileFocus(focus)
}

func (m Model) profilePanelContent(title, body string, width int, t homeTheme) string {
	var lines []string
	for _, line := range wrapWizardText(title, width-2, "") {
		lines = append(lines, t.panelTitle.Render(line))
	}
	lines = append(lines, "")
	for _, paragraph := range strings.Split(body, "\n") {
		lines = append(lines, wrapWizardText(paragraph, width-2, "")...)
	}
	return strings.Join(lines, "\n")
}

// profileField, profileAction, and profileFeedback keep the direct profile
// flow's repeated controls and semantic status formatting cohesive.
func (m Model) profileField(marker, label, value string) string {
	return marker + " " + label + ": " + value
}

func (m Model) profileAction(marker, label string) string {
	return marker + " [ " + label + " ]"
}

func (m Model) profileFeedback(marker, message string, t homeTheme) string {
	style := t.statusNeutral
	switch marker {
	case "[OK]":
		style = t.statusOK
	case "[INFO]":
		style = t.statusInfo
	case "[WARN]":
		style = t.statusWarning
	case "[ERR]":
		style = t.statusError
	}
	return style.Render(marker) + " " + t.fieldsetContent.Render(message)
}

func (m *Model) revealProfileFocus(focus string) {
	if focus == "" {
		return
	}
	lines := strings.Split(m.profileViewportText, "\n")
	for _, needle := range profileFocusNeedles(focus) {
		for line, text := range lines {
			if !strings.Contains(text, needle) {
				continue
			}
			if line < m.profileViewport.YOffset {
				m.profileViewport.YOffset = line
			} else if line >= m.profileViewport.YOffset+m.profileViewport.Height {
				m.profileViewport.YOffset = line - m.profileViewport.Height + 1
			}
			return
		}
	}
}

func profileFocusNeedles(focus string) []string {
	needles := []string{focus}
	if words := strings.Fields(focus); len(words) > 0 {
		needles = append(needles, words[0])
	}
	return needles
}

func (m Model) profileFrameDimensions() (int, int) {
	return m.shellFrameDimensions()
}

func newProfileViewport() viewport.Model { return viewport.New(1, 1) }
