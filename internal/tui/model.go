// Package tui contains the optional local configuration adapter. It owns
// terminal state only; profile validation and persistence remain lower-layer
// responsibilities.
package tui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/hostidentity"
	"bac-nexus/internal/localization"
	"bac-nexus/internal/profile"
)

const profileLimit = 128

type screen uint8

const (
	screenHome screen = iota
	screenList
	screenDetail
	screenForm
	screenProfileStep
	screenProfileConnection
	screenProfileIdentity
	screenProfileMapepire
	screenConfirm
	screenSecurity
)

type profileStore interface {
	Save(profile.Profile) (string, error)
	List(int) ([]profile.Profile, error)
	Read(string) (profile.Profile, error)
	Update(profile.Profile, string) (profile.ProfileUpdateResult, error)
	Delete(string, profile.DeleteConfirmation) (profile.ProfileDeleteResult, error)
	Restore(string) error
}

type profilesMsg struct{ profiles []profile.Profile }
type profileMsg struct{ profile profile.Profile }
type operationMsg struct {
	code operationCode
	err  error
}

type operationCode uint8

const (
	operationLoadFailed operationCode = iota
	operationRefreshFailed
	operationProfileSaved
	operationProfileCreated
	operationProfileUpdated
	operationProfileDeleted
)

// profileStepAcceptedMsg is deliberately transport-free: later wizard steps
// can consume the accepted draft without Step 1 persisting any profile data.
type profileStepAcceptedMsg struct{ name string }

// profileConnectionAcceptedMsg is deliberately local: it carries only the
// validated Step 2 draft to the future wizard seam and triggers no I/O.
type profileConnectionAcceptedMsg struct {
	host, username string
	port           int
}

type profileConnectionDraft struct {
	host, username string
	port           int
}

type profileIdentityPhase uint8

const (
	profileIdentityAuthorize profileIdentityPhase = iota
	profileIdentityLoading
	profileIdentityReview
	profileIdentityError
	profileIdentityCompleted
)

type profileIdentityDraft struct {
	host, algorithm, fingerprint string
	trustMethod                  profile.HostKeyTrust
	port                         int
}

type profileIdentityInspectionMsg struct {
	request   uint64
	host      string
	port      int
	candidate hostidentity.Candidate
	err       error
}

type profileIdentityAcceptedMsg struct {
	request   uint64
	candidate hostidentity.Candidate
}

type field struct {
	label string
	input textinput.Model
}

// Model is the deterministic shell model. It contains profile metadata only;
// credential material is intentionally not accepted by this adapter.
type Model struct {
	store              profileStore
	screen             screen
	profiles           []profile.Profile
	selected           int
	homeSelected       homeActionID
	homeFocus          homeFocus
	menuOffset         int
	readinessOffset    int
	homeMenuRows       []homeMenuRow
	homeReadinessRows  []string
	profilesLoaded     bool
	profilesLoadFailed bool
	form               []field
	formEdit           string
	formReturn         screen
	profileName        textinput.Model
	profileFocus       profileStepFocus
	profileDraftName   string
	connectionHost     textinput.Model
	connectionUsername textinput.Model
	connectionPort     textinput.Model
	connectionFocus    profileConnectionFocus
	connectionDraft    profileConnectionDraft
	connectionReady    bool
	connectionValidate bool
	identityFocus      profileIdentityFocus
	identityPhase      profileIdentityPhase
	identityCandidate  hostidentity.Candidate
	identityDraft      profileIdentityDraft
	identityRequest    uint64
	identityCancel     context.CancelFunc
	identityInspector  hostidentity.Inspector
	identityParent     context.Context
	identityTimeout    time.Duration
	mapepireProbe      preAuthProbe
	mapepireResolution configuration.Resolution
	step8Client        profileProofClient
	wizardViewport     viewport.Model
	legacyViewport     viewport.Model
	legacyViewportText string
	wizardFocusStart   int
	wizardFocusEnd     int
	confirm            string
	confirmInput       textinput.Model
	width              int
	height             int
	noColor            bool
	buildInfo          BuildInfo
	status             string
	err                error
	security           *SecurityModel
	localizer          localization.Localizer
}

// BuildInfo is the build identity supplied by the composition root. The TUI
// does not inspect Git state or execute external commands to derive it.
type BuildInfo struct {
	Version  string
	Revision string
}

func NewModel(store configuration.ProfilesStore) Model {
	return NewModelWithBuildInfoAndInspector(store, BuildInfo{Version: "dev", Revision: "unknown"}, nil)
}

