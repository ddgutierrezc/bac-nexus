package tui

import (
	"context"
	"strings"
	"time"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/credential"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const securityOperationTimeout = 15 * time.Second

type securityScreen uint8

const (
	securityMenu securityScreen = iota
	securityProgress
	securityConfirmCredential
	securityConfirmMigration
	securityTrust
	securityConfirmTOFU
	securityResult
)

// SecurityServices is the secret-free TUI boundary. Implementations own
// transient secret entry and return only presence or opaque outcomes.
type SecurityServices interface {
	Status(context.Context, string) (credential.Presence, error)
	Set(context.Context, string) (configuration.CredentialOutcome, error)
	Rotate(context.Context, string) (configuration.CredentialOutcome, error)
	Delete(context.Context, string, string) (configuration.CredentialOutcome, error)
	Migrate(context.Context, string, bool) (configuration.CredentialOutcome, error)
	EnrollManual(context.Context, string, string, string, string) (configuration.TrustOutcome, error)
	InspectTOFU(context.Context, string) (string, error)
	EnrollTOFU(context.Context, string, string, string) (configuration.TrustOutcome, error)
	InspectAndEnroll(context.Context, string, bool, string) (configuration.TrustOutcome, error)
}

type credentialStatusMsg struct{ presence credential.Presence }
type securityOutcomeMsg struct {
	text string
	err  error
}
type tofuObservationMsg struct {
	evidence string
	err      error
}

// SecurityModel contains only non-secret form values and typed operation
// results. Secret bytes never enter commands, messages, or rendered views.
type SecurityModel struct {
	ctx          context.Context
	profile      string
	services     SecurityServices
	screen       securityScreen
	status       credential.Presence
	text         string
	err          error
	width        int
	height       int
	trust        [3]textinput.Model
	confirm      textinput.Model
	tofuEvidence string
	cancel       context.CancelFunc
}

func NewSecurityModel(ctx context.Context, profileName string, services SecurityServices) SecurityModel {
	if ctx == nil {
		ctx = context.Background()
	}
	m := SecurityModel{ctx: ctx, profile: profileName, services: services, screen: securityMenu}
	labels := []string{"Fingerprint", "Provenance", "Exact confirmation"}
	for i, label := range labels {
		m.trust[i] = textinput.New()
		m.trust[i].Prompt = label + ": "
		m.trust[i].CharLimit = 256
	}
	m.confirm = newConfirmationInput()
	m.trust[0].Focus()
	return m
}

func (m SecurityModel) Init() tea.Cmd { return m.statusCmd() }

func (m SecurityModel) statusCmd() tea.Cmd {
	return func() tea.Msg {
		if m.services == nil {
			return securityOutcomeMsg{text: "Credential status unavailable", err: configuration.ErrCredentialUnavailable}
		}
		presence, err := m.services.Status(m.ctx, m.profile)
		if err != nil {
			return securityOutcomeMsg{text: "Credential status unavailable", err: err}
		}
		return credentialStatusMsg{presence: presence}
	}
}

func (m SecurityModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case credentialStatusMsg:
		m.status, m.text, m.err = msg.presence, "", nil
		return m, nil
	case securityOutcomeMsg:
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		m.screen = securityResult
		m.text, m.err = msg.text, msg.err
		return m, nil
	case tofuObservationMsg:
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		if msg.err != nil {
			m.screen, m.err = securityResult, msg.err
			return m, nil
		}
		m.tofuEvidence, m.err, m.screen = msg.evidence, nil, securityConfirmTOFU
		m.confirm = newConfirmationInput()
		return m, nil
	case tea.KeyMsg:
		return m.updateSecurityKey(msg)
	}
	return m, nil
}

