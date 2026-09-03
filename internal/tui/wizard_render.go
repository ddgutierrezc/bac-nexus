package tui

import "strings"

// wizardFeedbackKind defines the complete semantic vocabulary available inside
// wizard panels. It keeps feedback presentation independent from Home status rows.
type wizardFeedbackKind uint8

const (
	wizardFeedbackOK wizardFeedbackKind = iota
	wizardFeedbackInfo
	wizardFeedbackWarning
	wizardFeedbackError
	wizardFeedbackNeutral
)

type wizardFeedback struct {
	kind    wizardFeedbackKind
	message string
}

func renderWizardFeedback(width int, t homeTheme, feedback wizardFeedback) string {
	prefix, style := "[--] ", t.statusNeutral
	switch feedback.kind {
	case wizardFeedbackOK:
		prefix, style = "[OK] ", t.statusOK
	case wizardFeedbackInfo:
		prefix, style = "[INFO] ", t.statusInfo
	case wizardFeedbackWarning:
		prefix, style = "[WARN] ", t.statusWarning
	case wizardFeedbackError:
		prefix, style = "[ERR] ", t.statusError
	}
	lines := wrapWizardText(feedback.message, width, prefix)
	for i := range lines {
		lines[i] = style.Render(lines[i])
	}
	return strings.Join(lines, "\n")
}

func wizardFeedbackFromRow(row string) (wizardFeedback, bool) {
	marker, message, found := strings.Cut(row, " ")
	if !found || message == "" {
		return wizardFeedback{}, false
	}
	kind := wizardFeedbackNeutral
	switch marker {
	case "[OK]":
		kind = wizardFeedbackOK
	case "[INFO]":
		kind = wizardFeedbackInfo
	case "[WARN]":
		kind = wizardFeedbackWarning
	case "[ERR]":
		kind = wizardFeedbackError
	case "[--]":
	default:
		return wizardFeedback{}, false
	}
	return wizardFeedback{kind: kind, message: message}, true
}
