package tui

import (
	"context"
	"strconv"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
)

type step8ActionPhase uint8

const (
	step8ActionPhaseIdle step8ActionPhase = iota
	step8ActionPhaseRunning
	step8ActionPhaseSuccess
	step8ActionPhaseTerminal
	step8ActionPhaseCancelled
)

type step8Action struct {
	request    configuration.Step8Request
	generation uint64
	phase      step8ActionPhase
	cancel     context.CancelFunc
}

type step8ActionMsg struct {
	requestID string
	class     configuration.ResultClass
}

func (m Model) startStep8Action(p profile.Profile, consent bool) (Model, tea.Cmd) {
	if m.step8Action.cancel != nil {
		m.step8Action.cancel()
	}
	m.step8Action.generation++
	requestID := "step8-" + strconv.FormatUint(m.step8Action.generation, 10)
	ctx, cancel := context.WithCancel(context.Background())
	m.step8Action.request = configuration.Step8Request{RequestID: requestID, Profile: p, Consent: consent}
	m.step8Action.phase, m.step8Action.cancel = step8ActionPhaseRunning, cancel
	if m.step8Runner == nil {
		return m, func() tea.Msg {
			return step8ActionMsg{requestID: requestID, class: configuration.ResultDowngradeBlocked}
		}
	}
	return m, func() tea.Msg {
		result := m.step8Runner.Run(ctx, m.step8Action.request)
		return step8ActionMsg{requestID: requestID, class: result.Class}
	}
}

func (m *Model) applyStep8Action(msg step8ActionMsg) {
	if m.step8Action.phase != step8ActionPhaseRunning || msg.requestID != m.step8Action.request.RequestID {
		return
	}
	m.step8Action.cancel = nil
	if msg.class == configuration.ResultProofSuccess {
		m.step8Action.phase = step8ActionPhaseSuccess
		return
	}
	if msg.class == configuration.ResultCancelled {
		m.step8Action.phase = step8ActionPhaseCancelled
		return
	}
	m.step8Action.phase = step8ActionPhaseTerminal
}

func (m Model) updateStep8Action(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		if m.step8Action.phase == step8ActionPhaseRunning {
			if m.step8Action.cancel != nil {
				m.step8Action.cancel()
			}
			m.step8Action.cancel, m.step8Action.phase = nil, step8ActionPhaseCancelled
			return m, nil
		}
		m.screen = screenProfileMapepire
	case "enter":
		if m.step8Action.phase == step8ActionPhaseIdle && m.step8Action.request.Profile.Name != "" {
			return m.startStep8Action(m.step8Action.request.Profile, m.step8Action.request.Consent)
		}
	case "r":
		if (m.step8Action.phase == step8ActionPhaseCancelled || m.step8Action.phase == step8ActionPhaseTerminal) && m.step8Action.request.Profile.Name != "" {
			return m.startStep8Action(m.step8Action.request.Profile, m.step8Action.request.Consent)
		}
	}
	return m, nil
}
