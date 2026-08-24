package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// wrapWizardText preserves explicit newlines (including blank paragraphs) and
// normalizes each non-blank paragraph's horizontal whitespace to one ASCII
// space. Removing the supplied first-line prefix and continuation indentation
// then reconstructs that normalized text exactly.
func wrapWizardText(text string, width int, prefix string) []string {
	if width < 1 {
		return []string{""}
	}
	// Prefixes are structural (for example "[ERR] "), so continuations start
	// exactly beneath the text that follows them. Do not add a cosmetic space:
	// that would make the visual and cell-width contracts disagree.
	if lipgloss.Width(prefix) >= width {
		prefix = ""
	}
	indent := strings.Repeat(" ", lipgloss.Width(prefix))
	var out []string
	firstLine := true
	for _, paragraph := range strings.Split(text, "\n") {
		line := indent
		if firstLine {
			line = prefix
		}
		words := strings.Fields(paragraph)
		for wordIndex, word := range words {
			separator := ""
			if wordIndex > 0 {
				separator = " "
			}
			for word != "" {
				available := width - lipgloss.Width(line) - lipgloss.Width(separator)
				freshCapacity := width - lipgloss.Width(indent)
				if separator != "" && lipgloss.Width(word) <= freshCapacity && lipgloss.Width(word)+lipgloss.Width(separator) > available && line != indent && line != prefix {
					if separator != "" && lipgloss.Width(line)+lipgloss.Width(separator) <= width {
						line += separator
						separator = ""
					}
					out = append(out, line)
					line = indent
					firstLine = false
					continue
				}
				if available < 1 {
					// The normalized separator is data, not decoration. Put it on
					// a continuation by itself when required rather than losing it.
					if separator != "" {
						if lipgloss.Width(line)+lipgloss.Width(separator) <= width {
							out = append(out, line+separator)
							separator = ""
						} else {
							out = append(out, line)
						}
						line = indent
						firstLine = false
						continue
					}
					out = append(out, line)
					line = indent
					firstLine = false
					continue
				}
				chunk := ""
				for _, r := range word {
					if lipgloss.Width(chunk+string(r)) > available {
						break
					}
					chunk += string(r)
				}
				if chunk == "" {
					if line != indent && line != prefix {
						out = append(out, line)
						line = indent
						firstLine = false
						continue
					}
					// A single wide rune at width one is the only physically
					// impossible bound. Emit it once to guarantee progress.
					for _, r := range word {
						line += separator + string(r)
						word = strings.TrimPrefix(word, string(r))
						separator = ""
						break
					}
				} else {
					line += separator + chunk
					word = strings.TrimPrefix(word, chunk)
					separator = ""
				}
				if word != "" {
					out = append(out, line)
					line = indent
					firstLine = false
				}
			}
		}
		out = append(out, line)
		firstLine = false
	}
	return out
}
