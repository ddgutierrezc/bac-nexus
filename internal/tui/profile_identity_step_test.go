package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"strings"
	"testing"
	"unicode"

	"github.com/muesli/termenv"
)

func newProfileIdentityTestModel(t *testing.T, w, h int) Model {
	t.Helper()
	m := newProfileConnectionTestModel(t, &profileStoreStub{}, w, h)
	m.connectionHost.SetValue("ibmi.example.test")
	m.connectionUsername.SetValue("USER")
	m.connectionFocus = profileConnectionFocusContinue
	u, c := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if c == nil {
		t.Fatal("connection seam missing")
	}
	u, _ = u.(Model).Update(c())
	return u.(Model)
}
func TestProfileIdentitySelectionAndLocalSeam(t *testing.T) {
	m := newProfileIdentityTestModel(t, 120, 40)
	if m.identityFocus != profileIdentityFocusKnown || m.identityDecision != profileIdentityNone {
		t.Fatal("initial identity state")
	}
	u, c := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if c != nil || u.(Model).identityDecision != profileIdentityNone {
		t.Fatal("enter selected without choice")
	}
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = u.(Model)
	if m.identityDecision != profileIdentityKnownFingerprint {
		t.Fatal("space did not select known")
	}
	u, c = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if c != nil || u.(Model).identityDecision != profileIdentityKnownFingerprint {
		t.Fatal("enter on a selected choice must not continue or change selection")
	}
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = u.(Model)
	u, c = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if c == nil {
		t.Fatal("focused continue did not continue")
	}
	u, _ = u.(Model).Update(c())
	m = u.(Model)
	if m.identityBranch != profileIdentityBranchFingerprint || m.screen != screenProfileIdentity {
		t.Fatal("identity seam drifted")
	}
}

func TestProfileIdentityFocusCycleAndInputSemantics(t *testing.T) {
	m := newProfileIdentityTestModel(t, 80, 24)
	if m.identityFocus != profileIdentityFocusKnown || m.identityDecision != profileIdentityNone {
		t.Fatalf("initial state = focus %v decision %v", m.identityFocus, m.identityDecision)
	}
	forward := []profileIdentityFocus{profileIdentityFocusObserved, profileIdentityFocusBack, profileIdentityFocusContinue, profileIdentityFocusKnown}
	for _, want := range forward {
		u, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = u.(Model)
		if m.identityFocus != want {
			t.Fatalf("tab focus = %v, want %v", m.identityFocus, want)
		}
	}
	reverse := []profileIdentityFocus{profileIdentityFocusContinue, profileIdentityFocusBack, profileIdentityFocusObserved, profileIdentityFocusKnown}
	for _, want := range reverse {
		u, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
		m = u.(Model)
		if m.identityFocus != want {
			t.Fatalf("shift-tab focus = %v, want %v", m.identityFocus, want)
		}
	}
	for _, focus := range []profileIdentityFocus{profileIdentityFocusBack, profileIdentityFocusContinue} {
		m.identityFocus, m.identityDecision = focus, profileIdentityNone
		u, cmd := m.Update(tea.KeyMsg{Type: tea.KeySpace})
		if cmd != nil || u.(Model).identityDecision != profileIdentityNone {
			t.Fatalf("space on focus %v changed decision", focus)
		}
	}
	m.identityFocus = profileIdentityFocusObserved
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = u.(Model)
	if m.identityDecision != profileIdentityObservedKey {
		t.Fatal("space on observed choice did not select it")
	}
	for _, focus := range []profileIdentityFocus{profileIdentityFocusKnown, profileIdentityFocusObserved} {
		m.identityFocus = focus
		u, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if cmd != nil || u.(Model).screen != screenProfileIdentity {
			t.Fatalf("enter on focus %v escaped the local identity step", focus)
		}
	}
	m.identityFocus = profileIdentityFocusContinue
	u, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("selected continue did not emit the local acceptance seam")
	}
	accepted, _ := u.(Model).Update(cmd())
	if accepted.(Model).identityBranch != profileIdentityBranchObservedKey {
		t.Fatal("observed decision did not retain its local branch")
	}
}

