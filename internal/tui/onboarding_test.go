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
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type onboardingOperationsStub struct {
	captures    int
	captureCode remote.PromptCode
	starts      int
	cancels     []string
	revokes     []configuration.OperationIdentity
	result      configuration.OnboardingResult
}

func (s *onboardingOperationsStub) Capture(_ context.Context, _ configuration.OnboardingRequest, _ remote.SecretPrompt, _, _ *os.File, _ string) (configuration.OperationIdentity, remote.PromptCode) {
	s.captures++
	if s.captureCode != "" && s.captureCode != remote.PromptCaptured {
		return configuration.OperationIdentity{}, s.captureCode
	}
	return configuration.OperationIdentity{ID: "operation-1", Generation: 1}, remote.PromptCaptured
}

func (s *onboardingOperationsStub) StartCaptured(_ context.Context, _ configuration.OnboardingRequest, _ configuration.OperationIdentity) configuration.OnboardingCode {
	s.starts++
	return configuration.OnboardingStarted
}

func (s *onboardingOperationsStub) Wait(context.Context, string) configuration.OnboardingResult {
	return s.result
}

func (s *onboardingOperationsStub) Cancel(id string) { s.cancels = append(s.cancels, id) }
func (s *onboardingOperationsStub) Revoke(id configuration.OperationIdentity) {
	s.revokes = append(s.revokes, id)
}

func TestOnboardingBackRevokesCapturedLease(t *testing.T) {
	operations := &onboardingOperationsStub{}
	m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), operations, remote.SecretPrompt{})
	m.screen, m.onboardingStep, m.onboardingCaptured = screenDirectOnboarding, onboardingStepReview, true
	m.onboardingOperation, m.directFocus = configuration.OperationIdentity{ID: "lease-1", Generation: 2}, onboardingFocusReviewBack
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if len(operations.revokes) != 1 || operations.revokes[0] != (configuration.OperationIdentity{ID: "lease-1", Generation: 2}) || m.onboardingCaptured {
		t.Fatalf("Back revoke/capture state = %#v/%t", operations.revokes, m.onboardingCaptured)
	}
}

