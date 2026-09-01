package tui

import (
	"context"
	"strings"
	"testing"

	"bac-nexus/internal/localization"
	"bac-nexus/internal/remote"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestDirectOnboardingLocaleAndViewportMatrix(t *testing.T) {
	for _, tt := range []struct {
		name, want, forbidden string
		localizer             localization.Localizer
	}{
		{"spanish", "CONECTAR Y GUARDAR", "CONNECT AND SAVE", localization.Spanish()},
		{"english", "CONNECT AND SAVE", "CONECTAR Y GUARDAR", localization.English()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, frame := range []struct {
				width, height int
				noColor       bool
			}{{120, 40, false}, {80, 24, true}, {40, 16, false}} {
				m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
				m.localizer, m.noColor = tt.localizer, frame.noColor
				m.beginDirectOnboarding()
				updated, _ := m.Update(tea.WindowSizeMsg{Width: frame.width, Height: frame.height})
				view := updated.(Model).View()
				if !strings.Contains(view, tt.want) || strings.Contains(view, tt.forbidden) || lipgloss.Width(view) > frame.width || lipgloss.Height(view) > frame.height {
					t.Fatalf("direct frame invalid at %dx%d: %q", frame.width, frame.height, view)
				}
				if frame.noColor && strings.Contains(view, "\x1b[") {
					t.Fatalf("NO_COLOR frame contains ANSI: %q", view)
				}
			}
		})
	}
}
