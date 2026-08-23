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

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"bac-nexus/internal/configuration"
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
	text string
	err  error
}

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

type profileIdentityDecision uint8

const (
	profileIdentityNone profileIdentityDecision = iota
	profileIdentityKnownFingerprint
	profileIdentityObservedKey
)

type profileIdentityBranch uint8

const (
	profileIdentityBranchNone profileIdentityBranch = iota
	profileIdentityBranchFingerprint
	profileIdentityBranchObservedKey
)

type profileIdentityAcceptedMsg struct{ decision profileIdentityDecision }

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
	identityDecision   profileIdentityDecision
	identityBranch     profileIdentityBranch
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
}

// BuildInfo is the build identity supplied by the composition root. The TUI
// does not inspect Git state or execute external commands to derive it.
type BuildInfo struct {
	Version  string
	Revision string
}

func NewModel(store configuration.ProfilesStore) Model {
	return NewModelWithBuildInfo(store, BuildInfo{Version: "dev", Revision: "unknown"})
}

// NewModelWithBuildInfo constructs the Home model with build identity supplied
// by the caller. Empty values retain truthful local-build defaults.
func NewModelWithBuildInfo(store configuration.ProfilesStore, buildInfo BuildInfo) Model {
	if buildInfo.Version == "" {
		buildInfo.Version = "dev"
	}
	if buildInfo.Revision == "" {
		buildInfo.Revision = "unknown"
	}
	m := Model{store: store, screen: screenHome, homeSelected: actionCreate, noColor: noColorEnabled(), buildInfo: buildInfo, wizardViewport: viewport.New(1, 1), legacyViewport: viewport.New(1, 1)}
	m.form = newFields(profile.Profile{})
	m.profileName = newProfileNameInput()
	return m
}

func noColorEnabled() bool {
	value, present := os.LookupEnv("NO_COLOR")
	return present && value != ""
}

// NewModelWithSecurity enables the security child screen without changing
// the default profile-only constructor used by existing callers.
func NewModelWithSecurity(store configuration.ProfilesStore, ctx context.Context, services SecurityServices) Model {
	m := NewModel(store)
	m.security = ptrSecurityModel(NewSecurityModel(ctx, "", services))
	m.security.noColor = m.noColor
	return m
}

func ptrSecurityModel(model SecurityModel) *SecurityModel { return &model }