func TestOnboardingExecCommandDelegatesCaptureWithoutSecretArgument(t *testing.T) {
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
	command := newOnboardingExecCommand(context.Background(), remote.SecretPrompt{}, operations, configuration.OnboardingRequest{Name: "test-profile", Host: "ibmi.example.test", Port: 2222, Username: "USER"})
	command.SetStdin(input)
	command.SetStderr(output)
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
	if operations.captures != 1 || operations.starts != 0 {
		t.Fatalf("capture/start calls = %d/%d, want 1/0", operations.captures, operations.starts)
	}
	if got := command.result(); got.ID != "operation-1" || got.Generation != 1 || got.Code != remote.PromptCaptured {
		t.Fatalf("secret-free result = %#v", got)
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
		t.Fatalf("operations capture/start = %d/%d, want 1/0", operations.captures, operations.starts)
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
	if operations.captures != 1 || operations.starts != 0 {
		t.Fatalf("operations capture/start = %d/%d, want 1/0", operations.captures, operations.starts)
	}
	got := command.result()
	if got.Code != remote.PromptCaptured || got.ID != "operation-1" || got.Generation != 1 {
		t.Fatalf("command result = %+v", got)
	}
	if strings.Contains(fmt.Sprintf("%+v", got), "opaque-password") {
		t.Fatalf("command result leaked password: %+v", got)
	}
}

func TestOnboardingExecCommandReturnsRetryableSecretFreeCaptureStatuses(t *testing.T) {
	input, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.CreateTemp(t.TempDir(), "prompt")
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	for _, tt := range []struct {
		name string
		ctx  context.Context
		read func(int) ([]byte, error)
		want remote.PromptCode
	}{
		{"eof", context.Background(), func(int) ([]byte, error) { return nil, io.EOF }, remote.PromptEOF},
		{"cancelled", canceledContext(), func(int) ([]byte, error) { return nil, context.Canceled }, remote.PromptCancelled},
	} {
		t.Run(tt.name, func(t *testing.T) {
			operations := &onboardingOperationsStub{captureCode: tt.want}
			prompt := remote.SecretPrompt{Input: input, Output: output, IsTerminal: func(int) bool { return true }, Read: tt.read}
			command := newOnboardingExecCommand(tt.ctx, prompt, operations, configuration.OnboardingRequest{Host: "ibmi.example.test", Username: "USER"}, "localized prompt")
			command.SetStdin(input)
			command.SetStderr(output)
			if err := command.Run(); err != nil || command.result().Code != tt.want || operations.starts != 0 {
				t.Fatalf("Run err=%v result=%+v starts=%d", err, command.result(), operations.starts)
			}
		})
	}
}

func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestDirectOnboardingEscapeCancelsAndRejectsStaleResult(t *testing.T) {
	operations := &onboardingOperationsStub{result: configuration.OnboardingResult{Code: configuration.OnboardingSaved}}
	m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), operations, remote.SecretPrompt{})
	m.screen = screenDirectOnboardingRunning
	m.onboardingOperation = configuration.OperationIdentity{ID: "operation-1", Generation: 2}
	m.onboardingRunning = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if len(operations.revokes) != 1 || operations.revokes[0].ID != "operation-1" || m.screen != screenDirectOnboarding {
		t.Fatalf("escape did not revoke and return to form: revokes=%v screen=%d", operations.revokes, m.screen)
	}
	updated, _ = m.Update(onboardingResultMsg{ID: "operation-1", Generation: 2, Result: configuration.OnboardingResult{Code: configuration.OnboardingSaved}})
	m = updated.(Model)
	if m.screen != screenDirectOnboarding || m.onboardingCompletion.Code == configuration.OnboardingSaved {
		t.Fatalf("stale result changed cancelled form: screen=%d result=%+v", m.screen, m.onboardingCompletion)
	}
}

