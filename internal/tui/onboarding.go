package tui

import (
	"context"
	"io"
	"os"
	"strconv"
	"strings"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// OnboardingOperations is the secret-free application seam. Password bytes
// cross it only inside the fixed terminal command and never enter Model state.
type OnboardingOperations interface {
	Capture(context.Context, configuration.OnboardingRequest, remote.SecretPrompt, *os.File, *os.File, string) (configuration.OperationIdentity, remote.PromptCode)
	StartCaptured(context.Context, configuration.OnboardingRequest, configuration.OperationIdentity) configuration.OnboardingCode
	Wait(context.Context, string) configuration.OnboardingResult
	Cancel(string)
	Revoke(configuration.OperationIdentity)
}

type onboardingStep uint8

const (
	onboardingStepName onboardingStep = iota
	onboardingStepConnection
	onboardingStepCredentials
	onboardingStepReview
)

type onboardingFocus uint8

const (
	onboardingFocusName onboardingFocus = iota
	onboardingFocusNameNext
	onboardingFocusHost
	onboardingFocusUsername
	onboardingFocusPort
	onboardingFocusConnectionBack
	onboardingFocusConnectionNext
	onboardingFocusCredentialsBack
	onboardingFocusCapture
	onboardingFocusReviewBack
	onboardingFocusConnect
)

type onboardingPromptMsg struct {
	ID         string
	Generation uint64
	Code       remote.PromptCode
}
type onboardingResultMsg struct {
	ID         string
	Generation uint64
	Result     configuration.OnboardingResult
}

// onboardingExecCommand is fixed in-process tea.Exec plumbing: it never starts
// a shell or external process and only returns identity/status metadata.
type onboardingExecCommand struct {
	ctx        context.Context
	prompt     remote.SecretPrompt
	operations OnboardingOperations
	request    configuration.OnboardingRequest
	promptText string
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	message    onboardingPromptMsg
}

func newOnboardingExecCommand(ctx context.Context, prompt remote.SecretPrompt, operations OnboardingOperations, request configuration.OnboardingRequest, localizedPrompt ...string) *onboardingExecCommand {
	promptText := ""
	if len(localizedPrompt) == 1 {
		promptText = localizedPrompt[0]
	}
	return &onboardingExecCommand{ctx: ctx, prompt: prompt, operations: operations, request: request, promptText: promptText}
}
func (c *onboardingExecCommand) SetStdin(input io.Reader)   { c.stdin = input }
func (c *onboardingExecCommand) SetStdout(output io.Writer) { c.stdout = output }
func (c *onboardingExecCommand) SetStderr(output io.Writer) { c.stderr = output }
func (c *onboardingExecCommand) Run() error {
	input, inputOK := c.stdin.(*os.File)
	output, outputOK := c.stderr.(*os.File)
	if !inputOK || !outputOK || c.operations == nil {
		c.message.Code = remote.PromptTerminalUnavailable
		return nil
	}
	identity, code := c.operations.Capture(c.ctx, c.request, c.prompt, input, output, c.promptText)
	c.message.Code = code
	if code != remote.PromptCaptured {
		return nil
	}
	c.message.ID, c.message.Generation = identity.ID, identity.Generation
	return nil
}
func (c *onboardingExecCommand) result() onboardingPromptMsg { return c.message }

func newDirectOnboardingInput(limit int) textinput.Model {
	input := textinput.New()
	input.Prompt, input.CharLimit, input.Width = "", limit, 36
	input.SetCursorMode(textinput.CursorStatic)
	input.Cursor.SetChar("█")
	return input
}

func (m *Model) beginDirectOnboarding() {
	m.revokeOnboardingLease()
	m.directName, m.directHost = newDirectOnboardingInput(64), newDirectOnboardingInput(253)
	m.directUsername, m.directPort = newDirectOnboardingInput(128), newDirectOnboardingInput(5)
	m.directPort.SetValue("22")
	m.onboardingStep, m.directFocus = onboardingStepName, onboardingFocusName
	m.focusDirectOnboarding(0)
	m.onboardingFeedback, m.onboardingValidationFeedback = "", ""
	m.onboardingCompletion, m.onboardingOperation = configuration.OnboardingResult{}, configuration.OperationIdentity{}
	m.onboardingRunning, m.onboardingCaptured, m.screen = false, false, screenDirectOnboarding
}

func (m Model) updateDirectOnboarding(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.revokeOnboardingLease()
		if m.onboardingStep == onboardingStepName {
			m.screen = screenHome
		} else {
			m.onboardingStep--
			m.focusDirectOnboarding(0)
			m.refreshDirectOnboardingViewport()
		}
		return m, nil
	case "tab", "down":
		m.focusDirectOnboarding(1)
		m.refreshDirectOnboardingViewport()
		return m, nil
	case "shift+tab", "up":
		m.focusDirectOnboarding(-1)
		m.refreshDirectOnboardingViewport()
		return m, nil
	case "enter":
		return m.activateDirectOnboarding()
	}
	var cmd tea.Cmd
	switch m.directFocus {
	case onboardingFocusName:
		m.directName, cmd = m.directName.Update(msg)
	case onboardingFocusHost:
		m.directHost, cmd = m.directHost.Update(msg)
	case onboardingFocusUsername:
		m.directUsername, cmd = m.directUsername.Update(msg)
	case onboardingFocusPort:
		m.directPort, cmd = m.directPort.Update(msg)
	default:
		return m, nil
	}
	if m.directFocus == m.onboardingValidationFocus {
		m.onboardingValidationFeedback = ""
	}
	return m, cmd
}