// NewModelWithBuildInfo constructs the Home model with build identity supplied
// by the caller. Empty values retain truthful local-build defaults.
func NewModelWithBuildInfo(store configuration.ProfilesStore, buildInfo BuildInfo) Model {
	return NewModelWithBuildInfoAndInspector(store, buildInfo, nil)
}

// NewModelWithBuildInfoAndInspector composes the optional no-auth inspection boundary.
func NewModelWithBuildInfoAndInspector(store configuration.ProfilesStore, buildInfo BuildInfo, inspector hostidentity.Inspector) Model {
	return newModelWithIdentityInspector(store, buildInfo, inspector, context.Background(), identityInspectionTimeout)
}

func newModelWithIdentityInspector(store configuration.ProfilesStore, buildInfo BuildInfo, inspector hostidentity.Inspector, parent context.Context, timeout time.Duration) Model {
	m := NewModelWithBuildInfoAndLocalizer(store, buildInfo, localization.Spanish())
	m.identityInspector = inspector
	m.identityParent, m.identityTimeout = parent, timeout
	return m
}

// NewModelWithBuildInfoAndLocalizer is the explicit composition seam for a
// fully validated locale. Production callers use Spanish through the default.
func NewModelWithBuildInfoAndLocalizer(store configuration.ProfilesStore, buildInfo BuildInfo, localizer localization.Localizer) Model {
	if buildInfo.Version == "" {
		buildInfo.Version = "dev"
	}
	if buildInfo.Revision == "" {
		buildInfo.Revision = "unknown"
	}
	if localizer == nil {
		panic("nil localizer")
	}
	m := Model{store: store, screen: screenHome, homeSelected: actionCreate, noColor: noColorEnabled(), buildInfo: buildInfo, wizardViewport: viewport.New(1, 1), legacyViewport: viewport.New(1, 1), localizer: localizer}
	m.form = m.newFields(profile.Profile{})
	m.profileName = newProfileNameInput()
	return m
}

func (m Model) text(id string, data map[string]any) string {
	if m.localizer == nil {
		return localization.Spanish().Text(id, data)
	}
	return m.localizer.Text(id, data)
}

func noColorEnabled() bool {
	value, present := os.LookupEnv("NO_COLOR")
	return present && value != ""
}

// NewModelWithSecurity enables the security child screen without changing
// the default profile-only constructor used by existing callers.
func NewModelWithSecurity(store configuration.ProfilesStore, ctx context.Context, services SecurityServices) Model {
	m := NewModel(store)
	m.security = ptrSecurityModel(NewSecurityModelWithLocalizer(ctx, "", services, m.localizer))
	m.security.noColor = m.noColor
	return m
}

func ptrSecurityModel(model SecurityModel) *SecurityModel { return &model }

