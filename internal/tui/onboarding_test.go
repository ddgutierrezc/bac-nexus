package tui

import (
	"context"
	"fmt"
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
	captures       int
	captureCode    remote.PromptCode
	captureRequest configuration.OnboardingRequest
	starts         int
	startRequest   configuration.OnboardingRequest
	startIdentity  configuration.OperationIdentity
	startCode      configuration.OnboardingCode
	cancels        []string
	result         configuration.OnboardingResult
}

func (s *onboardingOperationsStub) Capture(_ context.Context, request configuration.OnboardingRequest, _ remote.SecretPrompt, _, _ *os.File, _ string) (configuration.OperationIdentity, remote.PromptCode) {
	s.captures++
	s.captureRequest = request
	if s.captureCode != "" && s.captureCode != remote.PromptCaptured {
		return configuration.OperationIdentity{}, s.captureCode
	}
	return configuration.OperationIdentity{ID: "operation-1", Generation: 1}, remote.PromptCaptured
}

func (s *onboardingOperationsStub) StartCaptured(_ context.Context, request configuration.OnboardingRequest, identity configuration.OperationIdentity) configuration.OnboardingCode {
	s.starts++
	s.startRequest, s.startIdentity = request, identity
	if s.startCode != "" {
		return s.startCode
	}
	return configuration.OnboardingStarted
}

func (s *onboardingOperationsStub) Wait(context.Context, string) configuration.OnboardingResult {
	return s.result
}

func (s *onboardingOperationsStub) Cancel(id string) { s.cancels = append(s.cancels, id) }

func TestOnboardingExecCommandStartsOpaqueLeaseAndPropagatesLegacyRequest(t *testing.T) {
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
	request := configuration.OnboardingRequest{Name: "direct-onboarding", Host: "ibmi.example.test", Port: 22, Username: "USER"}
	command := newOnboardingExecCommand(context.Background(), remote.SecretPrompt{}, operations, request)
	command.SetStdin(input)
	command.SetStderr(output)
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
	if operations.captures != 1 || operations.starts != 1 || operations.captureRequest != request || operations.startRequest != request || operations.startIdentity != (configuration.OperationIdentity{ID: "operation-1", Generation: 1}) {
		t.Fatalf("capture/start bridge = captures:%d starts:%d capture:%#v start:%#v identity:%#v", operations.captures, operations.starts, operations.captureRequest, operations.startRequest, operations.startIdentity)
	}
	if got := command.result(); got != (onboardingPromptMsg{ID: "operation-1", Generation: 1, Code: remote.PromptCaptured}) {
		t.Fatalf("secret-free result = %#v", got)
	}
}

func TestOnboardingExecCommandDoesNotReturnLeaseWhenStartIsRejected(t *testing.T) {
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
	operations := &onboardingOperationsStub{startCode: configuration.OnboardingRejected}
	command := newOnboardingExecCommand(context.Background(), remote.SecretPrompt{}, operations, configuration.OnboardingRequest{Name: "direct-onboarding", Host: "ibmi.example.test", Port: 22, Username: "USER"})
	command.SetStdin(input)
	command.SetStderr(output)
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
	if operations.captures != 1 || operations.starts != 1 || command.result() != (onboardingPromptMsg{Code: remote.PromptUnavailable}) {
		t.Fatalf("rejected start = captures:%d starts:%d result:%#v", operations.captures, operations.starts, command.result())
	}
}

func TestOnboardingExecCommandPromptFailureStartsNoOperation(t *testing.T) {
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
	operations := &onboardingOperationsStub{captureCode: remote.PromptTerminalUnavailable}
	command := newOnboardingExecCommand(context.Background(), remote.SecretPrompt{}, operations, configuration.OnboardingRequest{Host: "ibmi.example.test", Username: "USER"})
	command.SetStdin(input)
	command.SetStdout(io.Discard)
	command.SetStderr(output)

	if err := command.Run(); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if operations.captures != 1 || operations.starts != 0 {
		t.Fatalf("operations captures/started = %d/%d, want 1/0", operations.captures, operations.starts)
	}
	if got := command.result(); got.Code != remote.PromptTerminalUnavailable || got.ID != "" || got.Generation != 0 {
		t.Fatalf("secret-free prompt result = %+v", got)
	}
}

