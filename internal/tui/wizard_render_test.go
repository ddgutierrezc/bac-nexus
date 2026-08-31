package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestWizardRenderThemeHierarchy(t *testing.T) {
	theme := newHomeTheme(false)
	for _, tt := range []struct {
		name, got, want string
	}{
		{"active marker", fmt.Sprint(theme.selectedMarker.GetForeground()), bacRed},
		{"active label", fmt.Sprint(theme.selectedLabel.GetForeground()), textPrimary},
		{"inactive label", fmt.Sprint(theme.fieldsetTitle.GetForeground()), salmonMuted},
		{"inactive value", fmt.Sprint(theme.metadata.GetForeground()), mutedMetadata},
		{"content heading", fmt.Sprint(theme.wizardContentHeading.GetForeground()), textPrimary},
		{"step indicator", fmt.Sprint(theme.fieldsetTitle.GetForeground()), salmonMuted},
		{"divider", fmt.Sprint(theme.fieldsetBorder.GetForeground()), borderSurface},
		{"selected surface", fmt.Sprint(theme.selectedRow.GetBackground()), insetSurface},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}

	input := textinput.New()
	input.Focus()
	active := renderWizardInputRow("Host", input, true, 40, theme, wizardInputOptions{})
	inactive := renderWizardInputRow("Host", input, false, 40, theme, wizardInputOptions{})
	if !strings.Contains(active, theme.selectedMarker.Render("▸ ")) || !strings.Contains(active, theme.selectedLabel.Render("Host")) {
		t.Fatalf("active hierarchy omitted shared marker or label role: %q", active)
	}
	if strings.Contains(inactive, "▸") || !strings.Contains(inactive, theme.fieldsetTitle.Render("Host")) {
		t.Fatalf("inactive hierarchy did not use shared secondary label role: %q", inactive)
	}
}

func TestWizardActionStatesPreserveProgressSemantics(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	for _, noColor := range []bool{false, true} {
		for _, tt := range []struct {
			name            string
			options         []wizardActionOptions
			wantMarker      bool
			wantUnavailable bool
			wantFocused     bool
		}{
			{name: "ready default", wantMarker: true, wantFocused: true},
			{name: "ready explicit", options: []wizardActionOptions{{rightState: wizardProgressReady}}, wantMarker: true, wantFocused: true},
			{name: "blocked", options: []wizardActionOptions{{rightState: wizardProgressBlocked}}, wantMarker: true},
			{name: "disabled", options: []wizardActionOptions{{rightState: wizardProgressDisabled}}, wantUnavailable: true},
		} {
			t.Run(fmt.Sprintf("%s/no-color=%t", tt.name, noColor), func(t *testing.T) {
				theme := newHomeTheme(noColor)
				output := renderWizardActionsBlock(80, theme, "< BACK >", "[ NEXT ]", false, true, noColor, tt.options...).text
				plain := ansiEscape.ReplaceAllString(output, "")
				if got := strings.Contains(plain, "▸ [ NEXT ]"); got != tt.wantMarker {
					t.Fatalf("focused marker = %t, want %t: %q", got, tt.wantMarker, output)
				}
				if got := strings.Contains(plain, "[--]"); got != tt.wantUnavailable {
					t.Fatalf("unavailable cue = %t, want %t: %q", got, tt.wantUnavailable, output)
				}
				if noColor {
					if ansiEscape.MatchString(output) {
						t.Fatalf("NO_COLOR action contains ANSI: %q", output)
					}
					return
				}
				focusedStyle := strings.TrimSuffix(lipgloss.NewStyle().Background(lipgloss.Color(bacRed)).Foreground(lipgloss.Color(white)).Bold(true).Render("x"), "x\x1b[0m")
				if got := strings.Contains(output, focusedStyle); got != tt.wantFocused {
					t.Fatalf("focused style = %t, want %t: %q", got, tt.wantFocused, output)
				}
				if tt.wantUnavailable {
					neutralStyle := strings.TrimSuffix(theme.statusNeutral.Render("x"), "x\x1b[0m")
					if !strings.Contains(output, neutralStyle) {
						t.Fatalf("disabled action lost neutral style: %q", output)
					}
				}
			})
		}
	}
}