func (m Model) newFields(p profile.Profile) []field {
	values := []struct{ label, value string }{
		{m.text("form.label.name", nil), p.Name}, {m.text("form.label.host", nil), p.Host}, {m.text("form.label.port", nil), strconv.Itoa(p.Port)},
		{m.text("form.label.username", nil), p.Username}, {m.text("form.label.fingerprint", nil), p.HostKeyFingerprint},
		{m.text("form.label.trust", nil), m.trustDisplay(p.HostKeyTrust)}, {m.text("form.label.java_home", nil), p.JavaHome},
		{m.text("form.label.mapepire_jar", nil), p.MapepireJAR}, {m.text("form.label.credential_mode", nil), m.credentialModeDisplay(p.CredentialMode)},
	}
	fields := make([]field, len(values))
	for i, v := range values {
		input := textinput.New()
		input.Prompt = ""
		input.CharLimit = 4096
		input.SetValue(v.value)
		fields[i] = field{label: v.label, input: input}
	}
	if len(fields) > 0 {
		fields[0].input.Focus()
	}
	return fields
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		profiles, err := m.store.List(profileLimit)
		if err != nil {
			return operationMsg{code: operationLoadFailed, err: err}
		}
		return profilesMsg{profiles: profiles}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.screen == screenSecurity && m.security != nil {
			m.security.noColor = m.noColor
			updated, _ := m.security.Update(msg)
			security := updated.(SecurityModel)
			m.security = &security
		}
		if m.screen == screenHome {
			m.homeSelected = m.clampHomeSelection()
			m.menuOffset = m.visibleMenuWindow().Start
			m.readinessOffset = m.visibleReadinessWindow().Start
		}
		m.refreshWizardViewport()
		m.refreshLegacyViewport()
		return m, nil
	case profilesMsg:
		m.profiles, m.err, m.profilesLoaded, m.profilesLoadFailed = append([]profile.Profile(nil), msg.profiles...), nil, true, false
		m.selected = clamp(m.selected, len(m.profiles))
		return m, nil
	case profileMsg:
		m.profiles = replaceProfile(m.profiles, msg.profile)
		m.screen, m.status, m.err = screenDetail, m.text("operation.profile_saved", nil), nil
		return m, nil
	case profileStepAcceptedMsg:
		m.profileDraftName = msg.name
		m.beginProfileConnectionStep()
		m.refreshWizardViewport()
		return m, nil
	case profileConnectionAcceptedMsg:
		changedEndpoint := m.connectionDraft.host != msg.host || m.connectionDraft.port != msg.port
		m.connectionDraft = profileConnectionDraft{host: msg.host, username: msg.username, port: msg.port}
		if changedEndpoint {
			m.resetProfileIdentityStep()
		}
		m.beginProfileIdentityStep()
		m.refreshWizardViewport()
		return m, nil
	case profileIdentityInspectionMsg:
		if msg.request != m.identityRequest || m.identityPhase != profileIdentityLoading || msg.host != m.connectionDraft.host || msg.port != m.connectionDraft.port {
			return m, nil
		}
		m.identityCancel = nil
		if msg.err != nil {
			if hostidentity.SafeFailure(msg.err) == hostidentity.FailureCancelled {
				return m, nil
			}
			m.identityPhase, m.status, m.err = profileIdentityError, wizardFeedbackRow(wizardFeedback{kind: wizardFeedbackError, message: m.identityFailureText(hostidentity.SafeFailure(msg.err))}), nil
			m.refreshWizardViewport()
			return m, nil
		}
		m.identityCandidate, m.identityPhase, m.status, m.err = msg.candidate, profileIdentityReview, "", nil
		m.identityFocus = profileIdentityFocusTrust
		m.refreshWizardViewport()
		return m, nil
	case profileIdentityAcceptedMsg:
		if msg.request != m.identityRequest || m.identityPhase != profileIdentityReview || msg.candidate != m.identityCandidate {
			return m, nil
		}
		m.identityDraft = profileIdentityDraft{host: m.connectionDraft.host, port: m.connectionDraft.port, algorithm: msg.candidate.Algorithm, fingerprint: msg.candidate.Fingerprint, trustMethod: profile.HostKeyTrustTOFU}
		m.identityPhase, m.status = profileIdentityCompleted, wizardFeedbackRow(wizardFeedback{kind: wizardFeedbackOK, message: m.text("wizard.identity.completed", nil)})
		m.refreshWizardViewport()
		return m, nil
	case mapepireProbeMsg:
		m.mapepireResolution, m.status, m.err = msg.resolution, "", msg.err
		m.refreshWizardViewport()
		return m, nil
	case operationMsg:
		m.status, m.err = m.operationText(msg.code), msg.err
		if msg.err != nil && (msg.code == operationLoadFailed || msg.code == operationRefreshFailed) {
			m.profilesLoaded, m.profilesLoadFailed = false, true
		}
		if msg.err == nil {
			m.screen = screenList
			return m, m.reload()
		}
		return m, nil
	case tea.KeyMsg:
		if m.screen == screenProfileStep || m.screen == screenProfileConnection || m.screen == screenProfileIdentity || m.screen == screenProfileMapepire {
			switch msg.String() {
			case "up", "k":
				m.wizardViewport.LineUp(1)
				return m, nil
			case "down", "j":
				m.wizardViewport.LineDown(1)
				return m, nil
			case "pgup":
				m.wizardViewport.ViewUp()
				return m, nil
			case "pgdown":
				m.wizardViewport.ViewDown()
				return m, nil
			}
		}
		if m.screen == screenList || m.screen == screenDetail || m.screen == screenForm || m.screen == screenConfirm {
			switch msg.String() {
			case "pgup":
				m.legacyViewport.LineUp(max(m.legacyViewport.Height, 1))
				return m, nil
			case "pgdown":
				m.legacyViewport.LineDown(max(m.legacyViewport.Height, 1))
				return m, nil
			}
		}
		if m.screen == screenSecurity && m.security != nil {
			if (msg.String() == "esc" || msg.String() == "b") && m.security.screen == securityMenu {
				m.screen = screenDetail
				return m, nil
			}
			m.security.noColor = m.noColor
			updated, cmd := m.security.Update(msg)
			security := updated.(SecurityModel)
			m.security = &security
			return m, cmd
		}
		if m.screen == screenProfileStep {
			updated, cmd := m.updateProfileStep(msg)
			wizard := updated.(Model)
			wizard.refreshWizardViewport()
			return wizard, cmd
		}
		if m.screen == screenProfileConnection {
			updated, cmd := m.updateProfileConnectionStep(msg)
			wizard := updated.(Model)
			wizard.refreshWizardViewport()
			return wizard, cmd
		}
		if m.screen == screenProfileIdentity {
			updated, cmd := m.updateProfileIdentityStep(msg)
			wizard := updated.(Model)
			wizard.refreshWizardViewport()
			return wizard, cmd
		}
		if m.screen == screenProfileMapepire {
			updated, cmd := m.updateProfileMapepireStep(msg)
			wizard := updated.(Model)
			wizard.refreshWizardViewport()
			return wizard, cmd
		}
		if m.screen == screenForm {
			updated, cmd := m.updateForm(msg)
			legacy := updated.(Model)
			legacy.refreshLegacyViewport()
			return legacy, cmd
		}
		updated, cmd := m.updateShell(msg)
		legacy := updated.(Model)
		legacy.refreshLegacyViewport()
		return legacy, cmd
	}
	return m, nil
}

