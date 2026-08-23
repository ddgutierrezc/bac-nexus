package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

type wizardInputOptions struct {
	compactEditableWidth int
	labelWidth           int
	suffix               string
}

// WizardChoice is a stable, transport-free description of a selectable wizard
// decision. Selection belongs to the caller; rendering focus never changes it.
type WizardChoice struct {
	ID          string
	Label       string
	Description string
	Note        string
}

// wizardRenderedBlock retains a rendered shared control with its own relative
// line range. Parent panels translate this range once while composing content;
// focus handling must never rediscover it by matching display text.
type wizardRenderedBlock struct {
	text       string
	start, end int
}

// renderWizardChoice renders a complete focus surface, including wrapped
// description and note lines. Continuations align below the description start
// (the display width of marker + radio + separator, exactly).
func renderWizardChoice(width int, t homeTheme, choice WizardChoice, focused, selected bool) string {
	return renderWizardChoiceBlock(width, t, choice, focused, selected).text
}

func renderWizardChoiceBlock(width int, t homeTheme, choice WizardChoice, focused, selected bool) wizardRenderedBlock {
	if width < 1 {
		return wizardRenderedBlock{}
	}
	marker := "  "
	if focused {
		marker = "▸ "
	}
	radio := "( )"
	if selected {
		radio = "(*)"
	}
	labelStyle := t.fieldsetTitle
	if focused {
		labelStyle = t.selectedLabel
	}
	prefix := marker + radio + " "
	indent := strings.Repeat(" ", lipgloss.Width(prefix))
	labelLines := wrapWizardText(choice.Label, width-lipgloss.Width(prefix), "")
	if len(labelLines) == 0 {
		labelLines = []string{""}
	}
	lines := make([]string, 0, len(labelLines)+4)
	for i, line := range labelLines {
		if i == 0 {
			line = prefix + line
		} else {
			line = indent + line
		}
		lines = append(lines, labelStyle.Render(line))
	}
	for _, line := range wrapWizardText(choice.Description, width, indent) {
		lines = append(lines, t.metadata.Render(line))
	}
	if choice.Note != "" {
		for _, line := range wrapWizardText(choice.Note, width, indent) {
			lines = append(lines, t.statusWarning.Render(line))
		}
	}
	text := strings.Join(lines, "\n")
	block := wizardRenderedBlock{text: text, start: 0, end: len(lines) - 1}
	if focused {
		block.text = t.selectedRow.Width(width).Render(text)
		return block
	}
	block.text = t.fieldsetContent.Width(width).Render(text)
	return block
}

// wizardRhythm is the only vertical-spacing policy for wizard Steps 1–3.
// Compact terminals retain controls and feedback, while taller screens gain
// only semantic separation rather than blank rows between every line.
type wizardRhythm struct {
	top, titleDivider, sectionDescription, relatedText            int
	descriptionControl, controls, feedback, actions, panelPadding int
}

func newWizardRhythm(height int) wizardRhythm {
	if height >= 30 {
		return wizardRhythm{top: 3, titleDivider: 1, sectionDescription: 1, relatedText: 0, descriptionControl: 1, controls: 1, feedback: 1, actions: 2, panelPadding: 1}
	}
	if height >= 18 {
		return wizardRhythm{top: 1, titleDivider: 0, sectionDescription: 0, relatedText: 0, descriptionControl: 1, controls: 0, feedback: 1, actions: 1}
	}
	return wizardRhythm{}
}

func appendWizardGap(lines []string, count int) []string {
	for range count {
		lines = append(lines, "")
	}
	return lines
}

func renderWizardHeader(width int, t homeTheme, profile string) string {
	segments := []struct {
		text  string
		style lipgloss.Style
	}{
		{"BAC NEXUS", t.headerBrand}, {"PERFIL: " + profile, t.headerProfile}, {"ESTADO: CONFIGURANDO", t.headerStatus},
	}
	joined := "  " + strings.Join([]string{segments[0].text, "│", segments[1].text, "│", segments[2].text}, "  ")
	if lipgloss.Width(joined) <= width {
		return t.header.Width(width).Render(joined)
	}
	lines := make([]string, 0, 3)
	for _, segment := range segments {
		for _, line := range wrapWizardText(segment.text, width, "") {
			lines = append(lines, segment.style.Render(line))
		}
	}
	return strings.Join(lines, "\n")
}