func (m SecurityModel) updateSecurityKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.screen == securityProgress {
		if key == "esc" || key == "ctrl+c" {
			if m.cancel != nil {
				m.cancel()
				m.cancel = nil
			}
			m.screen, m.text = securityMenu, "Operation cancelled"
		}
		return m, nil
	}
	if key == "esc" || key == "b" {
		m.screen = securityMenu
		m.err = nil
		return m, nil
	}
	if (m.screen == securityConfirmCredential || m.screen == securityConfirmTOFU) && key != "enter" && key != "n" {
		var cmd tea.Cmd
		m.confirm, cmd = m.confirm.Update(msg)
		return m, cmd
	}
	switch m.screen {
	case securityMenu:
		switch key {
		case "s":
			return m.start(func(ctx context.Context) (string, error) {
				outcome, err := m.services.Set(ctx, m.profile)
				return "Credential " + string(outcome), err
			})
		case "r":
			return m.start(func(ctx context.Context) (string, error) {
				outcome, err := m.services.Rotate(ctx, m.profile)
				return "Credential " + string(outcome), err
			})
		case "d":
			m.screen = securityConfirmCredential
			m.confirm = newConfirmationInput()
		case "m":
			m.screen = securityConfirmMigration
		case "t":
			m.screen = securityTrust
		case "o":
			if m.services == nil {
				return m, nil
			}
			ctx, cancel := context.WithTimeout(m.ctx, securityOperationTimeout)
			m.cancel, m.screen, m.err = cancel, securityProgress, nil
			return m, func() tea.Msg {
				evidence, err := m.services.InspectTOFU(ctx, m.profile)
				return tofuObservationMsg{evidence: evidence, err: err}
			}
		}
	case securityConfirmCredential:
		if key == "enter" && strings.TrimSpace(m.confirm.Value()) == "delete credential "+m.profile {
			return m.start(func(ctx context.Context) (string, error) {
				outcome, err := m.services.Delete(ctx, m.profile, "delete credential "+m.profile)
				return "Credential " + string(outcome), err
			})
		}
		if key == "n" || key == "esc" {
			m.screen = securityMenu
		}
	case securityConfirmMigration:
		if key == "y" || key == "enter" {
			return m.start(func(ctx context.Context) (string, error) {
				outcome, err := m.services.Migrate(ctx, m.profile, true)
				return "Credential " + string(outcome), err
			})
		}
		if key == "n" {
			m.screen = securityMenu
		}
	case securityConfirmTOFU:
		if key == "enter" && strings.TrimSpace(m.confirm.Value()) == "enroll "+m.tofuEvidence {
			return m.start(func(ctx context.Context) (string, error) {
				outcome, err := m.services.EnrollTOFU(ctx, m.profile, m.tofuEvidence, strings.TrimSpace(m.confirm.Value()))
				return "Host key " + string(outcome), err
			})
		}
		if key == "n" || key == "esc" {
			m.screen = securityMenu
		}
	case securityTrust:
		if key == "tab" || key == "down" {
			m.focusTrust(1)
			return m, nil
		}
		if key == "shift+tab" || key == "up" {
			m.focusTrust(-1)
			return m, nil
		}
		if key == "enter" {
			fingerprint, provenance, confirmation := strings.TrimSpace(m.trust[0].Value()), strings.TrimSpace(m.trust[1].Value()), strings.TrimSpace(m.trust[2].Value())
			return m.start(func(ctx context.Context) (string, error) {
				outcome, err := m.services.EnrollManual(ctx, m.profile, fingerprint, provenance, confirmation)
				return "Host key " + string(outcome), err
			})
		}
		index := m.trustIndex()
		if index >= 0 {
			var cmd tea.Cmd
			m.trust[index], cmd = m.trust[index].Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

type securityOperation func(context.Context) (string, error)

func (m SecurityModel) start(operation securityOperation) (tea.Model, tea.Cmd) {
	if m.services == nil {
		m.screen, m.text, m.err = securityResult, "Operation unavailable", configuration.ErrCredentialUnavailable
		return m, nil
	}
	ctx, cancel := context.WithTimeout(m.ctx, securityOperationTimeout)
	m.cancel, m.screen, m.err = cancel, securityProgress, nil
	return m, func() tea.Msg {
		text, err := operation(ctx)
		return securityOutcomeMsg{text: text, err: err}
	}
}

func (m *SecurityModel) focusTrust(delta int) {
	index := m.trustIndex()
	if index < 0 {
		index = 0
	}
	index = (index + delta + len(m.trust)) % len(m.trust)
	for i := range m.trust {
		m.trust[i].Blur()
	}
	m.trust[index].Focus()
}

func (m SecurityModel) trustIndex() int {
	for i := range m.trust {
		if m.trust[i].Focused() {
			return i
		}
	}
	return -1
}

func (m SecurityModel) View() string {
	var b strings.Builder
	b.WriteString("Security for " + m.profile + "\n\n")
	switch m.screen {
	case securityMenu:
		b.WriteString("Credential: " + credentialPresence(m.status) + "\n")
		b.WriteString("s set  r rotate  d delete  m migrate\n")
		b.WriteString("t manual trust  o inspect TOFU  esc back\n")
	case securityProgress:
		b.WriteString("Operation in progress; esc cancels.\n")
	case securityConfirmCredential:
		b.WriteString("Delete this profile credential? Type delete credential " + m.profile + " exactly, then press enter.\n" + m.confirm.View() + "\n")
	case securityConfirmMigration:
		b.WriteString("Migrate the legacy credential vault? Type y only after review.\n")
	case securityConfirmTOFU:
		b.WriteString("WARNING: remote inspection is unverified TOFU. Observed evidence:\n" + m.tofuEvidence + "\nType enroll " + m.tofuEvidence + " exactly, then press enter.\n" + m.confirm.View() + "\n")
	case securityTrust:
		b.WriteString("Manual verified host-key enrollment\n")
		for i := range m.trust {
			b.WriteString(m.trust[i].View() + "\n")
		}
	case securityResult:
		b.WriteString(m.text + "\n")
		if m.err != nil {
			b.WriteString("Error: " + sanitizeError(m.err) + "\n")
		}
		b.WriteString("b back\n")
	}
	return responsive(b.String(), m.width, m.height)
}

func credentialPresence(p credential.Presence) string {
	switch p {
	case credential.PresencePresent:
		return "present"
	case credential.PresenceAbsent:
		return "absent"
	default:
		return "unavailable"
	}
}