func (m Model) updateShell(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch m.screen {
	case screenHome:
		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.status, m.err = m.text("home.help", nil), nil
		case "tab":
			if m.readinessCanScroll() {
				if m.homeFocus == homeFocusReadiness {
					m.homeFocus = homeFocusMenu
				} else {
					m.homeFocus = homeFocusReadiness
				}
			}
		case "esc":
			m.homeFocus = homeFocusMenu
		case "up", "k":
			if m.homeFocus == homeFocusReadiness {
				m.readinessOffset = m.moveReadinessOffset(-1)
			} else {
				m.homeSelected = m.moveHomeSelection(-1)
				m.menuOffset = m.visibleMenuWindow().Start
			}
		case "down", "j":
			if m.homeFocus == homeFocusReadiness {
				m.readinessOffset = m.moveReadinessOffset(1)
			} else {
				m.homeSelected = m.moveHomeSelection(1)
				m.menuOffset = m.visibleMenuWindow().Start
			}
		case "enter":
			action := m.selectedHomeAction()
			if !action.available {
				m.status, m.err = m.text("home.unavailable", map[string]any{"Action": action.label}), nil
				return m, nil
			}
			switch action.id {
			case actionManage:
				m.screen, m.status = screenList, ""
				return m, m.reload()
			case actionCreate:
				m.beginProfileStep()
			case actionExit:
				return m, tea.Quit
			}
		}
	case screenList:
		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc", "b":
			m.screen = screenHome
		case "up", "k":
			m.selected = clamp(m.selected-1, len(m.profiles))
		case "down", "j":
			m.selected = clamp(m.selected+1, len(m.profiles))
		case "n":
			m.beginForm(profile.Profile{}, screenList)
		case "enter":
			if len(m.profiles) > 0 {
				m.screen = screenDetail
			}
		case "d":
			if len(m.profiles) > 0 {
				m.confirm = m.profiles[m.selected].Name
				m.confirmInput = m.newConfirmationInput()
				m.screen = screenConfirm
			}
		}
	case screenDetail:
		switch key {
		case "esc", "b":
			m.screen = screenList
		case "e":
			if len(m.profiles) > 0 {
				m.beginForm(m.profiles[m.selected], screenDetail)
			}
		case "d":
			if len(m.profiles) > 0 {
				m.confirm = m.profiles[m.selected].Name
				m.confirmInput = m.newConfirmationInput()
				m.screen = screenConfirm
			}
		case "s":
			if len(m.profiles) > 0 && m.security != nil {
				security := NewSecurityModelWithLocalizer(m.security.ctx, m.profiles[m.selected].Name, m.security.services, m.localizer)
				security.noColor = m.noColor
				m.security, m.screen = &security, screenSecurity
			}
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case screenConfirm:
		switch key {
		case "esc", "n", "b":
			m.screen = screenList
		case "enter":
			name := m.confirm
			if strings.TrimSpace(m.confirmInput.Value()) != "delete "+name {
				return m, nil
			}
			return m, func() tea.Msg {
				_, err := m.store.Delete(name, profile.DeleteConfirmation("delete "+name))
				if err != nil {
					return operationMsg{code: operationProfileDeleted, err: err}
				}
				return operationMsg{code: operationProfileDeleted}
			}
		}
		if m.screen == screenConfirm {
			var cmd tea.Cmd
			m.confirmInput, cmd = m.confirmInput.Update(msg)
			return m, cmd
		}
	case screenSecurity:
		// Security child input is handled before shell dispatch.
	}
	return m, nil
}