// renderWizardTitleRow keeps all wizard panels on a shared title, salmon step
// indicator, and responsive fallback contract.
func renderWizardTitleRow(width int, t homeTheme, title, step string) []string {
	const minimumGap = 2
	if width >= lipgloss.Width(title)+minimumGap+lipgloss.Width(step) {
		return []string{t.panelTitle.Render(title) + strings.Repeat(" ", width-lipgloss.Width(title)-lipgloss.Width(step)) + t.fieldsetTitle.Render(step)}
	}
	lines := make([]string, 0)
	for _, line := range wrapWizardText(title, width, "") {
		lines = append(lines, t.panelTitle.Render(line))
	}
	for _, line := range wrapWizardText(step, width, "") {
		lines = append(lines, t.fieldsetTitle.Render(line))
	}
	return lines
}

func renderWizardDivider(width int, t homeTheme) string {
	return t.fieldsetBorder.Render(strings.Repeat("─", max(width, 1)))
}

// renderWizardInputRow keeps wizard fields visually and behaviorally aligned:
// the Bubbles input owns the cursor and editing viewport, while this shared
// shell owns focus marker, brackets, padding, and field surface.
func renderWizardInputRow(label string, input textinput.Model, focused bool, width int, t homeTheme, options wizardInputOptions) string {
	focusMarker := "  "
	labelStyle, valueStyle := t.fieldsetTitle, t.metadata
	if focused {
		focusMarker = t.selectedMarker.Render("▸ ")
		labelStyle, valueStyle = t.selectedLabel, t.selectedLabel
		// Bubbles v1 renders the configured cursor glyph through its placeholder
		// path when an empty focused input is viewed.
		if input.Value() == "" {
			input.Placeholder = "█"
		}
	}
	if options.labelWidth > 0 {
		label = label + strings.Repeat(" ", max(options.labelWidth-lipgloss.Width(label), 0))
	}
	input.TextStyle = valueStyle
	input.PlaceholderStyle = valueStyle
	input.Cursor.Style = valueStyle
	input.Cursor.TextStyle = valueStyle
	prefix := focusMarker + labelStyle.Render(label) + valueStyle.Render("  [ ")
	// At constrained widths retain the label and real input independently;
	// neither is functional chrome that may be ellipsized.
	if lipgloss.Width(prefix)+lipgloss.Width(" ]") > width {
		labelLines := wrapWizardText(focusMarker+label, width, "")
		for i := range labelLines {
			labelLines[i] = labelStyle.Render(labelLines[i])
		}
		input.Width = max(width-lipgloss.Width("[  ]")-1, 1)
		row := "[ " + input.View() + " ]"
		if focused {
			return t.selectedRow.Width(width).Render(strings.Join(append(labelLines, valueStyle.Render(row)), "\n"))
		}
		return t.fieldsetContent.Width(width).Render(strings.Join(append(labelLines, valueStyle.Render(row)), "\n"))
	}
	editableWidth := max(width-lipgloss.Width(prefix)-lipgloss.Width(" ]")-lipgloss.Width(options.suffix), 1)
	if options.compactEditableWidth > 0 {
		editableWidth = options.compactEditableWidth
	}
	// Bubbles reserves a cursor cell beyond Width, so leave one cell inside the
	// bracketed surface rather than letting the closing bracket wrap.
	input.Width = max(editableWidth-1, 1)
	view := input.View()
	row := prefix + view + strings.Repeat(" ", max(editableWidth-lipgloss.Width(view), 0)) + valueStyle.Render(" ]") + options.suffix
	if focused {
		return t.selectedRow.Width(width).Render(row)
	}
	return t.fieldsetContent.Width(width).Render(row)
}

// renderWizardActions preserves the approved Step 1 action focus semantics
// for every wizard screen without introducing a second focus surface.
type wizardActionOptions struct{ rightEnabled bool }

func renderWizardActions(width int, t homeTheme, left, right string, leftFocused, rightFocused, noColor bool, options ...wizardActionOptions) string {
	return renderWizardActionsBlock(width, t, left, right, leftFocused, rightFocused, noColor, options...).text
}

func renderWizardActionsBlock(width int, t homeTheme, left, right string, leftFocused, rightFocused, noColor bool, options ...wizardActionOptions) wizardRenderedBlock {
	rightEnabled := true
	if len(options) > 0 {
		rightEnabled = options[0].rightEnabled
	}
	if leftFocused {
		left = "▸ " + left
	}
	if rightFocused {
		right = "▸ " + right
		if rightEnabled && !noColor {
			right = lipgloss.NewStyle().Background(lipgloss.Color(bacRed)).Foreground(lipgloss.Color(white)).Bold(true).Render(right)
		}
	}
	if !rightEnabled {
		right += " [--]"
		right = t.statusNeutral.Render(right)
	}
	line := left + "    " + right
	if lipgloss.Width(line) > width {
		return wizardRenderedBlock{text: left + "\n" + right, start: 0, end: 1}
	}
	return wizardRenderedBlock{text: lipgloss.PlaceHorizontal(width, lipgloss.Right, line), start: 0, end: 0}
}
