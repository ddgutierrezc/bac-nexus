package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
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

func TestWizardTitleAndDividerHelpersAreShared(t *testing.T) {
	theme := newHomeTheme(true)
	if got, want := renderProfileStepTitleRow(66, theme), renderWizardTitleRow(66, theme, "Crear perfil IBM i", "Paso 1 de 9 — Perfil"); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("Step 1 title helper drifted: %q != %q", got, want)
	}
	if got, want := renderProfileConnectionTitleRow(66, theme), renderWizardTitleRow(66, theme, connectionPanelTitle, connectionStepIndicator); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("Step 2 title helper drifted: %q != %q", got, want)
	}
	for _, width := range []int{66, 30, 12} {
		for _, line := range renderWizardTitleRow(width, theme, connectionPanelTitle, connectionStepIndicator) {
			if lipgloss.Width(line) > width {
				t.Fatalf("title fallback overflowed %d: %q", width, line)
			}
		}
		if divider := renderWizardDivider(width, theme); lipgloss.Width(divider) != width {
			t.Fatalf("divider width = %d, want %d", lipgloss.Width(divider), width)
		}
	}
}

func TestProfileConnectionGridReservesSharedLabelColumn(t *testing.T) {
	m := newProfileConnectionTestModel(t, &profileStoreStub{}, 120, 40)
	theme := newHomeTheme(true)
	rows := []string{
		m.renderConnectionInputRow("Host", m.connectionHost, profileConnectionFocusHost, 66, theme),
		m.renderConnectionInputRow("Usuario", m.connectionUsername, profileConnectionFocusUsername, 66, theme),
		m.renderConnectionPortRow(66, theme),
	}
	firstBracket := strings.Index(rows[0], "[")
	bracket := lipgloss.Width(rows[0][:firstBracket])
	for _, row := range rows {
		index := strings.Index(row, "[")
		if column := lipgloss.Width(row[:index]); column != bracket {
			t.Fatalf("input bracket column = %d, want %d: %q", column, bracket, row)
		}
	}
	if !strings.Contains(rows[2], " ]  Predeterminado: 22") {
		t.Fatalf("port metadata gap drifted: %q", rows[2])
	}
}

func TestProfileConnectionColorRenderPreservesHierarchyAndBounds(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })

	m := newProfileConnectionTestModel(t, &profileStoreStub{}, 120, 40)
	m.noColor = false
	view := m.View()
	t.Logf("Step 2 color render 120x40:\n%s", view)
	if !strings.Contains(view, "\x1b[") || !strings.Contains(view, "Paso 2 de 9 — Conexión") || !strings.Contains(view, "▸") {
		t.Fatalf("color render lost hierarchy: %q", view)
	}
	if lipgloss.Width(view) > 120 || lipgloss.Height(view) > 40 {
		t.Fatalf("color render bounds = %dx%d", lipgloss.Width(view), lipgloss.Height(view))
	}
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > 120 {
			t.Fatalf("color line overflowed: %q", line)
		}
	}
}