func (m *Model) beginProfileStep() {
	m.profileName = newProfileNameInput()
	m.profileFocus = profileFocusName
	m.profileDraftName = ""
	m.resetProfileConnectionStep()
	m.resetProfileIdentityStep()
	m.status, m.err = "", nil
	m.screen = screenProfileStep
	m.wizardViewport.SetYOffset(0)
	m.refreshWizardViewport()
}

func (m *Model) resetProfileIdentityStep() {
	if m.identityCancel != nil {
		m.identityCancel()
	}
	m.identityFocus = profileIdentityFocusInspect
	m.identityPhase = profileIdentityAuthorize
	m.identityCandidate = hostidentity.Candidate{}
	m.identityDraft = profileIdentityDraft{}
	m.identityCancel = nil
	m.identityRequest++
}
func (m *Model) beginProfileIdentityStep() {
	if m.identityPhase == profileIdentityAuthorize {
		m.identityFocus = profileIdentityFocusInspect
	}
	m.status, m.err, m.screen = "", nil, screenProfileIdentity
	m.wizardViewport.SetYOffset(0)
	m.refreshWizardViewport()
}

// resetProfileConnectionStep is called only when Home starts a genuinely new
// wizard. Direct Step 1/Step 2 navigation deliberately retains this state.
func (m *Model) resetProfileConnectionStep() {
	m.connectionHost = textinput.Model{}
	m.connectionUsername = textinput.Model{}
	m.connectionPort = textinput.Model{}
	m.connectionFocus = profileConnectionFocusHost
	m.connectionDraft = profileConnectionDraft{}
	m.connectionReady = false
	m.connectionValidate = false
}

func (m *Model) beginProfileConnectionStep() {
	if !m.connectionReady {
		m.connectionHost = newProfileConnectionInput(253)
		m.connectionUsername = newProfileConnectionInput(128)
		m.connectionPort = newProfileConnectionInput(5)
		m.connectionPort.SetValue("22")
		m.connectionReady = true
	}
	m.connectionFocus = profileConnectionFocusHost
	m.focusProfileConnectionInput()
	m.connectionValidate = false
	m.status, m.err = "", nil
	m.screen = screenProfileConnection
	m.wizardViewport.SetYOffset(0)
	m.refreshWizardViewport()
}

func (m Model) selectedHomeAction() homeAction {
	for _, action := range m.visibleHomeActions() {
		if action.id == m.homeSelected {
			return action
		}
	}
	return m.visibleHomeActions()[0]
}

func (m Model) moveHomeSelection(delta int) homeActionID {
	actions := m.visibleHomeActions()
	index := 0
	for i, action := range actions {
		if action.id == m.homeSelected {
			index = i
			break
		}
	}
	return actions[clamp(index+delta, len(actions))].id
}

func (m Model) clampHomeSelection() homeActionID {
	return m.selectedHomeAction().id
}