func newFields(p profile.Profile) []field {
	values := []struct{ label, value string }{
		{"Name", p.Name}, {"Host", p.Host}, {"Port", strconv.Itoa(p.Port)},
		{"Username", p.Username}, {"Fingerprint", p.HostKeyFingerprint},
		{"Trust (tofu/verified)", string(p.HostKeyTrust)}, {"Java home", p.JavaHome},
		{"Mapepire JAR", p.MapepireJAR}, {"Credential mode (vault/prompt)", string(p.CredentialMode)},
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
			return operationMsg{text: "Unable to load profiles", err: err}
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
		m.screen, m.status, m.err = screenDetail, "Profile saved", nil
		return m, nil
	case profileStepAcceptedMsg:
		m.profileDraftName = msg.name
		m.beginProfileConnectionStep()
		m.refreshWizardViewport()
		return m, nil
	case profileConnectionAcceptedMsg:
		m.connectionDraft = profileConnectionDraft{host: msg.host, username: msg.username, port: msg.port}
		m.beginProfileIdentityStep()
		m.refreshWizardViewport()
		return m, nil
	case profileIdentityAcceptedMsg:
		m.identityDecision = msg.decision
		if msg.decision == profileIdentityKnownFingerprint {
			m.identityBranch = profileIdentityBranchFingerprint
		} else {
			m.identityBranch = profileIdentityBranchObservedKey
		}
		m.refreshWizardViewport()
		return m, nil
	case operationMsg:
		m.status, m.err = msg.text, msg.err
		if msg.err != nil && (msg.text == "Unable to load profiles" || msg.text == "Unable to refresh profiles") {
			m.profilesLoaded, m.profilesLoadFailed = false, true
		}
		if msg.err == nil {
			m.screen = screenList
			return m, m.reload()
		}
		return m, nil
	case tea.KeyMsg:
		if m.screen == screenProfileStep || m.screen == screenProfileConnection || m.screen == screenProfileIdentity {
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
			m.status, m.err = "Ayuda: ↑/↓ navegar; Enter seleccionar; q salir.", nil
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
				m.status, m.err = action.label+": todavía no está disponible", nil
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
				m.confirmInput = newConfirmationInput()
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
				m.confirmInput = newConfirmationInput()
				m.screen = screenConfirm
			}
		case "s":
			if len(m.profiles) > 0 && m.security != nil {
				security := NewSecurityModel(m.security.ctx, m.profiles[m.selected].Name, m.security.services)
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
					return operationMsg{text: "Profile was not deleted", err: err}
				}
				return operationMsg{text: "Profile deleted"}
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
	m.identityFocus = profileIdentityFocusKnown
	m.identityDecision = profileIdentityNone
	m.identityBranch = profileIdentityBranchNone
}
func (m *Model) beginProfileIdentityStep() {
	m.identityFocus = profileIdentityFocusKnown
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
			return m, func() tea.Msg { _, err := m.store.Save(p); return operationMsg{text: "Profile created", err: err} }
		}
		previous := m.formEdit
		return m, func() tea.Msg {
			_, err := m.store.Update(p, previous)
			return operationMsg{text: "Profile updated", err: err}
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
	m.form = newFields(p)
	m.formEdit = p.Name
	m.formReturn = returnTo
	if p.Name == "" {
		m.formEdit = ""
	}
	m.screen = screenForm
	m.err = nil
}

func newConfirmationInput() textinput.Model {
	input := textinput.New()
	input.Prompt = "Confirmation: "
	input.CharLimit = 256
	input.Focus()
	return input
}

func (m Model) formProfile() (profile.Profile, error) {
	values := make([]string, len(m.form))
	for i := range m.form {
		values[i] = strings.TrimSpace(m.form[i].input.Value())
	}
	port, err := strconv.Atoi(values[2])
	if err != nil {
		return profile.Profile{}, fmt.Errorf("port must be a number")
	}
	p := profile.Profile{Name: values[0], Host: values[1], Port: port, Username: values[3], HostKeyFingerprint: values[4], HostKeyTrust: profile.HostKeyTrust(values[5]), JavaHome: values[6], MapepireJAR: values[7], CredentialMode: profile.CredentialMode(values[8])}
	if err := p.Validate(); err != nil {
		return profile.Profile{}, err
	}
	return p, nil
}

func (m Model) View() string {
	var b strings.Builder
	title := lipgloss.NewStyle().Bold(true).Render("Nexus configuration")
	b.WriteString(title + "\n\n")
	switch m.screen {
	case screenHome:
		return m.renderHome()
	case screenList:
		b.WriteString("Profiles\n")
		if len(m.profiles) == 0 {
			b.WriteString("No profiles configured. Press n to create one.\n")
		}
		for i, p := range m.profiles {
			marker := " "
			if i == m.selected {
				marker = ">"
			}
			fmt.Fprintf(&b, "%s %s\n", marker, p.Name)
		}
		b.WriteString("\nn new  enter inspect  d delete  b inicio  q quit\n")
	case screenDetail:
		p := m.profiles[m.selected]
		fmt.Fprintf(&b, "Profile: %s\nHost: %s:%d\nUsername: %s\nTrust: %s\n\ne edit  d delete  b back\n", p.Name, p.Host, p.Port, p.Username, p.HostKeyTrust)
	case screenForm:
		b.WriteString("Profile fields\n")
		for i := range m.form {
			fmt.Fprintf(&b, "%s: %s\n", m.form[i].label, m.form[i].input.View())
		}
		b.WriteString("\nenter save  esc cancel\n")
	case screenProfileStep:
		return m.renderProfileStep()
	case screenProfileConnection:
		return m.renderProfileConnectionStep()
	case screenProfileIdentity:
		return m.renderProfileIdentityStep()
	case screenConfirm:
		fmt.Fprintf(&b, "Delete profile %q?\nThis retains a recoverable backup.\nType delete %s exactly, then press enter.\n%s\nn/esc cancel\n", m.confirm, m.confirm, m.confirmInput.View())
	case screenSecurity:
		if m.security != nil {
			security := *m.security
			security.noColor = m.noColor
			return security.View()
		}
	}
	if m.status != "" {
		b.WriteString("\nStatus: " + m.status + "\n")
	}
	if m.err != nil {
		b.WriteString("Error: " + sanitizeError(m.err) + "\n")
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
	return strings.TrimRight(v.View(), "\n") + "\n" + wizardOverflowIndicator(v, width, newHomeTheme(m.noColor))
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
			fmt.Sprintf("Delete profile %q?", m.confirm),
			"This retains a recoverable backup.",
			fmt.Sprintf("Type delete %s exactly, then press enter.", m.confirm),
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
	title := lipgloss.NewStyle().Bold(true).Render("Nexus configuration")
	b.WriteString(title + "\n\n")
	switch m.screen {
	case screenList:
		b.WriteString("Profiles\n")
		if len(m.profiles) == 0 {
			b.WriteString("No profiles configured. Press n to create one.\n")
		}
		for i, p := range m.profiles {
			marker := " "
			if i == m.selected {
				marker = ">"
			}
			fmt.Fprintf(&b, "%s %s\n", marker, p.Name)
		}
		b.WriteString("\nn new  enter inspect  d delete  b inicio  q quit\n")
	case screenDetail:
		p := m.profiles[m.selected]
		fmt.Fprintf(&b, "Profile: %s\nHost: %s:%d\nUsername: %s\nTrust: %s\n\ne edit  d delete  b back\n", p.Name, p.Host, p.Port, p.Username, p.HostKeyTrust)
	case screenForm:
		b.WriteString("Profile fields\n")
		for i := range m.form {
			fmt.Fprintf(&b, "%s: %s\n", m.form[i].label, m.form[i].input.View())
		}
		b.WriteString("\nenter save  esc cancel\n")
	case screenConfirm:
		fmt.Fprintf(&b, "Delete profile %q?\nThis retains a recoverable backup.\nType delete %s exactly, then press enter.\n%s\nn/esc cancel\n", m.confirm, m.confirm, m.confirmInput.View())
	}
	if m.status != "" {
		b.WriteString("\nStatus: " + m.status + "\n")
	}
	if m.err != nil {
		b.WriteString("Error: " + sanitizeError(m.err) + "\n")
	}
	return b.String()
}

func sanitizeError(err error) string { return strings.Join(strings.Fields(err.Error()), " ") }

func (m Model) reload() tea.Cmd {
	return func() tea.Msg {
		profiles, err := m.store.List(profileLimit)
		if err != nil {
			return operationMsg{text: "Unable to refresh profiles", err: err}
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
	program := tea.NewProgram(NewModelWithBuildInfo(store, buildInfo), tuiProgramOptions(ctx)...)
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