func TestProfileIdentityBackPreservesWizardDraftsAndNewWizardResets(t *testing.T) {
	m := newProfileIdentityTestModel(t, 80, 24)
	m.profileDraftName = "desarrollo"
	m.connectionDraft = profileConnectionDraft{host: "ibmi.example.test", username: "USER", port: 22}
	m.identityFocus, m.identityDecision = profileIdentityFocusObserved, profileIdentityObservedKey
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = u.(Model)
	if m.screen != screenProfileConnection || m.profileDraftName != "desarrollo" || m.connectionDraft.host != "ibmi.example.test" || m.identityDecision != profileIdentityObservedKey || m.identityFocus != profileIdentityFocusObserved {
		t.Fatalf("identity back lost wizard state: %#v", m)
	}
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = u.(Model)
	if m.screen != screenProfileStep || m.profileDraftName != "desarrollo" || m.connectionHost.Value() != "ibmi.example.test" {
		t.Fatal("connection back did not preserve steps 1 and 2")
	}
	m.beginProfileStep()
	if m.connectionReady || m.connectionDraft != (profileConnectionDraft{}) || m.identityDecision != profileIdentityNone || m.identityFocus != profileIdentityFocusKnown {
		t.Fatal("new wizard did not reset steps 2 and 3")
	}
}

func TestProfileIdentityLocalNoticeAndCompleteContentReachability(t *testing.T) {
	m := newProfileIdentityTestModel(t, 40, 16)
	m.noColor = true
	seen := ""
	for range 120 {
		u, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
		m = u.(Model)
	}
	for range 120 {
		seen += "\n" + m.View()
		u, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = u.(Model)
	}
	compact := alphaNumericOnly(seen)
	panelCompact := alphaNumericOnly(m.wizardPanelContent())
	for _, want := range []string{
		"Paso 3 de 9 — Identidad", "Tab siguiente", "Verificar un fingerprint conocido", "Confiar en la clave observada ahora",
		"Esta decisión solo se registra localmente", "no conecta con el servidor", "ni guarda credenciales o perfiles todavía.", "< VOLVER >", "[ CONTINUAR ]",
	} {
		needle := alphaNumericOnly(want)
		if !strings.Contains(compact, needle) && !strings.Contains(panelCompact, needle) {
			t.Fatalf("viewport did not reconstruct %q:\n%s", want, seen)
		}
	}
}

func alphaNumericOnly(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, text)
}
func TestProfileIdentityRendersBounded(t *testing.T) {
	for _, s := range []struct{ w, h int }{{120, 40}, {80, 24}, {40, 16}} {
		t.Run("size", func(t *testing.T) {
			m := newProfileIdentityTestModel(t, s.w, s.h)
			v := m.View()
			for _, x := range []string{"Paso 3 de 9 — Identidad", "Identidad del servidor", "Verificar un fingerprint conocido", "Confiar en la clave observada ahora", "< VOLVER >", "[ CONTINUAR ]"} {
				if !strings.Contains(v, x) && s.w >= 120 {
					t.Fatalf("missing %q", x)
				}
			}
			if lipgloss.Width(v) > s.w {
				t.Fatalf("horizontal overflow %dx%d", lipgloss.Width(v), lipgloss.Height(v))
			}
		})
	}
}
func TestWrapWizardTextLosslessAndBounded(t *testing.T) {
	for _, tt := range []struct {
		text  string
		width int
	}{{"áéíóú palabra", 6}, {"abcdefghijk", 4}, {"uno\ndos", 4}} {
		for _, line := range wrapWizardText(tt.text, tt.width, "[--] ") {
			if lipgloss.Width(line) > tt.width {
				t.Fatalf("wrapped overflow %q", line)
			}
		}
	}
}

func TestWrapWizardTextKeepsPrefixAlignmentAndAllContent(t *testing.T) {
	lines := wrapWizardText("áéíóú abcdefghijkl\nsiguiente", 12, "[ERR] ")
	if len(lines) < 3 || !strings.HasPrefix(lines[0], "[ERR] ") {
		t.Fatalf("prefix lost: %#v", lines)
	}
	indent := strings.Repeat(" ", lipgloss.Width("[ERR] "))
	for i, line := range lines {
		if lipgloss.Width(line) > 12 {
			t.Fatalf("line %d overflowed: %q", i, line)
		}
		if i > 0 && !strings.HasPrefix(line, indent) {
			t.Fatalf("continuation indent = %q, want %q", line, indent)
		}
	}
	parts := make([]string, len(lines))
	parts[0] = strings.TrimPrefix(lines[0], "[ERR] ")
	for i := 1; i < len(lines); i++ {
		parts[i] = strings.TrimPrefix(lines[i], indent)
	}
	joined := strings.Join(parts, "")
	if !strings.Contains(joined, "áéíóú") || !strings.Contains(joined, "siguiente") {
		t.Fatalf("content was lost: %#v", lines)
	}
}