func (m Model) activateDirectOnboarding() (tea.Model, tea.Cmd) {
	if m.directFocus == onboardingFocusConnectionBack || m.directFocus == onboardingFocusCredentialsBack || m.directFocus == onboardingFocusReviewBack {
		m.revokeOnboardingLease()
		m.onboardingStep--
		m.focusDirectOnboarding(0)
		return m, nil
	}
	if m.directFocus == onboardingFocusName {
		if focus, messageID := m.directOnboardingValidation(); messageID != "" {
			m.onboardingValidationFeedback, m.onboardingValidationFocus = m.text(messageID, nil), focus
			return m, nil
		}
		m.focusDirectOnboarding(1)
		return m, nil
	}
	if m.directFocus == onboardingFocusHost || m.directFocus == onboardingFocusUsername || m.directFocus == onboardingFocusPort {
		m.focusDirectOnboarding(1)
		return m, nil
	}
	if m.directFocus == onboardingFocusNameNext || m.directFocus == onboardingFocusConnectionNext {
		if focus, messageID := m.directOnboardingValidation(); messageID != "" {
			m.onboardingValidationFeedback, m.onboardingValidationFocus, m.directFocus = m.text(messageID, nil), focus, focus
			m.focusDirectOnboarding(0)
			m.refreshDirectOnboardingViewport()
			return m, nil
		}
		m.onboardingStep++
		m.focusDirectOnboarding(0)
		return m, nil
	}
	if m.directFocus == onboardingFocusCapture {
		if m.onboardingOperations == nil {
			m.onboardingFeedback = m.text("onboarding.unavailable", nil)
			return m, nil
		}
		if m.onboardingOperation.ID != "" {
			m.onboardingOperations.Revoke(m.onboardingOperation)
		}
		request := m.directOnboardingRequest()
		command := newOnboardingExecCommand(m.onboardingContext, m.onboardingPrompt, m.onboardingOperations, request, m.text("onboarding.password_prompt", nil))
		return m, tea.Exec(command, func(error) tea.Msg { return command.result() })
	}
	if m.directFocus == onboardingFocusConnect {
		if !m.onboardingCaptured || m.onboardingOperation.ID == "" {
			m.onboardingFeedback = m.text("onboarding.capture_required", nil)
			return m, nil
		}
		if m.onboardingOperations.StartCaptured(m.onboardingContext, m.directOnboardingRequest(), m.onboardingOperation) != configuration.OnboardingStarted {
			m.onboardingCaptured, m.onboardingFeedback = false, m.text("onboarding.capture_required", nil)
			return m, nil
		}
		m.onboardingRunning, m.onboardingFeedback, m.screen = true, "", screenDirectOnboardingRunning
		id, generation := m.onboardingOperation.ID, m.onboardingOperation.Generation
		return m, func() tea.Msg {
			return onboardingResultMsg{ID: id, Generation: generation, Result: m.onboardingOperations.Wait(m.onboardingContext, id)}
		}
	}
	return m, nil
}

