package tui

import (
	"context"
	"strings"
	"time"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/credential"
	"bac-nexus/internal/localization"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
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
	result       string
	err          error
	width        int
	height       int
	trust        [3]textinput.Model
	confirm      textinput.Model
	tofuEvidence string
	cancel       context.CancelFunc
	viewport     viewport.Model
	viewportText string
	noColor      bool
	localizer    localization.Localizer
}

func NewSecurityModel(ctx context.Context, profileName string, services SecurityServices) SecurityModel {
	return NewSecurityModelWithLocalizer(ctx, profileName, services, localization.Spanish())
}

// NewSecurityModelWithLocalizer is the explicit locale injection seam.
func NewSecurityModelWithLocalizer(ctx context.Context, profileName string, services SecurityServices, localizer localization.Localizer) SecurityModel {
	if ctx == nil {
		ctx = context.Background()
	}
	if localizer == nil {
		panic("nil localizer")
	}
	m := SecurityModel{ctx: ctx, profile: profileName, services: services, screen: securityMenu, viewport: viewport.New(1, 1), noColor: noColorEnabled(), localizer: localizer}
	labels := []string{m.text("security.trust.fingerprint", nil), m.text("security.trust.provenance", nil), m.text("security.trust.confirmation", nil)}
	for i, label := range labels {
		m.trust[i] = textinput.New()
		m.trust[i].Prompt = label + ": "
		m.trust[i].CharLimit = 256
	}
	m.confirm = m.newConfirmationInput()
	m.trust[0].Focus()
	return m
}

func (m SecurityModel) text(id string, data map[string]any) string { return m.localizer.Text(id, data) }

func (m SecurityModel) newConfirmationInput() textinput.Model {
	input := textinput.New()
	input.Prompt, input.CharLimit = m.text("common.confirmation", nil)+": ", 256
	input.Focus()
	return input
}

func (m SecurityModel) Init() tea.Cmd { return m.statusCmd() }

func (m SecurityModel) statusCmd() tea.Cmd {
	return func() tea.Msg {
		if m.services == nil {
			return securityOutcomeMsg{text: m.text("operation.unavailable", nil), err: configuration.ErrCredentialUnavailable}
		}
		presence, err := m.services.Status(m.ctx, m.profile)
		if err != nil {
			return securityOutcomeMsg{text: m.text("operation.unavailable", nil), err: err}
		}
		return credentialStatusMsg{presence: presence}
	}
}

func (m SecurityModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.refreshViewport()
		return m, nil
	case credentialStatusMsg:
		m.status, m.result, m.err = msg.presence, "", nil
		m.refreshViewport()
		return m, nil
	case securityOutcomeMsg:
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		m.screen = securityResult
		m.result, m.err = msg.text, msg.err
		m.refreshViewport()
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
		m.confirm = m.newConfirmationInput()
		m.refreshViewport()
		return m, nil
	case tea.KeyMsg:
		return m.updateSecurityKey(msg)
	}
	return m, nil
}

func (m SecurityModel) updateSecurityKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "pgup" {
		m.viewport.ViewUp()
		return m, nil
	}
	if key == "pgdown" {
		m.viewport.ViewDown()
		return m, nil
	}
	editableConfirmation := m.screen == securityConfirmCredential || m.screen == securityConfirmTOFU
	if m.screen != securityTrust && !editableConfirmation {
		switch key {
		case "up", "k":
			m.viewport.LineUp(1)
			return m, nil
		case "down", "j":
			m.viewport.LineDown(1)
			return m, nil
		}
	}
	if m.screen == securityProgress {
		if key == "esc" || key == "ctrl+c" {
			if m.cancel != nil {
				m.cancel()
				m.cancel = nil
			}
			m.screen, m.result = securityMenu, m.text("operation.cancelled", nil)
		}
		return m, nil
	}
	if key == "esc" || (key == "b" && !editableConfirmation) {
		m.screen = securityMenu
		m.err = nil
		return m, nil
	}
	confirmationTarget := ""
	if m.screen == securityConfirmCredential {
		confirmationTarget = "delete credential " + m.profile
	}
	if m.screen == securityConfirmTOFU {
		confirmationTarget = "enroll " + m.tofuEvidence
	}
	if (m.screen == securityConfirmCredential || m.screen == securityConfirmTOFU) && key != "enter" && !(key == "n" && !strings.HasPrefix(confirmationTarget, m.confirm.Value()+"n")) {
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
				return m.credentialOutcomeText(outcome), err
			})
		case "r":
			return m.start(func(ctx context.Context) (string, error) {
				outcome, err := m.services.Rotate(ctx, m.profile)
				return m.credentialOutcomeText(outcome), err
			})
		case "d":
			m.screen = securityConfirmCredential
			m.confirm = m.newConfirmationInput()
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
				return m.credentialOutcomeText(outcome), err
			})
		}
		if key == "n" || key == "esc" {
			m.screen = securityMenu
		}
	case securityConfirmMigration:
		if key == "y" || key == "enter" {
			return m.start(func(ctx context.Context) (string, error) {
				outcome, err := m.services.Migrate(ctx, m.profile, true)
				return m.credentialOutcomeText(outcome), err
			})
		}
		if key == "n" {
			m.screen = securityMenu
		}
	case securityConfirmTOFU:
		if key == "enter" && strings.TrimSpace(m.confirm.Value()) == "enroll "+m.tofuEvidence {
			return m.start(func(ctx context.Context) (string, error) {
				outcome, err := m.services.EnrollTOFU(ctx, m.profile, m.tofuEvidence, strings.TrimSpace(m.confirm.Value()))
				return m.trustOutcomeText(outcome), err
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
				return m.trustOutcomeText(outcome), err
			})
		}
		index := m.trustIndex()
		if index >= 0 {
			var cmd tea.Cmd
			m.trust[index], cmd = m.trust[index].Update(msg)
			return m, cmd
		}
	}
	m.refreshViewport()
	return m, nil
}

