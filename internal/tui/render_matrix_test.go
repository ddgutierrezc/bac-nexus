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
		if frame.width == 40 {
			updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
		}
		view := updated.(Model).View()
		if !strings.Contains(view, "SIGUIENTE") || lipgloss.Width(view) > frame.width || lipgloss.Height(view) > frame.height {
			t.Fatalf("frame invalid at %dx%d: %q", frame.width, frame.height, view)
		}
	}
}

func TestFourStepOnboardingRuntimeMatrix(t *testing.T) {
	frames := []struct{ width, height int }{{120, 40}, {80, 24}, {40, 16}}
	locales := []struct {
		name, forbidden string
		localizer       localization.Localizer
	}{
		{"spanish", "Step", localization.Spanish()},
		{"english", "Paso", localization.English()},
	}
	cases := 0
	for _, locale := range locales {
		t.Run(locale.name, func(t *testing.T) {
			for _, noColor := range []bool{false, true} {
				for _, frame := range frames {
					for step := onboardingStepName; step <= onboardingStepReview; step++ {
						t.Run(locale.name+"/step-"+string(rune('1'+step)), func(t *testing.T) {
							m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
							m.localizer, m.noColor = locale.localizer, noColor
							m.beginDirectOnboarding()
							m.profilesLoaded, m.onboardingStep = true, step
							m.directName.SetValue("profile-one")
							m.directHost.SetValue("ibmi.example.test")
							m.directUsername.SetValue("USER")
							m.directPort.SetValue("2222")
							if step == onboardingStepReview {
								m.onboardingCaptured = true
								m.onboardingOperation.ID = "do-not-render-secret"
							}
							m.focusDirectOnboarding(0)
							updated, _ := m.Update(tea.WindowSizeMsg{Width: frame.width, Height: frame.height})
							view := updated.(Model).View()
							if !strings.Contains(view, m.directOnboardingFocusText()) || strings.Contains(view, locale.forbidden) || lipgloss.Width(view) > frame.width || lipgloss.Height(view) > frame.height {
								t.Fatalf("runtime frame invalid at step %d %dx%d: %q", step+1, frame.width, frame.height, view)
							}
							if step == onboardingStepReview && strings.Contains(view, "do-not-render-secret") {
								t.Fatalf("review leaks a secret boundary value: %q", view)
							}
							if frame.width == 40 && !strings.Contains(view, "▼") {
								t.Fatalf("narrow frame does not disclose overflow: %q", view)
							}
							if noColor && strings.Contains(view, "\x1b[") {
								t.Fatalf("NO_COLOR frame contains ANSI: %q", view)
							}
							cases++
						})
					}
				}
			}
		})
	}
	if cases != 48 {
		t.Fatalf("runtime matrix executed %d cases, want 48", cases)
	}
}

func TestFourStepRuntimeNavigationPreservesValuesAndFeedbackPrecedence(t *testing.T) {
	m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
	m.localizer, m.noColor = localization.English(), true
	m.beginDirectOnboarding()
	m.profilesLoaded = true
	m.directName.SetValue("profile-one")
	m.directFocus = onboardingFocusNameNext
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.onboardingStep != onboardingStepConnection {
		t.Fatalf("next did not reach connection: %d", m.onboardingStep)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.onboardingStep != onboardingStepName || m.directName.Value() != "profile-one" {
		t.Fatalf("back did not preserve name: step=%d name=%q", m.onboardingStep, m.directName.Value())
	}
	m.onboardingFeedback, m.onboardingValidationFeedback = "operation failed safely", "name is invalid"
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := updated.(Model).View()
	if !strings.Contains(view, "operation failed safely") || strings.Contains(view, "name is invalid") || strings.Contains(view, "\x1b[") {
		t.Fatalf("operation feedback did not take precedence in NO_COLOR: %q", view)
	}

	blocked := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
	blocked.localizer, blocked.noColor = localization.English(), true
	blocked.beginDirectOnboarding()
	blocked.profilesLoaded, blocked.directFocus = true, onboardingFocusNameNext
	updated, _ = blocked.Update(tea.KeyMsg{Type: tea.KeyEnter})
	blocked = updated.(Model)
	updated, _ = blocked.Update(tea.WindowSizeMsg{Width: 40, Height: 16})
	view = updated.(Model).View()
	if blocked.onboardingStep != onboardingStepName || blocked.directFocus != onboardingFocusName || blocked.onboardingValidationFeedback == "" || !strings.Contains(view, "valid profile") || !strings.Contains(view, "▼") {
		t.Fatalf("blocked next did not reveal the first invalid field: step=%d focus=%d view=%q", blocked.onboardingStep, blocked.directFocus, view)
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