func (m *Model) revokeOnboardingLease() {
	if m.onboardingOperations != nil && m.onboardingOperation.ID != "" {
		m.onboardingOperations.Revoke(m.onboardingOperation)
	}
	m.onboardingOperation = configuration.OperationIdentity{}
	m.onboardingCaptured = false
}

func (m Model) directOnboardingRequest() configuration.OnboardingRequest {
	port, _ := strconv.Atoi(strings.TrimSpace(m.directPort.Value()))
	return configuration.OnboardingRequest{Name: strings.TrimSpace(m.directName.Value()), Host: strings.TrimSpace(m.directHost.Value()), Port: port, Username: strings.TrimSpace(m.directUsername.Value())}
}

func (m Model) directOnboardingValidation() (onboardingFocus, string) {
	if !m.profilesLoaded {
		return onboardingFocusName, "onboarding.validation.name_loading"
	}
	name := strings.TrimSpace(m.directName.Value())
	if profile.ValidateName(name) != nil {
		return onboardingFocusName, "profile.validation.name"
	}
	for _, existing := range m.profiles {
		if strings.EqualFold(existing.Name, name) {
			return onboardingFocusName, "onboarding.validation.name_duplicate"
		}
	}
	if m.onboardingStep == onboardingStepName {
		return onboardingFocusNameNext, ""
	}
	if profile.ValidateHost(strings.TrimSpace(m.directHost.Value())) != nil {
		return onboardingFocusHost, "onboarding.validation.host"
	}
	if profile.ValidateUsername(strings.TrimSpace(m.directUsername.Value())) != nil {
		return onboardingFocusUsername, "onboarding.validation.username"
	}
	port, err := strconv.Atoi(strings.TrimSpace(m.directPort.Value()))
	if err != nil || profile.ValidatePort(port) != nil {
		return onboardingFocusPort, "onboarding.validation.port"
	}
	return onboardingFocusConnectionNext, ""
}

func (m *Model) focusDirectOnboarding(delta int) {
	order := m.onboardingFocusOrder()
	current := 0
	for i, focus := range order {
		if focus == m.directFocus {
			current = i
			break
		}
	}
	m.directFocus = order[(current+delta+len(order))%len(order)]
	m.directName.Blur()
	m.directHost.Blur()
	m.directUsername.Blur()
	m.directPort.Blur()
	switch m.directFocus {
	case onboardingFocusName:
		m.directName.Focus()
	case onboardingFocusHost:
		m.directHost.Focus()
	case onboardingFocusUsername:
		m.directUsername.Focus()
	case onboardingFocusPort:
		m.directPort.Focus()
	}
}
func (m Model) onboardingFocusOrder() []onboardingFocus {
	switch m.onboardingStep {
	case onboardingStepName:
		return []onboardingFocus{onboardingFocusName, onboardingFocusNameNext}
	case onboardingStepConnection:
		return []onboardingFocus{onboardingFocusHost, onboardingFocusUsername, onboardingFocusPort, onboardingFocusConnectionBack, onboardingFocusConnectionNext}
	case onboardingStepCredentials:
		return []onboardingFocus{onboardingFocusCredentialsBack, onboardingFocusCapture}
	default:
		return []onboardingFocus{onboardingFocusReviewBack, onboardingFocusConnect}
	}
}

