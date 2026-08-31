package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func alphaNumericOnly(text string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, text)
}

func reconstructWrappedParagraphs(lines []string, _ string, prefix string) string {
	indent := strings.Repeat(" ", lipgloss.Width(prefix))
	for i := range lines {
		if i == 0 {
			lines[i] = strings.TrimPrefix(lines[i], prefix)
		} else {
			lines[i] = strings.TrimPrefix(lines[i], indent)
		}
	}
	return strings.Join(lines, "")
}
