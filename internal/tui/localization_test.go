package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"bac-nexus/internal/localization"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestDirectOnboardingLocaleAndViewportMatrix(t *testing.T) {
	for _, tt := range []struct {
		name, want, forbidden string
		localizer             localization.Localizer
	}{
		{"spanish", "SIGUIENTE", "NEXT", localization.Spanish()},
		{"english", "NEXT", "SIGUIENTE", localization.English()},
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
				if frame.width == 40 {
					updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
				}
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

func TestDirectOnboardingLocalizedFieldValidationRendersAtRuntime(t *testing.T) {
	for _, tt := range []struct {
		name, host, username, want string
		localizer                  localization.Localizer
	}{
		{"spanish host", "host:22", "USER", "host válido", localization.Spanish()},
		{"spanish username", "ibmi.example.test", "bad user", "usuario IBM i válido", localization.Spanish()},
		{"english host", "host:22", "USER", "valid host", localization.English()},
		{"english username", "ibmi.example.test", "bad user", "valid IBM i username", localization.English()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
			m.localizer, m.noColor = tt.localizer, true
			m.beginDirectOnboarding()
			m.profilesLoaded, m.onboardingStep = true, onboardingStepConnection
			m.directName.SetValue("new-profile")
			m.directHost.SetValue(tt.host)
			m.directUsername.SetValue(tt.username)
			m.directPort.SetValue("22")
			m.directFocus = onboardingFocusConnectionNext
			updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
			view := updated.(Model).View()
			if !strings.Contains(view, tt.want) || strings.Contains(view, "\x1b[") {
				t.Fatalf("localized validation did not render in NO_COLOR: %q", view)
			}
		})
	}
}

func TestEditOperationFeedbackStaysSanitizedAndPrecedesLocalValidation(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.localizer, m.noColor = localization.English(), true
	original := testProfile("edit-profile")
	original.SchemaVersion = profile.SchemaVersionV3
	m.beginForm(original, screenDetail)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)
	updated, _ = m.Update(operationMsg{code: operationProfileUpdated, err: errors.New("store\n details")})
	m = updated.(Model)
	m.formValidation = &profileValidation{FieldID: profileValidationFieldName, MessageID: "profile.validation.name", Cause: errors.New("invalid")}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = updated.(Model)
	for range len(m.form) {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(Model)
	}
	view := m.View()
	if m.formValidation != nil || m.formOperationFeedback != "store details" || !strings.Contains(view, "store details") || strings.Contains(view, "\x1b[") {
		t.Fatalf("operation feedback lost precedence or sanitization: validation=%#v feedback=%q view=%q", m.formValidation, m.formOperationFeedback, view)
	}
}
