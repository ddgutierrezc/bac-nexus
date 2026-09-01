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
	body := strings.TrimRight(vp.View(), "\n") + "\n" + wizardOverflowIndicator(vp, width, t, m.text("overflow.above", nil), m.text("overflow.below", nil))

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
}, width int, t homeTheme, above, below string) string {
	indicator := ""
	switch {
	case !vp.AtTop() && !vp.AtBottom():
		indicator = above + "  " + below
	case !vp.AtTop():
		indicator = above
	case !vp.AtBottom():
		indicator = below
	}
	if indicator == "" {
		return ""
	}
	return t.metadata.Width(width).Align(lipgloss.Right).Render(indicator)
}