func (m Model) renderDirectOnboarding() string {
	return m.profileScreen(m.text("onboarding.title", nil), m.directOnboardingBody(), m.text("onboarding.footer", nil), m.directOnboardingFocusText())
}
func (m Model) directOnboardingBody() string {
	var b strings.Builder
	b.WriteString(m.text("onboarding.step."+strconv.Itoa(int(m.onboardingStep)+1), nil) + "\n\n")
	field := func(f onboardingFocus, label string, input textinput.Model) {
		marker := " "
		if m.directFocus == f {
			marker = "▸"
		}
		b.WriteString(m.profileField(marker, label, input.View()) + "\n")
	}
	action := func(f onboardingFocus, id string) {
		marker := " "
		if m.directFocus == f {
			marker = "▸"
		}
		b.WriteString(m.profileAction(marker, m.text(id, nil)) + "\n")
	}
	switch m.onboardingStep {
	case onboardingStepName:
		b.WriteString(m.text("onboarding.name.description", nil) + "\n\n")
		field(onboardingFocusName, m.text("onboarding.name", nil), m.directName)
		b.WriteString("\n")
		action(onboardingFocusNameNext, "onboarding.next")
	case onboardingStepConnection:
		b.WriteString(m.text("onboarding.connection.description", nil) + "\n\n")
		field(onboardingFocusHost, m.text("onboarding.host", nil), m.directHost)
		field(onboardingFocusUsername, m.text("onboarding.username", nil), m.directUsername)
		field(onboardingFocusPort, m.text("onboarding.port", nil), m.directPort)
		b.WriteString("\n")
		action(onboardingFocusConnectionBack, "action.back")
		action(onboardingFocusConnectionNext, "onboarding.next")
	case onboardingStepCredentials:
		b.WriteString(m.text("onboarding.credentials.description", nil) + "\n\n")
		action(onboardingFocusCredentialsBack, "action.back")
		action(onboardingFocusCapture, "onboarding.capture")
	case onboardingStepReview:
		b.WriteString(m.text("onboarding.review.description", nil) + "\n\n")
		b.WriteString(m.profileField(" ", m.text("onboarding.name", nil), m.directName.Value()) + "\n")
		b.WriteString(m.profileField(" ", m.text("onboarding.host", nil), m.directHost.Value()) + "\n")
		b.WriteString(m.profileField(" ", m.text("onboarding.port", nil), m.directPort.Value()) + "\n")
		b.WriteString(m.profileField(" ", m.text("onboarding.username", nil), m.directUsername.Value()) + "\n\n")
		action(onboardingFocusReviewBack, "action.back")
		action(onboardingFocusConnect, "onboarding.connect")
	}
	if feedback := m.directOnboardingFeedback(); feedback != "" {
		b.WriteString("\n" + m.profileFeedback("[ERR]", feedback, newHomeTheme(m.noColor)) + "\n")
	}
	return b.String()
}
func (m Model) renderDirectOnboardingRunning() string {
	running := m.text("onboarding.running", nil)
	return m.profileScreen(running, "[INFO] "+running, m.text("onboarding.cancel", nil), running)
}
func (m Model) renderDirectOnboardingCompletion() string {
	title, body, focus := m.directOnboardingCompletionPresentation()
	return m.profileScreen(title, body, m.text("onboarding.footer", nil), focus)
}

func (m Model) directOnboardingCompletionPresentation() (string, string, string) {
	if m.onboardingCompletion.Code == configuration.OnboardingSaved {
		message := m.text("onboarding.saved", nil)
		return m.text("onboarding.completion_saved", nil), m.profileFeedback("[OK]", message, newHomeTheme(m.noColor)) + "\n\n" + m.profileAction("▸", m.text("onboarding.finalize", nil)), message
	}
	message := m.text("onboarding.failed", nil)
	if m.onboardingCompletion.CleanupRequired {
		message = m.text("onboarding.failed_cleanup", nil)
	}
	diagnostic := m.onboardingCompletion.Diagnostic
	details := m.text("onboarding.diagnostic.details", map[string]any{
		"Phase": m.directOnboardingDiagnosticPhase(diagnostic.Phase),
		"Class": directOnboardingDiagnosticClass(diagnostic.Phase, diagnostic.Class),
	})
	if diagnostic.Written && validDirectOnboardingDiagnosticReference(diagnostic.Reference) {
		details += "\n" + m.text("onboarding.diagnostic.reference", map[string]any{"Reference": diagnostic.Reference})
		details += "\n" + m.text("onboarding.diagnostic.written_guidance", nil)
	} else {
		details += "\n" + m.text("onboarding.diagnostic.unavailable_guidance", nil)
	}
	body := m.profileFeedback("[ERR]", message, newHomeTheme(m.noColor)) + "\n\n" + details + "\n\n" + m.profileAction("▸", m.text("onboarding.finalize", nil))
	return m.text("onboarding.completion_failed", nil), body, message
}

