package tui

import (
	"bac-nexus/internal/credential"
	"bac-nexus/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
)

type credentialStatusChecker interface {
	Status(string) (credential.Presence, error)
}
type profileCredentialsFocus uint8

const (
	profileCredentialsFocusPrompt profileCredentialsFocus = iota
	profileCredentialsFocusKeyring
	profileCredentialsFocusContinue
)

type profileCredentialStatusMsg struct {
	request  uint64
	presence credential.Presence
}

func (m Model) updateProfileCredentialsStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.screen = screenHome
	case "tab":
		m.credentialFocus = (m.credentialFocus + 1) % 3
	case "shift+tab":
		m.credentialFocus = (m.credentialFocus + 2) % 3
	case "enter":
		switch m.credentialFocus {
		case profileCredentialsFocusPrompt:
			m.credentialMode = profile.CredentialModePrompt
		case profileCredentialsFocusKeyring:
			m.credentialMode = profile.CredentialModeKeyring
		case profileCredentialsFocusContinue:
			if m.credentialMode == profile.CredentialModePrompt {
				m.screen = screenProfileReview
				break
			}
			m.credentialRequest++
			request := m.credentialRequest
			return m, func() tea.Msg {
				if m.credentialStatus == nil {
					return profileCredentialStatusMsg{request: request, presence: credential.PresenceUnavailable}
				}
				presence, _ := m.credentialStatus.Status(m.profileDraftName)
				return profileCredentialStatusMsg{request: request, presence: presence}
			}
		}
	}
	return m, nil
}

func (m Model) renderProfileCredentialsStep() string {
	fw, fh := m.shellFrameDimensions()
	w, h := m.shellInnerWidth(fw), m.shellInnerHeight(fh)
	t := newHomeTheme(m.noColor)
	return m.renderWizardShell(m.renderProfileConnectionHeader(w, t), renderFooterText(w, t, "Tab: move • Enter: select • Esc: back", m.buildInfo), m.renderProfileCredentialsPanel(w, h, t))
}

func (m Model) renderProfileCredentialsPanel(w, h int, t homeTheme) string {
	panel := newWizardPanelLayout(w, h, t)
	lines := renderWizardTitleRow(panel.contentWidth, t, "Credentials", "Step 5 of 8")
	lines = append(lines, "", renderWizardDivider(panel.contentWidth, t), "", t.wizardContentHeading.Render("Credential handling"))
	for _, row := range []struct {
		focus profileCredentialsFocus
		mode  profile.CredentialMode
		text  string
	}{{profileCredentialsFocusPrompt, profile.CredentialModePrompt, "Prompt for each authorized operation"}, {profileCredentialsFocusKeyring, profile.CredentialModeKeyring, "Use secure native keyring"}} {
		marker := "( )"
		if m.credentialMode == row.mode {
			marker = "(*)"
		}
		prefix := "  "
		if m.credentialFocus == row.focus {
			prefix = "▸ "
		}
		lines = append(lines, prefix+marker+" "+row.text)
	}
	prefix := "  "
	if m.credentialFocus == profileCredentialsFocusContinue {
		prefix = "▸ "
	}
	lines = append(lines, "", prefix+"[ CONTINUE ]")
	if m.status != "" {
		lines = append(lines, "", m.status)
	}
	return panel.render(w, lines)
}
