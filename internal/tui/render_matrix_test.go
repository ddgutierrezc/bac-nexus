package tui

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var ansiEscape = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

// TestNonWizardRenderMatrix protects the shared shell and the persistent
// legacy/security viewports at the supported terminal sizes.
func TestNonWizardRenderMatrix(t *testing.T) {
	previous := lipgloss.ColorProfile()
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
	profiles := make([]profile.Profile, 14)
	for i := range profiles {
		profiles[i] = testProfile(fmt.Sprintf("perfil-%02d", i))
	}
	for _, noColor := range []bool{false, true} {
		if noColor {
			lipgloss.SetColorProfile(termenv.Ascii)
		} else {
			lipgloss.SetColorProfile(termenv.TrueColor)
		}
		for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {40, 16}} {
			for _, screenCase := range []struct {
				name  string
				model func() Model
			}{
				{"direct-onboarding", func() Model {
					m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
					m.beginDirectOnboarding()
					return m
				}},
				{"home", func() Model { return NewModel(&profileStoreStub{profiles: profiles}) }},
				{"list", func() Model {
					m := NewModel(&profileStoreStub{profiles: profiles})
					m.screen, m.profiles, m.selected = screenList, profiles, len(profiles)-1
					return m
				}},
				{"detail", func() Model {
					m := NewModel(&profileStoreStub{profiles: profiles})
					m.screen, m.profiles = screenDetail, profiles
					return m
				}},
				{"form", func() Model {
					m := NewModel(&profileStoreStub{})
					m.beginForm(testProfile("formulario"), screenList)
					return m
				}},
				{"confirm", func() Model {
					m := NewModel(&profileStoreStub{profiles: profiles})
					m.screen, m.confirm, m.confirmInput = screenConfirm, "perfil-00", textinput.New()
					return m
				}},
				{"security", func() Model {
					m := NewModelWithSecurity(&profileStoreStub{profiles: profiles}, t.Context(), nil)
					m.screen, m.security.screen = screenSecurity, securityTrust
					return m
				}},
			} {
				t.Run(fmt.Sprintf("%s/%dx%d/no-color=%v", screenCase.name, size.width, size.height, noColor), func(t *testing.T) {
					m := screenCase.model()
					m.noColor = noColor
					updated, _ := m.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
					m = updated.(Model)
					view := m.View()
					if lipgloss.Width(view) > size.width || lipgloss.Height(view) > size.height {
						t.Fatalf("render bounds = %dx%d", lipgloss.Width(view), lipgloss.Height(view))
					}
					for _, line := range strings.Split(view, "\n") {
						if lipgloss.Width(line) > size.width {
							t.Fatalf("horizontal overflow %d: %q", lipgloss.Width(line), line)
						}
					}
					if noColor && strings.Contains(view, "\x1b[") {
						t.Fatal("NO_COLOR render contains ANSI")
					}
					if !noColor && !strings.Contains(view, "\x1b[") {
						t.Fatal("true-color render lacks ANSI")
					}
					if screenCase.name == "list" && size.height == 16 {
						if !strings.Contains(view, "▲ más") {
							t.Fatalf("hidden legacy list lacks upward indicator:\n%s", view)
						}
						updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
						m = updated.(Model)
						if !strings.Contains(m.View(), "▼ más") {
							t.Fatalf("manual legacy navigation did not disclose downward content:\n%s", m.View())
						}
					}
				})
			}
		}
	}
}

func TestHomeFunctionalFeedbackIsLosslessAcrossRenderMatrix(t *testing.T) {
	message := "Estado funcional extenso con espacios que debe permanecer visible sin perder ninguna palabra importante"
	failure := "error funcional extenso con espacios que debe permanecer visible sin perder ninguna palabra importante"
	for _, noColor := range []bool{false, true} {
		for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {40, 16}} {
			t.Run(fmt.Sprintf("%dx%d/no-color=%v", size.width, size.height, noColor), func(t *testing.T) {
				setTestColorProfile(t, noColor)
				m := NewModel(&profileStoreStub{})
				m.noColor, m.status, m.err = noColor, message, errors.New(failure)
				updated, _ := m.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
				view := updated.(Model).View()
				assertRenderCell(t, view, size.width, size.height, noColor)
				if !strings.Contains(alphaNumericOnly(view), alphaNumericOnly(message)) || !strings.Contains(alphaNumericOnly(view), alphaNumericOnly(failure)) {
					t.Fatalf("Home lost functional feedback at %dx%d:\n%s", size.width, size.height, view)
				}
			})
		}
	}
}

