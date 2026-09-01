package tui

import (
	"context"
	"strings"
	"testing"

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