func TestOnboardingExecCommandCapturesOnlyAtTheFixedBoundary(t *testing.T) {
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
	prompt := remote.SecretPrompt{
		Input:      input,
		Output:     output,
		IsTerminal: func(int) bool { return true },
		Read:       func(int) ([]byte, error) { return []byte("opaque-password"), nil },
	}
	command := newOnboardingExecCommand(context.Background(), prompt, operations, configuration.OnboardingRequest{Host: "ibmi.example.test", Username: "USER"})
	command.SetStdin(input)
	command.SetStderr(output)
	if err := command.Run(); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if operations.starts != 1 {
		t.Fatalf("operations started = %d, want 1", operations.starts)
	}
	got := command.result()
	if got.Code != remote.PromptCaptured || got.ID != "operation-1" || got.Generation != 1 {
		t.Fatalf("command result = %+v", got)
	}
	if strings.Contains(fmt.Sprintf("%+v", got), "opaque-password") {
		t.Fatalf("command result leaked password: %+v", got)
	}
}

func TestDirectOnboardingEscapeCancelsAndRejectsStaleResult(t *testing.T) {
	operations := &onboardingOperationsStub{result: configuration.OnboardingResult{Code: configuration.OnboardingSaved}}
	m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), operations, remote.SecretPrompt{})
	m.screen = screenDirectOnboardingRunning
	m.onboardingOperation = configuration.OperationIdentity{ID: "operation-1", Generation: 2}
	m.onboardingRunning = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if len(operations.cancels) != 1 || operations.cancels[0] != "operation-1" || m.screen != screenDirectOnboarding {
		t.Fatalf("escape did not cancel and return to form: cancels=%v screen=%d", operations.cancels, m.screen)
	}
	updated, _ = m.Update(onboardingResultMsg{ID: "operation-1", Generation: 2, Result: configuration.OnboardingResult{Code: configuration.OnboardingSaved}})
	m = updated.(Model)
	if m.screen != screenDirectOnboarding || m.onboardingCompletion.Code == configuration.OnboardingSaved {
		t.Fatalf("stale result changed cancelled form: screen=%d result=%+v", m.screen, m.onboardingCompletion)
	}
}

func TestDirectOnboardingCompletionFinalizesToReloadedProfileList(t *testing.T) {
	operations := &onboardingOperationsStub{}
	store := &profileStoreStub{profiles: []profile.Profile{testProfile("saved-profile")}}
	m := NewModelWithOnboarding(store, context.Background(), operations, remote.SecretPrompt{})
	m.screen = screenDirectOnboardingCompletion
	m.onboardingCompletion = configuration.OnboardingResult{Code: configuration.OnboardingSaved}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.screen != screenList || cmd == nil {
		t.Fatalf("Finalize screen=%d cmd=%v, want list with reload", m.screen, cmd)
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if !m.profilesLoaded || len(m.profiles) != 1 || m.profiles[0].Name != "saved-profile" {
		t.Fatalf("Finalize did not reload profile list: %+v", m.profiles)
	}
}

func TestDirectOnboardingViewContainsNoSecretAndHasSpanishAction(t *testing.T) {
	m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
	m.screen = screenDirectOnboarding
	m.directHost.SetValue("ibmi.example.test")
	m.directUsername.SetValue("USER")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := updated.(Model).View()
	if !strings.Contains(view, "CONECTAR Y GUARDAR") || strings.Contains(view, "password") || strings.Contains(view, "secret") {
		t.Fatalf("direct onboarding view is not safe Spanish-first output: %q", view)
	}
}

func TestDirectOnboardingRuntimeFramesRemainReachable(t *testing.T) {
	for _, tt := range []struct {
		name          string
		width, height int
		noColor       bool
	}{
		{"wide", 120, 40, false},
		{"standard no color", 80, 24, true},
		{"narrow", 40, 16, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
			m.screen, m.noColor = screenDirectOnboarding, tt.noColor
			updated, _ := m.Update(tea.WindowSizeMsg{Width: tt.width, Height: tt.height})
			view := updated.(Model).View()
			if !strings.Contains(view, "CONECTAR Y GUARDAR") || (tt.noColor && strings.Contains(view, "\x1b[")) {
				t.Fatalf("direct action is not reachable: %q", view)
			}
			assertProfileFrameBounds(t, view, tt.width, tt.height)
		})
	}
}