func TestCanonicalWizardFramesHaveEightStepsWithoutJava(t *testing.T) {
	for _, step := range []struct {
		name   string
		screen screen
		label  string
		proof  bool
	}{
		{"mapepire", screenProfileMapepire, "Step 4 of 8", false},
		{"credentials", screenProfileCredentials, "Step 5 of 8", false},
		{"review", screenProfileReview, "Step 6 of 8", false},
		{"proof", screenProfileStep8Action, "Step 7 of 8", true},
		{"completion", screenProfileCompletion, "Step 8 of 8", false},
	} {
		t.Run(step.name, func(t *testing.T) {
			m := NewModel(&profileStoreStub{})
			m.screen, m.noColor = step.screen, true
			m.profileProof.enabled = step.proof
			updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
			view := updated.(Model).View()
			if !strings.Contains(view, step.label) || strings.Contains(view, "Step 5 of 8 — Java") || strings.Contains(view, "Paso 5 de 8 — Java") {
				t.Fatalf("wizard frame = %q, want %q without a Java step", view, step.label)
			}
		})
	}
}

func TestSecurityViewportReachabilityAndIndicatorsAcrossRenderMatrix(t *testing.T) {
	for _, noColor := range []bool{false, true} {
		for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {40, 16}} {
			t.Run(fmt.Sprintf("%dx%d/no-color=%v", size.width, size.height, noColor), func(t *testing.T) {
				setTestColorProfile(t, noColor)
				m := NewModelWithSecurity(&profileStoreStub{}, t.Context(), nil)
				m.noColor, m.screen = noColor, screenSecurity
				m.security.screen = securityConfirmTOFU
				m.security.profile = "perfil con espacios"
				m.security.tofuEvidence = numberedSemanticText("evidencia observada con espacios", 500)
				m.security.confirm = newConfirmationInput()
				updated, _ := m.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
				m = updated.(Model)
				pages := collectModelViewportPages(t, m, tea.KeyPgDown)
				assertRenderCell(t, pages[0], size.width, size.height, noColor)
				all := alphaNumericOnly(strings.Join(pages, "\n"))
				for _, want := range []string{"ADVERTENCIA: la inspección remota es TOFU no verificada", "Escribe enroll", "exactamente y luego presiona enter."} {
					if !strings.Contains(all, alphaNumericOnly(want)) {
						t.Fatalf("security viewport did not expose %q", want)
					}
				}
				if m.security.viewport.TotalLineCount() > m.security.viewport.Height {
					if len(pages) < 3 {
						t.Fatalf("security fixture must produce top, middle, and bottom pages; got %d", len(pages))
					}
					if !strings.Contains(pages[0], "▼ más") || strings.Contains(pages[0], "▲ más") {
						t.Fatalf("security top indicator incorrect:\n%s", pages[0])
					}
					if !strings.Contains(pages[1], "▲ más") || !strings.Contains(pages[1], "▼ más") {
						t.Fatalf("security middle indicator incorrect:\n%s", pages[1])
					}
					if !strings.Contains(pages[len(pages)-1], "▲ más") || strings.Contains(pages[len(pages)-1], "▼ más") {
						t.Fatalf("security bottom indicator incorrect:\n%s", pages[len(pages)-1])
					}
					for range 1024 {
						updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
						m = updated.(Model)
					}
					updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
					if !strings.Contains(updated.(Model).View(), "▲ más") {
						t.Fatal("security PgUp from bottom did not return to a disclosed prior page")
					}
				} else if strings.Contains(pages[0], "más") {
					t.Fatalf("security fitting content has false indicator:\n%s", pages[0])
				}
				// Spaces remain input, not navigation, on the confirmation control.
				updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
				if !strings.Contains(updated.(Model).security.confirm.Value(), " ") {
					t.Fatal("security confirmation input rejected a space")
				}
			})
		}
	}
}