func directOnboardingDiagnosticClass(phase configuration.OnboardingFailurePhase, class configuration.OnboardingFailureClass) string {
	switch class {
	case configuration.OnboardingClassHostKeyFailure, configuration.OnboardingClassHostKeyTimeout,
		configuration.OnboardingClassHostKeyNegotiation, configuration.OnboardingClassHostKeyNoKey,
		configuration.OnboardingClassHostKeyUnavailable, configuration.OnboardingClassHostKeyInvalidCandidate,
		configuration.OnboardingClassIdentityFailure, configuration.OnboardingClassProofFailure,
		configuration.OnboardingClassBootstrapAuditFailure, configuration.OnboardingClassKeyringUnavailable,
		configuration.OnboardingClassCommitFailure, configuration.OnboardingClassSaveFailure:
		return string(class)
	default:
		resultClass := configuration.ResultClass(class)
		if configuration.IsTerminalResult(resultClass) && resultClass != configuration.ResultCancelled {
			return string(class)
		}
		if phase == configuration.OnboardingPhaseHostKeyInspection {
			return string(configuration.OnboardingClassHostKeyFailure)
		}
		return "unavailable"
	}
}

func (m Model) directOnboardingDiagnosticPhase(phase configuration.OnboardingFailurePhase) string {
	switch phase {
	case configuration.OnboardingPhaseHostKeyInspection:
		return m.text("onboarding.diagnostic.phase.host_key_inspection", nil)
	case configuration.OnboardingPhaseExistingIdentity:
		return m.text("onboarding.diagnostic.phase.existing_identity", nil)
	case configuration.OnboardingPhaseAuthenticatedProof:
		return m.text("onboarding.diagnostic.phase.authenticated_proof", nil)
	case configuration.OnboardingPhaseBootstrapAudit:
		return m.text("onboarding.diagnostic.phase.bootstrap_audit", nil)
	case configuration.OnboardingPhaseKeyringPrecondition:
		return m.text("onboarding.diagnostic.phase.keyring_precondition", nil)
	case configuration.OnboardingPhaseCommit:
		return m.text("onboarding.diagnostic.phase.commit", nil)
	case configuration.OnboardingPhaseSave:
		return m.text("onboarding.diagnostic.phase.save", nil)
	default:
		return m.text("onboarding.diagnostic.phase.unavailable", nil)
	}
}

func validDirectOnboardingDiagnosticReference(reference string) bool {
	if len(reference) != len("ONB-")+32 || reference[:4] != "ONB-" {
		return false
	}
	for _, value := range reference[4:] {
		if !(value >= '0' && value <= '9' || value >= 'a' && value <= 'f') {
			return false
		}
	}
	return true
}
func (m Model) directOnboardingFeedback() string {
	if m.onboardingFeedback != "" {
		return m.onboardingFeedback
	}
	return m.onboardingValidationFeedback
}
func (m Model) directOnboardingFocusText() string {
	if m.onboardingValidationFeedback != "" {
		return m.onboardingValidationFeedback
	}
	switch m.directFocus {
	case onboardingFocusConnectionBack, onboardingFocusCredentialsBack, onboardingFocusReviewBack:
		return m.text("action.back", nil)
	case onboardingFocusName:
		return m.text("onboarding.name", nil)
	case onboardingFocusHost:
		return m.text("onboarding.host", nil)
	case onboardingFocusUsername:
		return m.text("onboarding.username", nil)
	case onboardingFocusPort:
		return m.text("onboarding.port", nil)
	case onboardingFocusCapture:
		return m.text("onboarding.capture", nil)
	case onboardingFocusConnect:
		return m.text("onboarding.connect", nil)
	}
	return m.text("onboarding.next", nil)
}
func (m *Model) refreshDirectOnboardingViewport() {
	m.refreshProfileViewport(m.text("onboarding.title", nil), m.directOnboardingBody(), m.directOnboardingFocusText())
}
func (m *Model) refreshDirectOnboardingLifecycleViewport() {
	if m.screen == screenDirectOnboardingRunning {
		running := m.text("onboarding.running", nil)
		m.refreshProfileViewport(running, "[INFO] "+running, running)
		return
	}
	title, body, focus := m.directOnboardingCompletionPresentation()
	m.refreshProfileViewport(title, body, focus)
}