func TestWrapWizardTextPreservesWideRunesAndBlankLines(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		width      int
	}{
		{"wide", "漢字🙂", 1}, {"combining", "e\u0301 café", 2}, {"blank", "uno\n\ndos", 3}, {"long", "abcdefghijkl", 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lines := wrapWizardText(tt.text, tt.width, "[--] ")
			joined := strings.ReplaceAll(strings.Join(lines, ""), "[--] ", "")
			joined = strings.ReplaceAll(joined, strings.Repeat(" ", lipgloss.Width("[--] ")), "")
			for _, r := range tt.text {
				if r != '\n' && !strings.ContainsRune(joined, r) {
					t.Fatalf("lost %q in %#v", r, lines)
				}
			}
			if tt.name == "blank" && len(lines) < 3 {
				t.Fatalf("blank line lost: %#v", lines)
			}
		})
	}
}

func TestWrapWizardTextReconstructsNormalizedParagraphs(t *testing.T) {
	for _, tt := range []struct {
		name, text, prefix string
		width              int
	}{
		{"spaces-tabs", "uno   dos\t\ttres", "[E] ", 5},
		{"wide", "漢字  café\t🙂", "", 2},
		{"combining", "e\u0301\tcole", "> ", 2},
		{"long-token", "abcdefghijkl", "", 1},
		{"blank-and-trailing", "uno\n\n dos \n", "! ", 4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lines := wrapWizardText(tt.text, tt.width, tt.prefix)
			for _, line := range lines {
				if lipgloss.Width(line) > tt.width && !(tt.width == 1 && lipgloss.Width(line) == 2) {
					t.Fatalf("avoidable overflow: %q", line)
				}
			}
			if got, want := reconstructWrappedParagraphs(lines, tt.text, tt.prefix), normalizeWrappedText(tt.text); got != want {
				t.Fatalf("reconstruction = %q, want %q; lines=%#v", got, want, lines)
			}
		})
	}
}

func normalizeWrappedText(text string) string {
	paragraphs := strings.Split(text, "\n")
	for i, paragraph := range paragraphs {
		paragraphs[i] = strings.Join(strings.Fields(paragraph), " ")
	}
	return strings.Join(paragraphs, "\n")
}

func reconstructWrappedParagraphs(lines []string, text, prefix string) string {
	indent := strings.Repeat(" ", lipgloss.Width(prefix))
	if lipgloss.Width(prefix) >= 1 && lipgloss.Width(prefix) >= maxLineWidth(lines) {
		prefix, indent = "", ""
	}
	want := strings.Split(normalizeWrappedText(text), "\n")
	got := make([]string, len(want))
	line := 0
	for paragraph := range want {
		for line < len(lines) {
			part := lines[line]
			if line == 0 && prefix != "" {
				part = strings.TrimPrefix(part, prefix)
			} else {
				part = strings.TrimPrefix(part, indent)
			}
			line++
			got[paragraph] += part
			if got[paragraph] == want[paragraph] {
				break
			}
		}
	}
	return strings.Join(got, "\n")
}

func maxLineWidth(lines []string) int {
	width := 0
	for _, line := range lines {
		width = max(width, lipgloss.Width(line))
	}
	return width
}

