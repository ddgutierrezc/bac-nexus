package tui

import (
	"context"
	"strings"
	"testing"

	"bac-nexus/internal/localization"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestDirectOnboardingRuntimeFramesRemainBounded(t *testing.T) {
	for _, frame := range []struct{ width, height int }{{120, 40}, {80, 24}, {40, 16}} {
		m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
		m.beginDirectOnboarding()
		updated, _ := m.Update(tea.WindowSizeMsg{Width: frame.width, Height: frame.height})
		view := updated.(Model).View()
		if !strings.Contains(view, "CONECTAR Y GUARDAR") || lipgloss.Width(view) > frame.width || lipgloss.Height(view) > frame.height {
			t.Fatalf("frame invalid at %dx%d: %q", frame.width, frame.height, view)
		}
	}
}

func TestEditLocalizedValidationFramesRemainReachable(t *testing.T) {
	for _, locale := range []struct {
		name, want, narrowWant string
		localizer              localization.Localizer
	}{
		{"spanish", "nombre de perfil válido", "Ingresa un nombre", localization.Spanish()},
		{"english", "valid profile name", "Enter a valid", localization.English()},
	} {
		t.Run(locale.name, func(t *testing.T) {
			for _, frame := range []struct {
				width, height int
				noColor       bool
			}{{120, 40, false}, {80, 24, true}, {40, 16, false}} {
				m := NewModel(&profileStoreStub{})
				m.localizer, m.noColor = locale.localizer, frame.noColor
				original := testProfile("edit-profile")
				original.SchemaVersion = profile.SchemaVersionV3
				m.beginForm(original, screenDetail)
				m.form[0].input.SetValue("!")
				updated, _ := m.Update(tea.WindowSizeMsg{Width: frame.width, Height: frame.height})
				m = updated.(Model)
				for range len(m.form) {
					updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
					m = updated.(Model)
				}
				updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
				m = updated.(Model)
				view := m.View()
				want := locale.want
				if frame.width == 40 {
					want = locale.narrowWant
				}
				if m.formValidation == nil || !strings.Contains(view, want) || lipgloss.Width(view) > frame.width || lipgloss.Height(view) > frame.height {
					t.Fatalf("localized Edit frame invalid at %dx%d: validation=%#v view=%q", frame.width, frame.height, m.formValidation, view)
				}
				if frame.width == 40 && !strings.Contains(view, "▼") {
					t.Fatalf("narrow Edit frame does not disclose overflow: %q", view)
				}
				if frame.noColor && strings.Contains(view, "\x1b[") {
					t.Fatalf("NO_COLOR Edit frame contains ANSI: %q", view)
				}
			}
		})
	}
}