func (m Model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "esc" || key == "ctrl+c" {
		m.screen = m.formReturn
		return m, nil
	}
	if key == "tab" || key == "down" {
		return m.focusField(1)
	}
	if key == "shift+tab" || key == "up" {
		return m.focusField(-1)
	}
	if key == "enter" {
		p, err := m.formProfile()
		if err != nil {
			m.err = err
			return m, nil
		}
		if m.formEdit == "" {
			return m, func() tea.Msg {
				_, err := m.store.Save(p)
				return operationMsg{code: operationProfileCreated, err: err}
			}
		}
		previous := m.formEdit
		return m, func() tea.Msg {
			_, err := m.store.Update(p, previous)
			return operationMsg{code: operationProfileUpdated, err: err}
		}
	}
	index := m.focusIndex()
	if index >= 0 {
		var cmd tea.Cmd
		m.form[index].input, cmd = m.form[index].input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) focusField(delta int) (tea.Model, tea.Cmd) {
	index := m.focusIndex()
	if index < 0 {
		index = 0
	}
	index = (index + delta + len(m.form)) % len(m.form)
	for i := range m.form {
		m.form[i].input.Blur()
	}
	cmd := m.form[index].input.Focus()
	return m, cmd
}

func (m Model) focusIndex() int {
	for i := range m.form {
		if m.form[i].input.Focused() {
			return i
		}
	}
	return -1
}

func (m *Model) beginForm(p profile.Profile, returnTo screen) {
	m.form = m.newFields(p)
	m.formEdit = p.Name
	m.formReturn = returnTo
	if p.Name == "" {
		m.formEdit = ""
	}
	m.screen = screenForm
	m.err = nil
}

func (m Model) newConfirmationInput() textinput.Model {
	input := textinput.New()
	input.Prompt = m.text("common.confirmation", nil) + ": "
	input.CharLimit = 256
	input.Focus()
	return input
}

// newConfirmationInput remains for package tests that construct legacy state.
func newConfirmationInput() textinput.Model { return NewModel(nil).newConfirmationInput() }

func (m Model) formProfile() (profile.Profile, error) {
	values := make([]string, len(m.form))
	for i := range m.form {
		values[i] = strings.TrimSpace(m.form[i].input.Value())
	}
	port, err := strconv.Atoi(values[2])
	if err != nil {
		return profile.Profile{}, fmt.Errorf("%s", m.text("error.port_number", nil))
	}
	trust, err := m.parseTrust(values[5])
	if err != nil {
		return profile.Profile{}, err
	}
	mode, err := m.parseCredentialMode(values[8])
	if err != nil {
		return profile.Profile{}, err
	}
	p := profile.Profile{Name: values[0], Host: values[1], Port: port, Username: values[3], HostKeyFingerprint: values[4], HostKeyTrust: trust, JavaHome: values[6], MapepireJAR: values[7], CredentialMode: mode}
	if err := p.Validate(); err != nil {
		return profile.Profile{}, err
	}
	return p, nil
}

func (m Model) trustDisplay(value profile.HostKeyTrust) string {
	if value == profile.HostKeyTrustTOFU {
		return m.text("domain.trust.tofu", nil)
	}
	return m.text("domain.trust.verified", nil)
}
func (m Model) credentialModeDisplay(value profile.CredentialMode) string {
	if value == profile.CredentialModeVault {
		return m.text("domain.credential.vault", nil)
	}
	return m.text("domain.credential.prompt", nil)
}
func (m Model) parseTrust(value string) (profile.HostKeyTrust, error) {
	if value == m.text("domain.trust.tofu", nil) {
		return profile.HostKeyTrustTOFU, nil
	}
	if value == m.text("domain.trust.verified", nil) {
		return profile.HostKeyTrustVerified, nil
	}
	return "", fmt.Errorf("unsupported trust value")
}
func (m Model) parseCredentialMode(value string) (profile.CredentialMode, error) {
	if value == m.text("domain.credential.vault", nil) {
		return profile.CredentialModeVault, nil
	}
	if value == m.text("domain.credential.prompt", nil) {
		return profile.CredentialModePrompt, nil
	}
	return "", fmt.Errorf("unsupported credential mode")
}

func (m Model) operationText(code operationCode) string {
	switch code {
	case operationLoadFailed:
		return m.text("operation.load_failed", nil)
	case operationRefreshFailed:
		return m.text("operation.refresh_failed", nil)
	case operationProfileSaved:
		return m.text("operation.profile_saved", nil)
	case operationProfileCreated:
		return m.text("operation.profile_created", nil)
	case operationProfileUpdated:
		return m.text("operation.profile_updated", nil)
	case operationProfileDeleted:
		return m.text("operation.profile_deleted", nil)
	default:
		panic("unknown operation code")
	}
}

func (m Model) View() string {
	var b strings.Builder
	title := lipgloss.NewStyle().Bold(true).Render(m.text("legacy.title", nil))
	b.WriteString(title + "\n\n")
	switch m.screen {
	case screenHome:
		return m.renderHome()
	case screenList:
		b.WriteString(m.text("legacy.list.heading", nil) + "\n")
		if len(m.profiles) == 0 {
			b.WriteString(m.text("legacy.list.empty", nil) + "\n")
		}
		for i, p := range m.profiles {
			marker := " "
			if i == m.selected {
				marker = ">"
			}
			fmt.Fprintf(&b, "%s %s\n", marker, p.Name)
		}
		b.WriteString("\n" + m.text("legacy.list.footer", nil) + "\n")
	case screenDetail:
		p := m.profiles[m.selected]
		fmt.Fprintf(&b, "%s: %s\n%s: %s:%d\n%s: %s\n%s: %s\n\n%s\n", m.text("legacy.detail.profile", nil), p.Name, m.text("legacy.detail.host", nil), p.Host, p.Port, m.text("legacy.detail.username", nil), p.Username, m.text("legacy.detail.trust", nil), m.trustDisplay(p.HostKeyTrust), m.text("legacy.detail.footer", nil))
	case screenForm:
		b.WriteString(m.text("form.fields", nil) + "\n")
		for i := range m.form {
			fmt.Fprintf(&b, "%s: %s\n", m.form[i].label, m.form[i].input.View())
		}
		b.WriteString("\n" + m.text("form.footer", nil) + "\n")
	case screenProfileStep:
		return m.renderProfileStep()
	case screenProfileConnection:
		return m.renderProfileConnectionStep()
	case screenProfileIdentity:
		return m.renderProfileIdentityStep()
	case screenProfileMapepire:
		return m.renderProfileMapepireStep()
	case screenConfirm:
		fmt.Fprintf(&b, "%s\n%s\n%s\n%s\n%s\n", m.text("legacy.confirm.delete", map[string]any{"Name": m.confirm}), m.text("legacy.confirm.retains_backup", nil), m.text("legacy.confirm.type_delete", map[string]any{"Name": m.confirm}), m.confirmInput.View(), m.text("legacy.confirm.cancel", nil))
	case screenSecurity:
		if m.security != nil {
			security := *m.security
			security.noColor = m.noColor
			return security.View()
		}
	}
	if m.status != "" {
		b.WriteString("\n" + m.text("common.status", nil) + ": " + m.status + "\n")
	}
	if m.err != nil {
		b.WriteString(m.text("common.error", nil) + ": " + sanitizeError(m.err) + "\n")
	}
	return m.renderLegacyViewport(b.String())
}

func responsive(view string, width, height int) string {
	if width > 0 && width < 40 {
		view = strings.ReplaceAll(view, "  enter inspect", "\nenter inspect")
		view = strings.ReplaceAll(view, "  d delete", "\nd delete")
	}
	return view
}

func (m Model) renderLegacyViewport(content string) string {
	width, height := m.width, m.height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		lines = append(lines, wrapWizardText(line, width, "")...)
	}
	text := strings.Join(lines, "\n")
	v := m.legacyViewport
	if v.Width != width || v.Height != max(height-1, 1) || m.legacyViewportText != text {
		v.Width, v.Height = width, max(height-1, 1)
		v.SetContent(text)
	}
	return strings.TrimRight(v.View(), "\n") + "\n" + wizardOverflowIndicator(v, width, newHomeTheme(m.noColor), m.text("overflow.above", nil), m.text("overflow.below", nil))
}

