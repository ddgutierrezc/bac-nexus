package tui

import (
	"context"
	"strconv"
	"time"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
)

const profileProofTimeout = 30 * time.Second

type profileProofState struct {
	enabled    bool
	request    string
	generation uint64
	running    bool
	cancel     context.CancelFunc
	ticket     string
	class      configuration.Step8Reason
}

type profileProofMsg struct {
	request    string
	generation uint64
	result     configuration.Step8Result
}

func (m Model) startProfileProof(p profile.Profile, ssh bool) (tea.Model, tea.Cmd) {
	if m.profileProof.cancel != nil {
		m.profileProof.cancel()
	}
	if !ssh {
		m.profileProof.generation++
		m.profileProof.request = "profile-proof-" + strconv.FormatUint(m.profileProof.generation, 10)
	}
	request := m.profileProof.request
	ctx, cancel := context.WithTimeout(context.Background(), profileProofTimeout)
	m.profileProof.enabled, m.profileProof.running, m.profileProof.cancel = true, true, cancel
	return m, func() tea.Msg {
		if m.step8Runner == nil {
			return profileProofMsg{request: request, generation: m.profileProof.generation, result: configuration.Step8Result{Class: configuration.ResultDowngradeBlocked}}
		}
		result := m.step8Runner.Run(ctx, configuration.Step8Request{RequestID: request, Generation: m.profileProof.generation, Profile: p, WSSConsent: !ssh, SSHConsent: ssh, FallbackTicket: m.profileProof.ticket, FallbackClass: m.profileProof.class})
		return profileProofMsg{request: request, generation: m.profileProof.generation, result: result}
	}
}

func (m *Model) applyProfileProof(msg profileProofMsg) {
	if !m.profileProof.running || msg.request != m.profileProof.request || msg.generation != m.profileProof.generation {
		return
	}
	m.profileProof.running = false
	if m.profileProof.cancel != nil {
		m.profileProof.cancel()
	}
	m.profileProof.cancel = nil
	if msg.result.Decision == configuration.DecisionSSHEligible && msg.result.FallbackTicket != "" {
		m.profileProof.ticket, m.profileProof.class = msg.result.FallbackTicket, msg.result.FallbackClass
		return
	}
	m.screen = screenProfileCompletion
	if msg.result.Class == configuration.ResultProofSuccess && msg.result.Cleanup {
		m.profileCompletion = profileCompletionSuccessful
		return
	}
	m.profileCompletion = profileCompletionFailed
}

func (m Model) updateProfileProofStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := m.step8Action.request.Profile
	switch msg.String() {
	case "o":
		if !m.profileProof.running {
			m.screen, m.profileCompletion = screenProfileCompletion, profileCompletionOmitted
		}
	case "esc", "ctrl+c":
		if m.profileProof.running {
			m.profileProof.cancel()
			m.profileProof.cancel, m.profileProof.running = nil, false
			m.screen, m.profileCompletion = screenProfileCompletion, profileCompletionCancelled
		}
	case "enter":
		if !m.profileProof.running && p.Name != "" {
			return m.startProfileProof(p, m.profileProof.ticket != "")
		}
	case "r":
		if p.Name != "" {
			return m.startProfileProof(p, false)
		}
	}
	return m, nil
}

func (m Model) renderProfileProofStep() string {
	fw, fh := m.shellFrameDimensions()
	w, h := m.shellInnerWidth(fw), m.shellInnerHeight(fh)
	t := newHomeTheme(m.noColor)
	return m.renderWizardShell(m.renderProfileConnectionHeader(w, t), renderFooterText(w, t, "Enter: consent and start • R: retry • O: omit • Esc: cancel", m.buildInfo), m.renderProfileProofPanel(w, h, t))
}

func (m Model) renderProfileProofPanel(w, h int, t homeTheme) string {
	panel := newWizardPanelLayout(w, h, t)
	feedback := wizardFeedback{kind: wizardFeedbackInfo, message: "Proof is optional. Explicit consent is required for each attempt."}
	action := "▸ [ START WSS PROOF ]  < OMIT >"
	if m.profileProof.ticket != "" {
		feedback, action = wizardFeedback{kind: wizardFeedbackWarning, message: "WSS proof permits SSH fallback. Press Enter to provide separate consent."}, "▸ [ START SSH FALLBACK ]  < OMIT >"
	}
	if m.profileProof.running {
		feedback, action = wizardFeedback{kind: wizardFeedbackInfo, message: "Proof is running within a bounded request. Esc cancels safely."}, "▸ [ CANCEL ]  < RETRY >"
	}
	lines := append(renderWizardTitleRow(panel.contentWidth, t, "Optional Remote Proof", "Step 7 of 8"), "", renderWizardDivider(panel.contentWidth, t), "", renderWizardFeedback(panel.contentWidth, t, feedback), "", action)
	return panel.render(w, lines)
}