func TestWizardSemanticFeedbackRendering(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
	for _, tt := range []struct {
		kind   wizardFeedbackKind
		marker string
		style  func(homeTheme) lipgloss.Style
	}{
		{wizardFeedbackOK, "[OK]", func(t homeTheme) lipgloss.Style { return t.statusOK }},
		{wizardFeedbackInfo, "[INFO]", func(t homeTheme) lipgloss.Style { return t.statusInfo }},
		{wizardFeedbackWarning, "[WARN]", func(t homeTheme) lipgloss.Style { return t.statusWarning }},
		{wizardFeedbackError, "[ERR]", func(t homeTheme) lipgloss.Style { return t.statusError }},
		{wizardFeedbackNeutral, "[--]", func(t homeTheme) lipgloss.Style { return t.statusNeutral }},
	} {
		t.Run(tt.marker, func(t *testing.T) {
			message := "mensaje completo con palabras suficientes para comprobar continuidad"
			colored := renderWizardFeedback(20, newHomeTheme(false), wizardFeedback{kind: tt.kind, message: message})
			plain := ansiEscape.ReplaceAllString(colored, "")
			styleCode := strings.TrimSuffix(tt.style(newHomeTheme(false)).Render("x"), "x\x1b[0m")
			if !strings.Contains(plain, tt.marker) || !strings.Contains(colored, styleCode) {
				t.Fatalf("semantic marker or style role missing: %q", colored)
			}
			lines := strings.Split(plain, "\n")
			for i, line := range lines {
				if lipgloss.Width(line) > 20 {
					t.Fatalf("line %d overflowed: %q", i, line)
				}
				if i > 0 && !strings.HasPrefix(line, strings.Repeat(" ", lipgloss.Width(tt.marker+" "))) {
					t.Fatalf("continuation indentation drifted: %q", line)
				}
			}
			if got := reconstructWrappedParagraphs(lines, message, tt.marker+" "); got != message {
				t.Fatalf("message was truncated or reconstructed incorrectly: %q", got)
			}
			noColor := renderWizardFeedback(20, newHomeTheme(true), wizardFeedback{kind: tt.kind, message: message})
			if ansiEscape.MatchString(noColor) || !strings.Contains(noColor, tt.marker) {
				t.Fatalf("NO_COLOR feedback drifted: %q", noColor)
			}
		})
	}
}

func TestWizardFeedbackPrecedence(t *testing.T) {
	m := Model{status: "[WARN] aviso", err: fmt.Errorf("fallo")}
	if got, _ := m.wizardFeedbackFor("[OK] válido"); got.kind != wizardFeedbackError || got.message != "fallo" {
		t.Fatalf("error did not win: %#v", got)
	}
	m.err = nil
	if got, _ := m.wizardFeedbackFor("[OK] válido"); got.kind != wizardFeedbackWarning || got.message != "aviso" {
		t.Fatalf("explicit status did not win: %#v", got)
	}
	m.status = ""
	if got, _ := m.wizardFeedbackFor("[OK] válido"); got.kind != wizardFeedbackOK || got.message != "válido" {
		t.Fatalf("validation was not revealed: %#v", got)
	}
}

type runtimePanel struct {
	left, right, top, bottom, width, height int
}

func runtimePanelHeight(t *testing.T, model Model, panel runtimePanel) int {
	t.Helper()
	for moves := 0; moves < 120; moves++ {
		lines := strings.Split(ansiEscape.ReplaceAllString(model.View(), ""), "\n")
		for bottom, line := range lines {
			if strings.Index(line, "└") == panel.left {
				return moves + bottom - panel.top + 1
			}
		}
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
	}
	t.Fatalf("runtime scrolling did not reveal the central panel bottom")
	return 0
}

func runtimePanelFromView(view string) (runtimePanel, bool) {
	lines := strings.Split(ansiEscape.ReplaceAllString(view, ""), "\n")
	for top, line := range lines {
		left, right := strings.Index(line, "┌"), strings.LastIndex(line, "┐")
		if left <= 0 || right <= left {
			continue
		}
		panel := runtimePanel{left: left, right: right, top: top, bottom: -1, width: lipgloss.Width(line[left : right+len("┐")])}
		for bottom := top + 1; bottom < len(lines); bottom++ {
			if strings.Index(lines[bottom], "└") == left {
				panel.bottom, panel.height = bottom, bottom-top+1
				break
			}
		}
		return panel, true
	}
	return runtimePanel{}, false
}

// runtimePanelFragments extracts only the central panel's visible content from
// real View frames. It intentionally has no access to internal render helpers.
func runtimePanelFragments(views []string) string {
	var fragments []string
	for _, view := range views {
		for _, line := range strings.Split(ansiEscape.ReplaceAllString(view, ""), "\n") {
			positions := make([]int, 0, 4)
			for offset := 0; ; {
				index := strings.Index(line[offset:], "│")
				if index < 0 {
					break
				}
				positions = append(positions, offset+index)
				offset += index + len("│")
			}
			if len(positions) >= 4 {
				fragments = append(fragments, line[positions[1]+len("│"):positions[2]])
			}
		}
	}
	return strings.Join(fragments, "\n")
}