func TestLegacyViewportFocusAndReachability(t *testing.T) {
	profiles := make([]profile.Profile, 18)
	for i := range profiles {
		profiles[i] = testProfile(fmt.Sprintf("perfil-%02d-con-nombre-extenso", i))
	}
	t.Run("list selection follows first middle and last", func(t *testing.T) {
		m := NewModel(&profileStoreStub{profiles: profiles})
		m.noColor, m.screen, m.profiles = true, screenList, profiles
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 16})
		m = updated.(Model)
		for _, selected := range []int{0, len(profiles) / 2, len(profiles) - 1} {
			for m.selected < selected {
				updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
				m = updated.(Model)
			}
			if !strings.Contains(m.View(), profiles[selected].Name) {
				t.Fatalf("selected list item %d was not revealed:\n%s", selected, m.View())
			}
		}
		if !strings.Contains(m.View(), "▲ más") {
			t.Fatal("last list item should disclose hidden content above")
		}
	})
	t.Run("detail form and confirmation retain complete functional copy", func(t *testing.T) {
		long := testProfile("perfil-confirmacion-extenso")
		long.Host, long.Username = strings.Repeat("host-con-espacios-y-detalle-", 4), strings.Repeat("usuario-extenso-", 5)
		for _, screenCase := range []struct {
			name  string
			new   func() Model
			focus func(*Model)
		}{
			{"detail", func() Model {
				m := NewModel(&profileStoreStub{profiles: []profile.Profile{long}})
				m.noColor, m.screen, m.profiles, m.status, m.err = true, screenDetail, []profile.Profile{long}, strings.Repeat("estado funcional ", 8), errors.New(strings.Repeat("error funcional ", 8))
				return m
			}, nil},
			{"form", func() Model {
				m := NewModel(&profileStoreStub{})
				m.noColor = true
				m.beginForm(long, screenList)
				for i := range m.form {
					m.form[i].input.SetValue(strings.Repeat("valor funcional largo ", 4))
				}
				return m
			}, func(m *Model) {
				for range len(m.form) - 1 {
					updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
					*m = updated.(Model)
				}
			}},
			{"confirm", func() Model {
				m := NewModel(&profileStoreStub{})
				m.noColor, m.screen, m.confirm, m.confirmInput = true, screenConfirm, strings.Repeat("perfil con espacios ", 6), newConfirmationInput()
				return m
			}, nil},
		} {
			t.Run(screenCase.name, func(t *testing.T) {
				m := screenCase.new()
				updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 16})
				m = updated.(Model)
				if screenCase.focus != nil {
					screenCase.focus(&m)
				}
				if screenCase.name == "form" {
					for _, index := range []int{0, len(m.form) / 2, len(m.form) - 1} {
						for m.focusIndex() != index {
							updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
							m = updated.(Model)
						}
						if !strings.Contains(m.View(), m.form[index].label) {
							t.Fatalf("focused form field %q was not revealed:\n%s", m.form[index].label, m.View())
						}
					}
				}
				pages := collectModelViewportPages(t, m, tea.KeyPgDown)
				if !strings.Contains(pages[0], "▼ más") && len(pages) > 1 {
					t.Fatalf("%s did not disclose hidden content", screenCase.name)
				}
				if len(pages) > 1 && (!strings.Contains(pages[len(pages)-1], "▲ más") || strings.Contains(pages[len(pages)-1], "▼ más")) {
					t.Fatalf("%s bottom indicator incorrect", screenCase.name)
				}
				if screenCase.name == "confirm" {
					updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
					m = updated.(Model)
					if !strings.Contains(m.confirmInput.Value(), " ") {
						t.Fatal("legacy confirmation rejected a space")
					}
				}
			})
		}
	})
}

func collectModelViewportPages(t *testing.T, m Model, key tea.KeyType) []string {
	t.Helper()
	pages := []string{m.View()}
	for range 1024 {
		updated, _ := m.Update(tea.KeyMsg{Type: key})
		m = updated.(Model)
		view := m.View()
		if view == pages[len(pages)-1] {
			break
		}
		pages = append(pages, view)
	}
	return pages
}

func assertRenderCell(t *testing.T, view string, width, height int, noColor bool) {
	t.Helper()
	if lipgloss.Width(view) > width || lipgloss.Height(view) > height {
		t.Fatalf("render bounds = %dx%d", lipgloss.Width(view), lipgloss.Height(view))
	}
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > width {
			t.Fatalf("horizontal overflow %d: %q", lipgloss.Width(line), line)
		}
	}
	if noColor && strings.Contains(view, "\x1b[") {
		t.Fatal("NO_COLOR render contains ANSI")
	}
	if !noColor && !strings.Contains(view, "\x1b[") {
		t.Fatal("true-color render lacks ANSI")
	}
}

func setTestColorProfile(t *testing.T, noColor bool) {
	t.Helper()
	previous := lipgloss.ColorProfile()
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
	if noColor {
		lipgloss.SetColorProfile(termenv.Ascii)
		return
	}
	lipgloss.SetColorProfile(termenv.TrueColor)
}