func TestDirectOnboardingHasEnglishParity(t *testing.T) {
	m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
	m.screen, m.localizer = screenDirectOnboarding, localization.English()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if view := updated.(Model).View(); !strings.Contains(view, "CONNECT AND SAVE") {
		t.Fatalf("English direct onboarding action missing: %q", view)
	}
}

func TestNoColorUsesInjectableEnvironmentAndPreservesLocaleParityAt80x24(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	home := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
	updated, _ := home.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if view := updated.(Model).View(); strings.Contains(view, "\x1b[") {
		t.Fatalf("NO_COLOR shell contains ANSI: %q", view)
	}
	for _, localizer := range []localization.Localizer{localization.Spanish(), localization.English()} {
		m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
		m.localizer = localizer
		m.beginDirectOnboarding()
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		view := updated.(Model).View()
		if strings.Contains(view, "\x1b[") {
			t.Fatalf("NO_COLOR view contains ANSI: %q", view)
		}
		assertProfileFrameBounds(t, view, 80, 24)
	}
}

func TestProfileManagementCreateOpenDeleteBackAndExitRuntime(t *testing.T) {
	store := &profileStoreStub{profiles: []profile.Profile{testProfile("saved-profile")}}
	m := NewModelWithOnboarding(store, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
	m.screen, m.profiles = screenList, store.profiles
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.screen != screenDetail {
		t.Fatalf("open screen = %d", m.screen)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.screen != screenList {
		t.Fatalf("back screen = %d", m.screen)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = updated.(Model)
	if m.screen != screenDirectOnboarding {
		t.Fatalf("create screen = %d", m.screen)
	}
	m.screen = screenList
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = updated.(Model)
	if m.screen != screenConfirm {
		t.Fatalf("delete screen = %d", m.screen)
	}
	m.confirmInput.SetValue("delete saved-profile")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("delete did not create runtime command")
	}
	updated, _ = updated.(Model).Update(cmd())
	m = updated.(Model)
	if m.screen != screenList || store.deleted != "saved-profile" {
		t.Fatalf("delete did not return to list: screen=%d deleted=%q", m.screen, store.deleted)
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("exit did not remain reachable")
	}
}

func TestOnboardingFeedbackIsClearedWhenAnotherContextStarts(t *testing.T) {
	m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
	m.screen, m.onboardingFeedback = screenDirectOnboarding, "failed"
	m.beginDirectOnboarding()
	if m.onboardingFeedback != "" {
		t.Fatalf("feedback leaked into new operation: %q", m.onboardingFeedback)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if view := updated.(Model).View(); strings.Contains(view, "failed") {
		t.Fatalf("feedback leaked into unrelated screen: %q", view)
	}
}

func TestSaveFailureCompletionStatesNotSavedRetainedCredentialAndCleanupGuidance(t *testing.T) {
	m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
	m.screen = screenDirectOnboardingCompletion
	m.onboardingCompletion = configuration.OnboardingResult{Code: configuration.OnboardingFailed, CleanupRequired: true, CredentialRetained: true}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := updated.(Model).View()
	for _, want := range []string{"no se guardó", "credencial", "limpieza", "nuevamente"} {
		if !strings.Contains(view, want) {
			t.Fatalf("save-failure guidance missing %q: %q", want, view)
		}
	}
}

func TestCreateNeverRoutesToLegacyOnboardingWithoutOperations(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := updated.(Model).screen; got != screenDirectOnboarding {
		t.Fatalf("Create screen = %d, want direct onboarding", got)
	}
}
