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

func TestModelDefaultsToSpanishAndSupportsExplicitEnglish(t *testing.T) {
	spanish := NewModel(&profileStoreStub{})
	if !strings.Contains(spanish.View(), "Crear un perfil") {
		t.Fatalf("default view is not Spanish: %q", spanish.View())
	}
	english := NewModelWithBuildInfoAndLocalizer(&profileStoreStub{}, BuildInfo{}, localization.English())
	if !strings.Contains(english.View(), "Create a profile") {
		t.Fatalf("explicit English view is not English: %q", english.View())
	}
}

func TestDirectOnboardingLocaleParity(t *testing.T) {
	for _, tt := range []struct {
		name, want string
		localizer  localization.Localizer
	}{
		{"spanish", "CONECTAR Y GUARDAR", localization.Spanish()},
		{"english", "CONNECT AND SAVE", localization.English()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
			m.localizer = tt.localizer
			m.beginDirectOnboarding()
			updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			if !strings.Contains(updated.(Model).View(), tt.want) {
				t.Fatalf("direct locale copy missing %q", tt.want)
			}
		})
	}
}

func TestLocalizedRuntimeFramesRemainBoundedAcrossViewportMatrix(t *testing.T) {
	for _, tt := range []struct {
		name      string
		localizer localization.Localizer
		want      string
		security  string
	}{{"spanish", localization.Spanish(), "Crear un perfil", "Seguridad para dev"}, {"english", localization.English(), "Create a profile", "Security for dev"}} {
		t.Run(tt.name, func(t *testing.T) {
			for _, noColor := range []bool{false, true} {
				for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {40, 16}} {
					m := NewModelWithBuildInfoAndLocalizer(&profileStoreStub{}, BuildInfo{}, tt.localizer)
					m.noColor = noColor
					updated, _ := m.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
					view := updated.(Model).View()
					if !strings.Contains(view, tt.want) || lipgloss.Width(view) > size.width || lipgloss.Height(view) > size.height {
						t.Fatalf("%s frame at %dx%d is invalid: %q", tt.name, size.width, size.height, view)
					}
				}
			}
			security := NewSecurityModelWithLocalizer(context.Background(), "dev", nil, tt.localizer)
			if !strings.Contains(security.View(), tt.security) {
				t.Fatalf("%s security view did not render locale copy", tt.name)
			}
		})
	}
}

func TestConfirmationTokensRemainProtocolValues(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.screen, m.confirm = screenConfirm, "operator"
	if !strings.Contains(m.legacyViewportContent(), "delete operator") {
		t.Fatal("profile delete token changed")
	}
	s := NewSecurityModel(nil, "operator", nil)
	s.screen, s.tofuEvidence = securityConfirmTOFU, "SHA256:proof"
	if !strings.Contains(s.viewportContent(120), "enroll SHA256:proof") {
		t.Fatal("TOFU enrollment token changed")
	}
}

func TestLocalizedDomainValuesRoundTripWithoutChangingStoredEnums(t *testing.T) {
	for _, tt := range []struct {
		name        string
		localizer   localization.Localizer
		trust, mode string
	}{
		{"spanish", localization.Spanish(), "Verificada", "Solicitar"},
		{"english", localization.English(), "Verified", "Prompt"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModelWithBuildInfoAndLocalizer(&profileStoreStub{}, BuildInfo{}, tt.localizer)
			p := testProfile("dev")
			m.beginForm(p, screenList)
			if got := m.form[5].input.Value(); got != tt.trust {
				t.Fatalf("trust display = %q", got)
			}
			if got := m.form[8].input.Value(); got != tt.mode {
				t.Fatalf("mode display = %q", got)
			}
			trust, err := m.parseTrust(m.form[5].input.Value())
			if err != nil || trust != p.HostKeyTrust {
				t.Fatalf("trust round trip = %q, %v", trust, err)
			}
			mode, err := m.parseCredentialMode(m.form[8].input.Value())
			if err != nil || mode != p.CredentialMode {
				t.Fatalf("mode round trip = %q, %v", mode, err)
			}
		})
	}
}

func TestEnglishRuntimeCoversWizardAndLegacyScreensWithoutSpanishLeakage(t *testing.T) {
	base := func() Model {
		return NewModelWithBuildInfoAndLocalizer(&profileStoreStub{}, BuildInfo{}, localization.English())
	}
	p := testProfile("dev")
	cases := []struct {
		name, want string
		model      func() Model
	}{
		{"list", "Profiles", func() Model { m := base(); m.screen, m.profiles = screenList, []profile.Profile{p}; return m }},
		{"detail", "Profile", func() Model { m := base(); m.screen, m.profiles = screenDetail, []profile.Profile{p}; return m }},
		{"form", "Profile fields", func() Model { m := base(); m.beginForm(p, screenList); return m }},
		{"delete", "Delete profile", func() Model {
			m := base()
			m.screen, m.confirm, m.confirmInput = screenConfirm, "dev", m.newConfirmationInput()
			return m
		}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.model()
			updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 16})
			m = updated.(Model)
			frames := []string{m.View()}
			for range 32 {
				updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
				m = updated.(Model)
				frames = append(frames, m.View())
			}
			all := strings.Join(frames, "\n")
			if !strings.Contains(all, tt.want) || strings.Contains(all, "Crear") || strings.Contains(all, "más") {
				t.Fatalf("English %s leaked or omitted copy: %q", tt.name, all)
			}
		})
	}
}