func TestFourStepOnboardingGuardsPreserveValuesAndKeepBlockedActionsFocusable(t *testing.T) {
	m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
	m.profiles, m.profilesLoaded = []profile.Profile{testProfile("existing")}, true
	m.beginDirectOnboarding()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.onboardingStep != onboardingStepName || m.directFocus != onboardingFocusName || m.onboardingValidationFeedback == "" {
		t.Fatalf("empty name advanced or lost blocked focus: step=%d focus=%d feedback=%q", m.onboardingStep, m.directFocus, m.onboardingValidationFeedback)
	}
	m.directName.SetValue("new-profile")
	m.directFocus = onboardingFocusNameNext
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.onboardingStep != onboardingStepConnection {
		t.Fatalf("valid name step = %d, want connection", m.onboardingStep)
	}
	m.directHost.SetValue("ibmi.example.test")
	m.directUsername.SetValue("USER")
	m.directPort.SetValue("2222")
	m.directFocus = onboardingFocusConnectionNext
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.onboardingStep != onboardingStepCredentials {
		t.Fatalf("valid endpoint step = %d, want credentials", m.onboardingStep)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.onboardingStep != onboardingStepConnection || m.directName.Value() != "new-profile" || m.directPort.Value() != "2222" {
		t.Fatalf("Back did not preserve safe values: step=%d name=%q port=%q", m.onboardingStep, m.directName.Value(), m.directPort.Value())
	}
}

func TestSecurePasswordActionUsesLocalizedPromptAndReturnsToReview(t *testing.T) {
	operations := &onboardingOperationsStub{}
	m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), operations, remote.SecretPrompt{})
	m.localizer = localization.English()
	m.beginDirectOnboarding()
	m.onboardingStep = onboardingStepCredentials
	m.directName.SetValue("new-profile")
	m.directHost.SetValue("ibmi.example.test")
	m.directUsername.SetValue("USER")
	m.directPort.SetValue("22")
	m.directFocus = onboardingFocusCapture
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || updated.(Model).onboardingRunning {
		t.Fatal("secure capture must issue tea.Exec without starting a backend operation")
	}
	if got := updated.(Model).text("onboarding.password_prompt", nil); got != "IBM i password: " {
		t.Fatalf("localized terminal prompt = %q", got)
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
	if !strings.Contains(view, "SIGUIENTE") || strings.Contains(view, "password") || strings.Contains(view, "secret") {
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
			m.beginDirectOnboarding()
			m.noColor = tt.noColor
			updated, _ := m.Update(tea.WindowSizeMsg{Width: tt.width, Height: tt.height})
			if tt.width == 40 {
				updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
			}
			view := updated.(Model).View()
			if !strings.Contains(view, "SIGUIENTE") || (tt.noColor && strings.Contains(view, "\x1b[")) {
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
	if view := updated.(Model).View(); !strings.Contains(view, "NEXT") {
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

func TestDirectOnboardingValidationBlocksTheFirstInvalidField(t *testing.T) {
	for _, tt := range []struct {
		name, host, username, want string
		focus                      onboardingFocus
	}{
		{"host before username", "host:22", "bad user", "host válido", onboardingFocusHost},
		{"username after valid host", "ibmi.example.test", "bad user", "usuario IBM i válido", onboardingFocusUsername},
	} {
		t.Run(tt.name, func(t *testing.T) {
			operations := &onboardingOperationsStub{}
			m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), operations, remote.SecretPrompt{})
			m.beginDirectOnboarding()
			m.profilesLoaded, m.onboardingStep = true, onboardingStepConnection
			m.directName.SetValue("new-profile")
			m.directHost.SetValue(tt.host)
			m.directUsername.SetValue(tt.username)
			m.directPort.SetValue("22")
			m.directFocus = onboardingFocusConnectionNext

			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			got := updated.(Model)
			if cmd != nil || operations.starts != 0 || got.directFocus != tt.focus || !strings.Contains(got.directOnboardingFeedback(), tt.want) {
				t.Fatalf("validation result cmd=%v starts=%d focus=%d feedback=%q", cmd, operations.starts, got.directFocus, got.directOnboardingFeedback())
			}
		})
	}
}

func TestDirectOnboardingValidationClearsOnlyTheEditedFieldAndDefersToOperationFeedback(t *testing.T) {
	m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
	m.beginDirectOnboarding()
	m.onboardingValidationFeedback = "host validation"
	m.onboardingFeedback = "operation failure"
	m.directFocus = onboardingFocusUsername

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("U")})
	got := updated.(Model)
	if got.onboardingValidationFeedback != "host validation" || got.directOnboardingFeedback() != "operation failure" {
		t.Fatalf("username edit cleared unrelated validation or changed feedback precedence: validation=%q feedback=%q", got.onboardingValidationFeedback, got.directOnboardingFeedback())
	}
}