func (m SecurityModel) credentialOutcomeText(outcome configuration.CredentialOutcome) string {
	switch outcome {
	case configuration.CredentialOutcomeStored:
		return m.text("operation.credential_stored", nil)
	case configuration.CredentialOutcomeRotated:
		return m.text("operation.credential_rotated", nil)
	case configuration.CredentialOutcomeDeleted:
		return m.text("operation.credential_deleted", nil)
	default:
		return m.text("operation.unavailable", nil)
	}
}

func (m SecurityModel) trustOutcomeText(outcome configuration.TrustOutcome) string {
	if outcome == configuration.TrustOutcomeEnrolled {
		return m.text("operation.host_key_enrolled", nil)
	}
	return m.text("operation.unavailable", nil)
}

type securityOperation func(context.Context) (string, error)

func (m SecurityModel) start(operation securityOperation) (tea.Model, tea.Cmd) {
	if m.services == nil {
		m.screen, m.result, m.err = securityResult, m.text("operation.unavailable", nil), configuration.ErrCredentialUnavailable
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
	width := m.width
	if width <= 0 {
		width = 80
	}
	height := m.height
	if height <= 0 {
		height = 24
	}
	// The header and overflow disclosure are fixed. All functional content,
	// including the menu instructions and warnings, belongs to the viewport.
	header := wrapWizardText(m.text("security.header", map[string]any{"Profile": m.profile}), width, "")
	viewportHeight := max(height-len(header)-1, 1)
	content := m.viewportContent(width)
	if m.viewport.Width != width || m.viewport.Height != viewportHeight || m.viewportText != content {
		m.viewport.Width, m.viewport.Height = width, viewportHeight
		m.viewport.SetContent(content)
	}
	return strings.Join(header, "\n") + "\n" + strings.TrimRight(m.viewport.View(), "\n") + "\n" + wizardOverflowIndicator(m.viewport, width, newHomeTheme(m.noColor), m.text("overflow.above", nil), m.text("overflow.below", nil))
}

func (m *SecurityModel) refreshViewport() {
	width := m.width
	if width <= 0 {
		width = 80
	}
	height := m.height
	if height <= 0 {
		height = 24
	}
	headerHeight := len(wrapWizardText(m.text("security.header", map[string]any{"Profile": m.profile}), width, ""))
	m.viewport.Width, m.viewport.Height = width, max(height-headerHeight-1, 1)
	m.viewportText = m.viewportContent(width)
	m.viewport.SetContent(m.viewportText)
	if m.screen == securityTrust {
		needle := m.trust[m.trustIndex()].Prompt
		for line, text := range strings.Split(m.viewportText, "\n") {
			if strings.Contains(text, needle) {
				if line < m.viewport.YOffset {
					m.viewport.SetYOffset(line)
				} else if line >= m.viewport.YOffset+m.viewport.Height {
					m.viewport.SetYOffset(line - m.viewport.Height + 1)
				}
				break
			}
		}
	}
}

func (m SecurityModel) viewportContent(width int) string {
	var b strings.Builder
	appendText := func(text string) {
		for _, line := range wrapWizardText(text, width, "") {
			b.WriteString(line + "\n")
		}
	}
	switch m.screen {
	case securityMenu:
		appendText(m.text("security.status", map[string]any{"Status": credentialPresence(m.status, m.localizer)}))
		appendText(m.text("security.menu", nil))
	case securityProgress:
		appendText(m.text("security.progress", nil))
	case securityConfirmCredential:
		appendText(m.text("security.confirm_credential", map[string]any{"Profile": m.profile}))
		b.WriteString(m.confirm.View() + "\n")
	case securityConfirmMigration:
		appendText(m.text("security.confirm_migration", nil))
	case securityConfirmTOFU:
		appendText(m.text("security.tofu_warning", nil))
		appendText(m.tofuEvidence)
		appendText(m.text("security.confirm_tofu", map[string]any{"Evidence": m.tofuEvidence}))
		b.WriteString(m.confirm.View() + "\n")
	case securityTrust:
		appendText(m.text("security.manual", nil))
		for i := range m.trust {
			b.WriteString(m.trust[i].View() + "\n")
		}
	case securityResult:
		appendText(m.result)
		if m.err != nil {
			for _, line := range wrapWizardText(sanitizeError(m.err), width, m.text("security.error_prefix", nil)) {
				b.WriteString(line + "\n")
			}
		}
		appendText(m.text("security.footer_back", nil))
	}
	return strings.TrimRight(b.String(), "\n")
}

func credentialPresence(p credential.Presence, localizer localization.Localizer) string {
	switch p {
	case credential.PresencePresent:
		return localizer.Text("credential.present", nil)
	case credential.PresenceAbsent:
		return localizer.Text("credential.absent", nil)
	default:
		return localizer.Text("credential.unavailable", nil)
	}
}