func TestWizardChoiceUsesIndependentFocusSelectionAndBoundedWrapping(t *testing.T) {
	choice := WizardChoice{ID: "observed", Label: "Confiar en la clave observada ahora", Description: "Descripción con palabras suficientemente largas para envolver.", Note: "Nota de seguridad importante."}
	view := renderWizardChoice(24, newHomeTheme(true), choice, true, false)
	if !strings.Contains(view, "▸ ( )") || strings.Contains(view, "(*)") {
		t.Fatalf("focus selected a choice: %q", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > 24 {
			t.Fatalf("choice line overflowed: %q", line)
		}
	}
	selected := renderWizardChoice(24, newHomeTheme(true), choice, false, true)
	if !strings.Contains(selected, "  (*)") || strings.Contains(selected, "▸") {
		t.Fatalf("selection/focus are coupled: %q", selected)
	}
}

func TestIdentityPanelUsesStructuredChoiceAndActionRanges(t *testing.T) {
	for _, size := range []struct{ width, height int }{{80, 24}, {40, 16}} {
		t.Run("size", func(t *testing.T) {
			m := newProfileIdentityTestModel(t, size.width, size.height)
			fw, fh := m.shellFrameDimensions()
			panel := m.renderProfileIdentityPanelContent(m.shellInnerWidth(fw), m.shellInnerHeight(fh), newHomeTheme(true))
			for _, focus := range []profileIdentityFocus{profileIdentityFocusObserved, profileIdentityFocusBack, profileIdentityFocusContinue} {
				r, ok := panel.ranges[focus]
				if !ok || r.end < r.start {
					t.Fatalf("missing structured range for %v: %#v", focus, panel.ranges)
				}
			}
			observed := panel.ranges[profileIdentityFocusObserved]
			if observed.end-observed.start < 2 {
				t.Fatalf("choice 2 range omitted description/note: %#v", observed)
			}
			for range 3 {
				u, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
				m = u.(Model)
			}
			r := panel.ranges[m.identityFocus]
			if m.wizardFocusStart != r.start || m.wizardFocusEnd != r.end {
				t.Fatalf("focus range = %d..%d, want structured %d..%d", m.wizardFocusStart, m.wizardFocusEnd, r.start, r.end)
			}
		})
	}
}

func TestIdentityViewportFollowsFocusAndRetainsReachability(t *testing.T) {
	for _, size := range []struct{ width, height int }{{80, 24}, {40, 16}} {
		t.Run("viewport", func(t *testing.T) {
			m := newProfileIdentityTestModel(t, size.width, size.height)
			if size.width == 80 && (strings.Contains(m.View(), "▲ más") || strings.Contains(m.View(), "▼ más")) == false {
				t.Fatal("narrow identity panel must disclose hidden content")
			}
			for _, want := range []profileIdentityFocus{profileIdentityFocusObserved, profileIdentityFocusBack, profileIdentityFocusContinue} {
				u, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
				m = u.(Model)
				if m.identityFocus != want {
					t.Fatalf("focus = %v, want %v", m.identityFocus, want)
				}
				anchor := "[ CONTINUAR ]"
				if want == profileIdentityFocusObserved {
					anchor = "Confiar"
				}
				if want == profileIdentityFocusBack {
					anchor = "< VOLVER >"
				}
				if !strings.Contains(m.View(), anchor) {
					t.Fatalf("focused anchor %q was clipped: %q", anchor, m.View())
				}
			}
		})
	}
}

func TestIdentityManualViewportScrollReachesChoiceAndActionsAt40x16(t *testing.T) {
	m := newProfileIdentityTestModel(t, 40, 16)
	m.noColor = true
	// Focus choice 2 first; its structured range must reveal the block start.
	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = u.(Model)
	if !strings.Contains(m.View(), "Confiar") {
		t.Fatalf("choice 2 start is not focused:\n%s", m.View())
	}
	seen := m.View()
	for range 80 {
		u, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = u.(Model)
		seen += "\n" + m.View()
	}
	for _, want := range []string{"Nexus inspeccionará", "Esta primera observación", "< VOLVER >", "[ CONTINUAR ]"} {
		if !strings.Contains(seen, want) {
			t.Fatalf("manual viewport scrolling did not reach %q:\n%s", want, seen)
		}
	}
	if !strings.Contains(m.View(), "▲ más") || strings.Contains(m.View(), "▼ más") {
		t.Fatalf("bottom overflow indicator is incorrect:\n%s", m.View())
	}
}

func TestWizardRenderProofTrueColorAndNoColor(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
	for _, color := range []bool{true, false} {
		if color {
			lipgloss.SetColorProfile(termenv.Ascii)
		} else {
			lipgloss.SetColorProfile(termenv.TrueColor)
		}
		for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {40, 16}} {
			for _, wizard := range []struct {
				name string
				new  func() Model
			}{
				{"step-1", func() Model { return newProfileStepTestModel(&profileStoreStub{}, size.width, size.height) }},
				{"step-2", func() Model { return newProfileConnectionTestModel(t, &profileStoreStub{}, size.width, size.height) }},
				{"step-3", func() Model { return newProfileIdentityTestModel(t, size.width, size.height) }},
			} {
				t.Run(wizard.name, func(t *testing.T) {
					m := wizard.new()
					m.noColor = color
					m.refreshWizardViewport()
					view := m.View()
					t.Logf("%s %dx%d noColor=%v:\n%s", wizard.name, size.width, size.height, color, view)
					if lipgloss.Width(view) > size.width || lipgloss.Height(view) > size.height {
						t.Fatalf("render bounds = %dx%d", lipgloss.Width(view), lipgloss.Height(view))
					}
					if color && strings.Contains(view, "\x1b[") {
						t.Fatalf("NO_COLOR render contains ANSI: %q", view)
					}
					if !color && !strings.Contains(view, "\x1b[") {
						t.Fatalf("true-color render lacks ANSI: %q", view)
					}
				})
			}
		}
	}
}
