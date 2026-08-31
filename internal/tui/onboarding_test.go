package tui

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/localization"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
	tea "github.com/charmbracelet/bubbletea"
)

type onboardingOperationsStub struct {
	starts  int
	cancels []string
	result  configuration.OnboardingResult
}

func (s *onboardingOperationsStub) StartCaptured(_ context.Context, _ configuration.OnboardingRequest, secret []byte) (configuration.OperationIdentity, configuration.OnboardingCode) {
	s.starts++
	remote.Zero(secret)
	return configuration.OperationIdentity{ID: "operation-1", Generation: 1}, configuration.OnboardingStarted
}
func (s *onboardingOperationsStub) Wait(context.Context, string) configuration.OnboardingResult {
	return s.result
}
func (s *onboardingOperationsStub) Cancel(id string) { s.cancels = append(s.cancels, id) }

func TestOnboardingPromptFailureStartsNoOperation(t *testing.T) {
	input, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	operations := &onboardingOperationsStub{}
	command := newOnboardingExecCommand(context.Background(), remote.SecretPrompt{}, operations, configuration.OnboardingRequest{Host: "ibmi.example.test", Username: "USER"})
	command.SetStdin(input)
	command.SetStdout(io.Discard)
	command.SetStderr(output)
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
	if operations.starts != 0 || command.result().Code != remote.PromptTerminalUnavailable {
		t.Fatalf("prompt failure started=%d result=%+v", operations.starts, command.result())
	}
}

func TestDirectOnboardingCancelRejectsStaleAndFinalizeReloads(t *testing.T) {
	operations := &onboardingOperationsStub{}
	m := NewModelWithOnboarding(&profileStoreStub{profiles: []profile.Profile{testProfile("saved-profile")}}, context.Background(), operations, remote.SecretPrompt{})
	m.screen, m.onboardingOperation, m.onboardingRunning = screenDirectOnboardingRunning, configuration.OperationIdentity{ID: "operation-1", Generation: 2}, true
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.screen != screenDirectOnboarding || len(operations.cancels) != 1 {
		t.Fatalf("cancel state=%d cancels=%v", m.screen, operations.cancels)
	}
	updated, _ = m.Update(onboardingResultMsg{ID: "operation-1", Generation: 2, Result: configuration.OnboardingResult{Code: configuration.OnboardingSaved}})
	m = updated.(Model)
	if m.onboardingCompletion.Code == configuration.OnboardingSaved {
		t.Fatal("stale result was accepted")
	}
	m.screen, m.onboardingCompletion = screenDirectOnboardingCompletion, configuration.OnboardingResult{Code: configuration.OnboardingSaved}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.screen != screenList || cmd == nil {
		t.Fatalf("finalize state=%d cmd=%v", m.screen, cmd)
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if !m.profilesLoaded || len(m.profiles) != 1 {
		t.Fatalf("finalize did not reload: %+v", m.profiles)
	}
}

func TestDirectRoutesHaveAuthorityAndRuntimeLocaleMatrix(t *testing.T) {
	for _, tt := range []struct {
		name, want    string
		localizer     localization.Localizer
		width, height int
		noColor       bool
	}{
		{"spanish-wide", "CONECTAR Y GUARDAR", localization.Spanish(), 120, 40, false},
		{"spanish-no-color", "CONECTAR Y GUARDAR", localization.Spanish(), 80, 24, true},
		{"english-narrow", "CONNECT AND SAVE", localization.English(), 40, 16, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
			m.localizer, m.noColor = tt.localizer, tt.noColor
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = updated.(Model)
			if m.screen != screenDirectOnboarding {
				t.Fatalf("Create routed to %d", m.screen)
			}
			updated, _ = m.Update(tea.WindowSizeMsg{Width: tt.width, Height: tt.height})
			view := updated.(Model).View()
			if !strings.Contains(view, tt.want) || (tt.noColor && strings.Contains(view, "\x1b[")) {
				t.Fatalf("frame=%q", view)
			}
			assertProfileFrameBounds(t, view, tt.width, tt.height)
		})
	}
	m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
	m.screen = screenList
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if updated.(Model).screen != screenDirectOnboarding {
		t.Fatal("list New did not route directly")
	}
}