// refreshLegacyViewport preserves a real Bubbles viewport between updates.
// Legacy screens intentionally keep their existing keyboard bindings; PgUp and
// PgDown are the non-conflicting manual navigation surface.
func (m *Model) refreshLegacyViewport() {
	if m.screen != screenList && m.screen != screenDetail && m.screen != screenForm && m.screen != screenConfirm {
		return
	}
	width, height := m.width, m.height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	content := m.legacyViewportContent()
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		lines = append(lines, wrapWizardText(line, width, "")...)
	}
	m.legacyViewport.Width, m.legacyViewport.Height = width, max(height-1, 1)
	m.legacyViewportText = strings.Join(lines, "\n")
	m.legacyViewport.SetContent(m.legacyViewportText)
	start, end := m.legacyFocusRange()
	if end-start+1 > m.legacyViewport.Height {
		m.legacyViewport.SetYOffset(start)
	} else if start < m.legacyViewport.YOffset {
		m.legacyViewport.SetYOffset(start)
	} else if end >= m.legacyViewport.YOffset+m.legacyViewport.Height {
		m.legacyViewport.SetYOffset(max(end-m.legacyViewport.Height+1, 0))
	}
}

// legacyFocusRange is a semantic range, deliberately derived from the stable
// legacy screen layout rather than rendered ANSI text.
func (m Model) legacyFocusRange() (int, int) {
	width := m.width
	if width <= 0 {
		width = 80
	}
	switch m.screen {
	case screenList:
		start := 3 // title, blank line, and the Profiles heading.
		for i := 0; i < m.selected && i < len(m.profiles); i++ {
			start += len(wrapWizardText("  "+m.profiles[i].Name, width, ""))
		}
		if m.selected >= len(m.profiles) {
			return start, start
		}
		return start, start + len(wrapWizardText("> "+m.profiles[m.selected].Name, width, "")) - 1
	case screenForm:
		index := m.focusIndex()
		if index < 0 {
			index = 0
		}
		start := 3 // title, blank line, and the Profile fields heading.
		for i := 0; i < index && i < len(m.form); i++ {
			start += len(wrapWizardText(m.form[i].label+": "+m.form[i].input.View(), width, ""))
		}
		if index >= len(m.form) {
			return start, start
		}
		return start, start + len(wrapWizardText(m.form[index].label+": "+m.form[index].input.View(), width, "")) - 1
	case screenConfirm:
		start := 2
		for _, line := range []string{
			m.text("legacy.confirm.delete", map[string]any{"Name": m.confirm}),
			m.text("legacy.confirm.retains_backup", nil),
			m.text("legacy.confirm.type_delete", map[string]any{"Name": m.confirm}),
		} {
			start += len(wrapWizardText(line, width, ""))
		}
		return start, start + len(wrapWizardText(m.confirmInput.View(), width, "")) - 1
	default:
		return 0, 0
	}
}

