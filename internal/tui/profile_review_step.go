package tui

import (
	"context"
	"crypto/sha256"
	"fmt"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
)

type profileCreator interface {
	Create(context.Context, configuration.CreateProfileRequest) (configuration.CreateProfileResult, error)
}
type profileReviewFocus uint8

const profileReviewFocusSave profileReviewFocus = iota

type profileCreateMsg struct {
	request    string
	generation uint64
	result     configuration.CreateProfileResult
	err        error
}

func (m Model) reviewProfile() profile.Profile {
	return profile.Profile{SchemaVersion: profile.SchemaVersionV3, Name: m.profileDraftName, Host: m.connectionDraft.host, Port: m.connectionDraft.port, Username: m.connectionDraft.username, HostKeyFingerprint: m.identityDraft.fingerprint, HostKeyTrust: m.identityDraft.trustMethod, CredentialMode: m.credentialMode}
}
func (m Model) updateProfileReviewStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.screen = screenProfileCredentials
		return m, nil
	}
	if msg.String() != "enter" || m.createPending {
		return m, nil
	}
	if m.profileCreator == nil {
		m.status = "[ERR] Profile creation is unavailable"
		return m, nil
	}
	p := m.reviewProfile()
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%#v", p))))
	m.createGeneration++
	request := fmt.Sprintf("profile-create-%d", m.createGeneration)
	ctx, cancel := context.WithCancel(context.Background())
	m.createPending, m.createRequest, m.createCancel = true, request, cancel
	return m, func() tea.Msg {
		result, err := m.profileCreator.Create(ctx, configuration.CreateProfileRequest{RequestID: request, Generation: m.createGeneration, DraftDigest: digest, Profile: p})
		return profileCreateMsg{request: request, generation: m.createGeneration, result: result, err: err}
	}
}
func (m Model) renderProfileReviewStep() string {
	fw, fh := m.shellFrameDimensions()
	w, h := m.shellInnerWidth(fw), m.shellInnerHeight(fh)
	t := newHomeTheme(m.noColor)
	return m.renderWizardShell(m.renderProfileConnectionHeader(w, t), renderFooterText(w, t, "Enter: save • Esc: back", m.buildInfo), m.renderProfileReviewPanel(w, h, t))
}

func (m Model) renderProfileReviewPanel(w, h int, t homeTheme) string {
	panel := newWizardPanelLayout(w, h, t)
	p := m.reviewProfile()
	lines := append(renderWizardTitleRow(panel.contentWidth, t, "Review & Save", "Step 6 of 8"), "", renderWizardDivider(panel.contentWidth, t), "", fmt.Sprintf("Profile: %s", p.Name), fmt.Sprintf("Endpoint: %s:%d", p.Host, p.Port), fmt.Sprintf("Credentials: %s", p.CredentialMode), "", "▸ [ SAVE PROFILE ]")
	if m.status != "" {
		lines = append(lines, "", m.status)
	}
	return panel.render(w, lines)
}
