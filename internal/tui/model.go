// Package tui contains the optional local configuration adapter. It owns
// terminal state only; profile validation and persistence remain lower-layer
// responsibilities.
package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/profile"
)

const profileLimit = 128

type screen uint8

const (
	screenList screen = iota
	screenDetail
	screenForm
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

type field struct {
	label string
	input textinput.Model
}

// Model is the deterministic shell model. It contains profile metadata only;
// credential material is intentionally not accepted by this adapter.
type Model struct {
	store        profileStore
	screen       screen
	profiles     []profile.Profile
	selected     int
	form         []field
	formEdit     string
	confirm      string
	confirmInput textinput.Model
	width        int
	height       int
	noColor      bool
	status       string
	err          error
	security     *SecurityModel
}

func NewModel(store configuration.ProfilesStore) Model {
	m := Model{store: store, screen: screenList, noColor: true}
	m.form = newFields(profile.Profile{})
	return m
}

// NewModelWithSecurity enables the security child screen without changing
// the default profile-only constructor used by existing callers.
func NewModelWithSecurity(store configuration.ProfilesStore, ctx context.Context, services SecurityServices) Model {
	m := NewModel(store)
	m.security = ptrSecurityModel(NewSecurityModel(ctx, "", services))
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
		m.noColor = true
		return m, nil
	case profilesMsg:
		m.profiles, m.err = append([]profile.Profile(nil), msg.profiles...), nil
		m.selected = clamp(m.selected, len(m.profiles))
		return m, nil
	case profileMsg:
		m.profiles = replaceProfile(m.profiles, msg.profile)
		m.screen, m.status, m.err = screenDetail, "Profile saved", nil
		return m, nil
	case operationMsg:
		m.status, m.err = msg.text, msg.err
		if msg.err == nil {
			m.screen = screenList
			return m, m.reload()
		}
		return m, nil
	case tea.KeyMsg:
		if m.screen == screenSecurity && m.security != nil {
			if (msg.String() == "esc" || msg.String() == "b") && m.security.screen == securityMenu {
				m.screen = screenDetail
				return m, nil
			}
			updated, cmd := m.security.Update(msg)
			security := updated.(SecurityModel)
			m.security = &security
			return m, cmd
		}
		if m.screen == screenForm {
			return m.updateForm(msg)
		}
		return m.updateShell(msg)
	}
	return m, nil
}

func (m Model) updateShell(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch m.screen {
	case screenList:
		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			m.selected = clamp(m.selected-1, len(m.profiles))
		case "down", "j":
			m.selected = clamp(m.selected+1, len(m.profiles))
		case "n":
			m.beginForm(profile.Profile{})
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
				m.beginForm(m.profiles[m.selected])
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

func (m Model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "esc" || key == "ctrl+c" {
		m.screen = screenList
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

func (m *Model) beginForm(p profile.Profile) {
	m.form = newFields(p)
	m.formEdit = p.Name
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
		b.WriteString("\nn new  enter inspect  d delete  q quit\n")
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
	case screenSecurity:
		if m.security != nil {
			return m.security.View()
		}
	}
	if m.status != "" {
		b.WriteString("\nStatus: " + m.status + "\n")
	}
	if m.err != nil {
		b.WriteString("Error: " + sanitizeError(m.err) + "\n")
	}
	return responsive(b.String(), m.width, m.height)
}

func responsive(view string, width, height int) string {
	if width > 0 && width < 40 {
		view = strings.ReplaceAll(view, "  enter inspect", "\nenter inspect")
		view = strings.ReplaceAll(view, "  d delete", "\nd delete")
	}
	if height > 0 {
		lines := strings.Split(view, "\n")
		if len(lines) > height {
			view = strings.Join(lines[:height], "\n")
		}
	}
	return view
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
func Run(ctx context.Context, store configuration.ProfilesStore) error {
	program := tea.NewProgram(NewModel(store), tea.WithContext(ctx))
	_, err := program.Run()
	return err
}
