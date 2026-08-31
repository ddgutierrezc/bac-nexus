package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/profile"
	"bac-nexus/internal/remote"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// OnboardingOperations is the secret-free application boundary used by the
// direct TUI route. The service takes ownership of captured bytes immediately.
type OnboardingOperations interface {
	StartCaptured(context.Context, configuration.OnboardingRequest, []byte) (configuration.OperationIdentity, configuration.OnboardingCode)
	Wait(context.Context, string) configuration.OnboardingResult
	Cancel(string)
}

type onboardingFocus uint8

const (
	onboardingFocusHost onboardingFocus = iota
	onboardingFocusUsername
	onboardingFocusConnect
)

type onboardingPromptMsg struct {
	ID         string
	Generation uint64
	Code       remote.PromptCode
}

// onboardingResultMsg carries only operation identity and secret-free result
// metadata. It must never gain a password field.
type onboardingResultMsg struct {
	ID         string
	Generation uint64
	Result     configuration.OnboardingResult
}

// onboardingExecCommand is a fixed in-process tea.Exec command. It has no
// executable, arguments, shell, or terminal-mode changes.
type onboardingExecCommand struct {
	ctx        context.Context
	prompt     remote.SecretPrompt
	operations OnboardingOperations
	request    configuration.OnboardingRequest
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	message    onboardingPromptMsg
}

func newOnboardingExecCommand(ctx context.Context, prompt remote.SecretPrompt, operations OnboardingOperations, request configuration.OnboardingRequest) *onboardingExecCommand {
	return &onboardingExecCommand{ctx: ctx, prompt: prompt, operations: operations, request: request}
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
	secret, code := c.prompt.Capture(c.ctx, input, output, "IBM i password")
	c.message.Code = code
	if code != remote.PromptCaptured {
		return nil
	}
	identity, start := c.operations.StartCaptured(c.ctx, c.request, secret)
	remote.Zero(secret)
	if start != configuration.OnboardingStarted {
		c.message.Code = remote.PromptUnavailable
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
	m.directHost = newDirectOnboardingInput(253)
	m.directUsername = newDirectOnboardingInput(128)
	m.directFocus = onboardingFocusHost
	m.directHost.Focus()
	m.onboardingFeedback = ""
	m.onboardingCompletion = configuration.OnboardingResult{}
	m.onboardingOperation = configuration.OperationIdentity{}
	m.onboardingRunning = false
	m.screen = screenDirectOnboarding
}

func (m Model) updateDirectOnboarding(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.screen, m.onboardingFeedback = screenHome, ""
		return m, nil
	case "tab", "down":
		m.focusDirectOnboarding(1)
		return m, nil
	case "shift+tab", "up":
		m.focusDirectOnboarding(-1)
		return m, nil
	case "enter":
		if m.directFocus != onboardingFocusConnect {
			m.focusDirectOnboarding(1)
			return m, nil
		}
		host, username := strings.TrimSpace(m.directHost.Value()), strings.TrimSpace(m.directUsername.Value())
		if profile.ValidateHost(host) != nil || profile.ValidateUsername(username) != nil {
			m.onboardingFeedback = m.text("onboarding.validation", nil)
			if profile.ValidateHost(host) != nil {
				m.directFocus = onboardingFocusHost
			} else {
				m.directFocus = onboardingFocusUsername
			}
			m.focusDirectOnboarding(0)
			return m, nil
		}
		if m.onboardingOperations == nil {
			m.onboardingFeedback = m.text("onboarding.unavailable", nil)
			return m, nil
		}
		m.onboardingRunning, m.onboardingFeedback = true, ""
		command := newOnboardingExecCommand(m.onboardingContext, m.onboardingPrompt, m.onboardingOperations, configuration.OnboardingRequest{Host: host, Username: username})
		return m, tea.Exec(command, func(error) tea.Msg { return command.result() })
	}
	var cmd tea.Cmd
	if m.directFocus == onboardingFocusHost {
		m.directHost, cmd = m.directHost.Update(msg)
	} else if m.directFocus == onboardingFocusUsername {
		m.directUsername, cmd = m.directUsername.Update(msg)
	}
	m.onboardingFeedback = ""
	return m, cmd
}

func (m *Model) focusDirectOnboarding(delta int) {
	count := int(onboardingFocusConnect) + 1
	m.directFocus = onboardingFocus((int(m.directFocus) + delta + count) % count)
	m.directHost.Blur()
	m.directUsername.Blur()
	if m.directFocus == onboardingFocusHost {
		m.directHost.Focus()
	}
	if m.directFocus == onboardingFocusUsername {
		m.directUsername.Focus()
	}
}

func (m Model) renderDirectOnboarding() string {
	var b strings.Builder
	b.WriteString(m.text("onboarding.title", nil) + "\n\n")
	b.WriteString(m.text("onboarding.description", nil) + "\n\n")
	hostMarker, usernameMarker, actionMarker := " ", " ", " "
	if m.directFocus == onboardingFocusHost {
		hostMarker = "▸"
	}
	if m.directFocus == onboardingFocusUsername {
		usernameMarker = "▸"
	}
	if m.directFocus == onboardingFocusConnect {
		actionMarker = "▸"
	}
	fmt.Fprintf(&b, "%s %s: %s\n%s %s: %s\n\n%s [ %s ]\n", hostMarker, m.text("onboarding.host", nil), m.directHost.View(), usernameMarker, m.text("onboarding.username", nil), m.directUsername.View(), actionMarker, m.text("onboarding.connect", nil))
	if m.onboardingFeedback != "" {
		b.WriteString("\n[ERR] " + m.onboardingFeedback + "\n")
	}
	b.WriteString("\n" + m.text("onboarding.footer", nil) + "\n")
	return m.renderLegacyViewport(b.String())
}

func (m Model) renderDirectOnboardingRunning() string {
	return m.renderLegacyViewport(m.text("onboarding.running", nil) + "\n\n" + m.text("onboarding.cancel", nil) + "\n")
}

func (m Model) renderDirectOnboardingCompletion() string {
	message := m.text("onboarding.failed", nil)
	if m.onboardingCompletion.Code == configuration.OnboardingSaved {
		message = m.text("onboarding.saved", nil)
	} else if m.onboardingCompletion.CleanupRequired {
		message = m.text("onboarding.failed_cleanup", nil)
	}
	return m.renderLegacyViewport(message + "\n\n▸ [ " + m.text("onboarding.finalize", nil) + " ]\n")
}