func TestDirectOnboardingEditingInvalidFieldClearsItsOwnValidation(t *testing.T) {
	m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
	m.beginDirectOnboarding()
	m.onboardingValidationFeedback = "host validation"
	m.onboardingValidationFocus = onboardingFocusHost
	m.directFocus = onboardingFocusHost

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	got := updated.(Model)
	if got.onboardingValidationFeedback != "" {
		t.Fatalf("editing invalid host retained local validation: %q", got.onboardingValidationFeedback)
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

func TestDirectOnboardingCompletionUsesTruthfulDiagnosticMarkers(t *testing.T) {
	const reference = "ONB-0123456789abcdef0123456789abcdef"
	for _, tt := range []struct {
		name        string
		result      configuration.OnboardingResult
		must, never []string
	}{
		{"saved", configuration.OnboardingResult{Code: configuration.OnboardingSaved}, []string{"[OK]", "Perfil conectado y guardado.", "FINALIZAR"}, []string{"[ERR]"}},
		{"ordinary failure", configuration.OnboardingResult{Code: configuration.OnboardingFailed, Diagnostic: configuration.OnboardingDiagnostic{Phase: configuration.OnboardingPhaseAuthenticatedProof, Class: configuration.OnboardingFailureClass(configuration.ResultAuthenticationFailed)}}, []string{"[ERR]", "Prueba autenticada", "authentication_failed"}, []string{"[OK]"}},
		{"cleanup takes precedence", configuration.OnboardingResult{Code: configuration.OnboardingFailed, CleanupRequired: true, Diagnostic: configuration.OnboardingDiagnostic{Phase: configuration.OnboardingPhaseSave, Class: configuration.OnboardingClassSaveFailure, Reference: reference, Written: true}}, []string{"[ERR]", "credencial", "limpieza", "Guardado", "save_failure", reference}, []string{"[OK]"}},
		{"diagnostic unavailable", configuration.OnboardingResult{Code: configuration.OnboardingFailed, Diagnostic: configuration.OnboardingDiagnostic{Phase: configuration.OnboardingPhaseCommit, Class: configuration.OnboardingClassCommitFailure}}, []string{"[ERR]", "Confirmación", "commit_failure", "no están disponibles"}, []string{"[OK]", reference}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
			m.screen, m.onboardingCompletion, m.noColor = screenDirectOnboardingCompletion, tt.result, true
			updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
			view := updated.(Model).View()
			for _, want := range tt.must {
				if !strings.Contains(view, want) {
					t.Fatalf("completion missing %q: %q", want, view)
				}
			}
			for _, forbidden := range tt.never {
				if strings.Contains(view, forbidden) {
					t.Fatalf("completion contains contradictory marker %q: %q", forbidden, view)
				}
			}
		})
	}
}

func TestDirectOnboardingCompletionFailsClosedForUnknownDiagnosticClass(t *testing.T) {
	m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
	m.screen, m.noColor = screenDirectOnboardingCompletion, true
	m.onboardingCompletion = configuration.OnboardingResult{Code: configuration.OnboardingFailed, Diagnostic: configuration.OnboardingDiagnostic{Phase: configuration.OnboardingPhaseCommit, Class: "raw-error-canary"}}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := updated.(Model).View()
	if !strings.Contains(view, "unavailable") || strings.Contains(view, "raw-error-canary") {
		t.Fatalf("unknown diagnostic class leaked or did not fail closed: %q", view)
	}
}

func TestDirectOnboardingCompletionRendersOnlySafeHostKeyClasses(t *testing.T) {
	for _, class := range []configuration.OnboardingFailureClass{
		configuration.OnboardingClassHostKeyTimeout,
		configuration.OnboardingClassHostKeyNegotiation,
		configuration.OnboardingClassHostKeyNoKey,
		configuration.OnboardingClassHostKeyUnavailable,
		configuration.OnboardingClassHostKeyInvalidCandidate,
	} {
		t.Run(string(class), func(t *testing.T) {
			m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
			m.screen, m.noColor = screenDirectOnboardingCompletion, true
			m.onboardingCompletion = configuration.OnboardingResult{Code: configuration.OnboardingFailed, Diagnostic: configuration.OnboardingDiagnostic{Phase: configuration.OnboardingPhaseHostKeyInspection, Class: class}}
			updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
			view := updated.(Model).View()
			if !strings.Contains(view, string(class)) || strings.Contains(view, "host.example.test") || strings.Contains(view, "raw-error-canary") {
				t.Fatalf("unsafe or incomplete host-key diagnostic rendering: %q", view)
			}
		})
	}
	m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
	m.screen, m.noColor = screenDirectOnboardingCompletion, true
	m.onboardingCompletion = configuration.OnboardingResult{Code: configuration.OnboardingFailed, Diagnostic: configuration.OnboardingDiagnostic{Phase: configuration.OnboardingPhaseHostKeyInspection, Class: "raw-error-canary"}}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := updated.(Model).View()
	if !strings.Contains(view, string(configuration.OnboardingClassHostKeyFailure)) || strings.Contains(view, "raw-error-canary") {
		t.Fatalf("unknown host-key diagnostic did not fail closed: %q", view)
	}
}

func TestDirectOnboardingCompletionRuntimeFramesPreserveMarkerViewportAndNoColor(t *testing.T) {
	const reference = "ONB-0123456789abcdef0123456789abcdef"
	for _, locale := range []localization.Localizer{localization.Spanish(), localization.English()} {
		for _, frame := range []struct {
			width, height int
			noColor       bool
		}{{120, 40, false}, {80, 24, true}, {40, 16, false}} {
			m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
			m.localizer, m.noColor, m.screen = locale, frame.noColor, screenDirectOnboardingCompletion
			m.onboardingCompletion = configuration.OnboardingResult{Code: configuration.OnboardingFailed, CleanupRequired: true, Diagnostic: configuration.OnboardingDiagnostic{Phase: configuration.OnboardingPhaseSave, Class: configuration.OnboardingClassSaveFailure, Reference: reference, Written: true}}
			updated, _ := m.Update(tea.WindowSizeMsg{Width: frame.width, Height: frame.height})
			m = updated.(Model)
			view := m.View()
			if !strings.Contains(m.profileViewportText, "[ERR]") || !strings.Contains(view, "[ERR]") || lipgloss.Width(view) > frame.width || lipgloss.Height(view) > frame.height {
				t.Fatalf("completion frame invalid at %dx%d: %q", frame.width, frame.height, view)
			}
			seenReference, seenFinalize := strings.Contains(view, "ONB-"), false
			for range 32 {
				seenReference = seenReference || strings.Contains(view, "ONB-")
				seenFinalize = seenFinalize || strings.Contains(view, m.text("onboarding.finalize", nil))
				updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
				m = updated.(Model)
				view = m.View()
			}
			seenReference = seenReference || strings.Contains(view, "ONB-")
			seenFinalize = seenFinalize || strings.Contains(view, m.text("onboarding.finalize", nil))
			if frame.width == 40 {
				seenReference = seenReference || strings.Contains(m.profileViewportText, "ONB-")
			}
			if !seenReference || !seenFinalize {
				t.Fatalf("completion reachability reference=%t finalize=%t at %dx%d: viewport=%q view=%q", seenReference, seenFinalize, frame.width, frame.height, m.profileViewportText, view)
			}
			if frame.noColor && strings.Contains(view, "\x1b[") {
				t.Fatalf("NO_COLOR completion contains ANSI: %q", view)
			}
		}
	}
}

func TestDirectOnboardingCompletionUsesSemanticColorsAndNoColor(t *testing.T) {
	previous := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(previous)
	lipgloss.SetColorProfile(termenv.TrueColor)
	result := configuration.OnboardingResult{Code: configuration.OnboardingFailed, Diagnostic: configuration.OnboardingDiagnostic{Phase: configuration.OnboardingPhaseCommit, Class: configuration.OnboardingClassCommitFailure}}
	m := NewModelWithOnboarding(&profileStoreStub{}, context.Background(), &onboardingOperationsStub{}, remote.SecretPrompt{})
	m.screen, m.onboardingCompletion = screenDirectOnboardingCompletion, result
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if view := updated.(Model).View(); !strings.Contains(view, "[ERR]") || !strings.Contains(view, "\x1b[") {
		t.Fatalf("true-color failure frame lost error semantics: %q", view)
	}
	m.noColor = true
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if view := updated.(Model).View(); strings.Contains(view, "\x1b[") {
		t.Fatalf("NO_COLOR failure frame contains ANSI: %q", view)
	}
}

func TestCreateNeverRoutesToLegacyOnboardingWithoutOperations(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := updated.(Model).screen; got != screenDirectOnboarding {
		t.Fatalf("Create screen = %d, want direct onboarding", got)
	}
}