func (m Model) legacyViewportContent() string {
	var b strings.Builder
	title := lipgloss.NewStyle().Bold(true).Render(m.text("legacy.title", nil))
	b.WriteString(title + "\n\n")
	switch m.screen {
	case screenList:
		b.WriteString(m.text("legacy.list.heading", nil) + "\n")
		if len(m.profiles) == 0 {
			b.WriteString(m.text("legacy.list.empty", nil) + "\n")
		}
		for i, p := range m.profiles {
			marker := " "
			if i == m.selected {
				marker = ">"
			}
			fmt.Fprintf(&b, "%s %s\n", marker, p.Name)
		}
		b.WriteString("\n" + m.text("legacy.list.footer", nil) + "\n")
	case screenDetail:
		p := m.profiles[m.selected]
		fmt.Fprintf(&b, "%s: %s\n%s: %s:%d\n%s: %s\n%s: %s\n\n%s\n", m.text("legacy.detail.profile", nil), p.Name, m.text("legacy.detail.host", nil), p.Host, p.Port, m.text("legacy.detail.username", nil), p.Username, m.text("legacy.detail.trust", nil), m.trustDisplay(p.HostKeyTrust), m.text("legacy.detail.footer", nil))
	case screenForm:
		b.WriteString(m.text("form.fields", nil) + "\n")
		for i := range m.form {
			fmt.Fprintf(&b, "%s: %s\n", m.form[i].label, m.form[i].input.View())
		}
		b.WriteString("\n" + m.text("form.footer", nil) + "\n")
	case screenConfirm:
		fmt.Fprintf(&b, "%s\n%s\n%s\n%s\n%s\n", m.text("legacy.confirm.delete", map[string]any{"Name": m.confirm}), m.text("legacy.confirm.retains_backup", nil), m.text("legacy.confirm.type_delete", map[string]any{"Name": m.confirm}), m.confirmInput.View(), m.text("legacy.confirm.cancel", nil))
	}
	if m.status != "" {
		b.WriteString("\n" + m.text("common.status", nil) + ": " + m.status + "\n")
	}
	if m.err != nil {
		b.WriteString(m.text("common.error", nil) + ": " + sanitizeError(m.err) + "\n")
	}
	return b.String()
}

func sanitizeError(err error) string { return strings.Join(strings.Fields(err.Error()), " ") }

func (m Model) reload() tea.Cmd {
	return func() tea.Msg {
		profiles, err := m.store.List(profileLimit)
		if err != nil {
			return operationMsg{code: operationRefreshFailed, err: err}
		}
		return profilesMsg{profiles: profiles}
	}
}
func clamp(value, length int) int {
	if length <= 0 {
		return 0
	}
	if value < 0 {
		return 0
	}
	if value >= length {
		return length - 1
	}
	return value
}
func replaceProfile(list []profile.Profile, p profile.Profile) []profile.Profile {
	for i := range list {
		if list[i].Name == p.Name {
			list[i] = p
			return list
		}
	}
	return append(list, p)
}

// Run starts the local terminal program. It never creates an MCP server or
// writes client configuration files.
func Run(ctx context.Context, store configuration.ProfilesStore, buildInfo BuildInfo) error {
	return RunWithHostIdentityInspector(ctx, store, buildInfo, nil)
}

func RunWithHostIdentityInspector(ctx context.Context, store configuration.ProfilesStore, buildInfo BuildInfo, inspector hostidentity.Inspector) error {
	program := tea.NewProgram(newModelWithIdentityInspector(store, buildInfo, inspector, ctx, identityInspectionTimeout), tuiProgramOptions(ctx)...)
	_, err := program.Run()
	return err
}

type tuiProgramConfig struct {
	altScreen bool
}

func defaultTUIProgramConfig() tuiProgramConfig { return tuiProgramConfig{altScreen: true} }

func tuiProgramOptions(ctx context.Context) []tea.ProgramOption {
	options := []tea.ProgramOption{tea.WithContext(ctx)}
	if defaultTUIProgramConfig().altScreen {
		options = append(options, tea.WithAltScreen())
	}
	return options
}
