package tui

import "github.com/charmbracelet/lipgloss"

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