func TestLegacyRuntimeReachabilityAcrossFullMatrix(t *testing.T) {
	for _, noColor := range []bool{false, true} {
		for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {40, 16}} {
			for _, screenCase := range legacyOverflowCases() {
				t.Run(fmt.Sprintf("%s/%dx%d/no-color=%v", screenCase.name, size.width, size.height, noColor), func(t *testing.T) {
					setTestColorProfile(t, noColor)
					m := screenCase.new()
					m.noColor = noColor
					updated, _ := m.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
					m = updated.(Model)
					if screenCase.exercise != nil {
						screenCase.exercise(t, &m)
					}
					for range 1024 {
						updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
						m = updated.(Model)
					}
					pages := collectModelViewportPages(t, m, tea.KeyPgDown)
					assertRenderCell(t, pages[0], size.width, size.height, noColor)
					if len(pages) < 3 {
						t.Fatalf("forced %s fixture must yield top/middle/bottom pages, got %d", screenCase.name, len(pages))
					}
					if !strings.Contains(pages[0], "▼ más") || strings.Contains(pages[0], "▲ más") {
						t.Fatalf("%s top indicator incorrect", screenCase.name)
					}
					if !strings.Contains(pages[1], "▲ más") || !strings.Contains(pages[1], "▼ más") {
						t.Fatalf("%s middle indicator incorrect", screenCase.name)
					}
					if !strings.Contains(pages[len(pages)-1], "▲ más") || strings.Contains(pages[len(pages)-1], "▼ más") {
						t.Fatalf("%s bottom indicator incorrect after %d pages:\n%s", screenCase.name, len(pages), pages[len(pages)-1])
					}
					all := normalizedView(strings.Join(pages, "\n"))
					for _, want := range screenCase.copy {
						if !strings.Contains(all, normalizedView(want)) {
							t.Fatalf("%s lost functional copy %q", screenCase.name, want)
						}
					}
				})
			}
		}
	}
}

type legacyOverflowCase struct {
	name     string
	new      func() Model
	exercise func(*testing.T, *Model)
	copy     []string
}

func legacyOverflowCases() []legacyOverflowCase {
	profiles := make([]profile.Profile, 80)
	for i := range profiles {
		profiles[i] = testProfile(fmt.Sprintf("semantic-long-profile-%03d", i))
	}
	long := testProfile("perfil-detalle-extenso")
	long.Host, long.Username = numberedSemanticText("host funcional extenso con espacios", 100), numberedSemanticText("usuario funcional extenso con espacios", 100)
	return []legacyOverflowCase{
		{"list", func() Model {
			m := NewModel(&profileStoreStub{profiles: profiles})
			m.screen, m.profiles = screenList, profiles
			return m
		}, func(t *testing.T, m *Model) {
			for _, target := range []int{0, len(profiles) / 2, len(profiles) - 1} {
				for m.selected < target {
					updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
					*m = updated.(Model)
				}
				if !strings.Contains(m.View(), profiles[target].Name) {
					t.Fatalf("list selection %d not revealed", target)
				}
			}
		}, []string{"Perfiles", profiles[0].Name, profiles[len(profiles)-1].Name, "n nuevo enter inspeccionar d eliminar b inicio q salir"}},
		{"detail", func() Model {
			m := NewModel(&profileStoreStub{profiles: []profile.Profile{long}})
			m.screen, m.profiles, m.status, m.err = screenDetail, []profile.Profile{long}, numberedSemanticText("estado funcional extenso", 100), errors.New(numberedSemanticText("error funcional extenso", 100))
			return m
		}, nil, []string{"Perfil:", "Host:", "Usuario:", "Confianza:", "estado funcional extenso", "error funcional extenso", "e editar d eliminar s seguridad b volver"}},
		{"form", func() Model {
			m := NewModel(&profileStoreStub{})
			m.beginForm(long, screenList)
			for i := range m.form {
				m.form[i].input.SetValue(numberedSemanticText("valor funcional extenso with spaces", 100))
			}
			return m
		}, func(t *testing.T, m *Model) {
			for _, target := range []int{0, len(m.form) / 2, len(m.form) - 1} {
				for m.focusIndex() != target {
					updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
					*m = updated.(Model)
				}
				if !strings.Contains(m.View(), m.form[target].label) {
					t.Fatalf("form focus %q not revealed", m.form[target].label)
				}
			}
		}, []string{"Campos del perfil", "Nombre:", "Fingerprint:", "Modo de credencial:", "enter guardar esc cancelar"}},
		{"confirm", func() Model {
			m := NewModel(&profileStoreStub{})
			m.screen, m.confirm, m.confirmInput = screenConfirm, numberedSemanticText("profile with semantic spaces", 500), newConfirmationInput()
			return m
		}, func(t *testing.T, m *Model) {
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
			*m = updated.(Model)
			if !strings.Contains(m.confirmInput.Value(), " ") {
				t.Fatal("confirmation space was not typed")
			}
		}, []string{"Eliminar el perfil", "Esto conserva una copia de respaldo recuperable.", "Escribe delete", "exactamente y luego presiona enter.", "n/esc cancelar"}},
	}
}

func normalizedView(text string) string {
	return alphaNumericOnly(ansiEscape.ReplaceAllString(text, ""))
}

func numberedSemanticText(label string, count int) string {
	parts := make([]string, count)
	for i := range parts {
		parts[i] = fmt.Sprintf("%s %03d", label, i)
	}
	return strings.Join(parts, " ")
}
