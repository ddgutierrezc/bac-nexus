package tui

import (
	"context"
	"fmt"
	"strings"

	"bac-nexus/internal/configuration"
	tea "github.com/charmbracelet/bubbletea"
)

// preAuthProbe is deliberately smaller than the resolver: Step 4 may observe
// readiness, but it cannot own credentials or start fallback runtime work.
type preAuthProbe interface {
	Probe(context.Context) (configuration.Resolution, error)
}

type managedDaemonPreAuth struct {
	probe *configuration.ManagedDaemonProbe
}

func (p managedDaemonPreAuth) Probe(ctx context.Context) (configuration.Resolution, error) {
	version, err := p.probe.Probe(ctx)
	if err != nil {
		return configuration.Resolution{Class: configuration.FailureAvailability}, err
	}
	if version != "2.3.5" {
		return configuration.Resolution{Class: configuration.FailureUnsupported, Version: version}, nil
	}
	return configuration.Resolution{Transport: configuration.TransportWSS, Version: version, AuthenticationPending: true}, nil
}

type profileProofClient interface {
	Connect(context.Context) error
	Query(context.Context) (int, error)
}

type mapepireProbeMsg struct {
	resolution configuration.Resolution
	err        error
}

func (m Model) updateProfileMapepireStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.screen = screenProfileConnection
		return m, nil
	case "enter":
		if m.mapepireProbe == nil && m.mapepireFactory != nil {
			m.mapepireProbe = m.mapepireFactory(m.connectionDraft.host, m.connectionDraft.port)
		}
		if m.mapepireProbe == nil || m.mapepireResolution.AuthenticationPending {
			return m, nil
		}
		return m, func() tea.Msg {
			resolution, err := m.mapepireProbe.Probe(context.Background())
			return mapepireProbeMsg{resolution: resolution, err: err}
		}
	}
	return m, nil
}

func (m Model) renderProfileMapepireStep() string {
	frameWidth, frameHeight := m.shellFrameDimensions()
	width, height := m.shellInnerWidth(frameWidth), m.shellInnerHeight(frameHeight)
	t := newHomeTheme(m.noColor)
	return m.renderWizardShell(m.renderProfileConnectionHeader(width, t), renderFooterText(width, t, m.text("wizard.footer.mapepire", nil), m.buildInfo), m.renderProfileMapepirePanel(width, height, t))
}

func (m Model) renderProfileMapepirePanel(width, height int, t homeTheme) string {
	panel := newWizardPanelLayout(width, height, t)
	lines := renderWizardTitleRow(panel.contentWidth, t, "Mapepire", "Step 4 of 9 — Mapepire")
	lines = append(lines, "", renderWizardDivider(panel.contentWidth, t))
	lines = append(lines, t.wizardContentHeading.Render("Local readiness"), "")
	message := "[--] Mapepire readiness has not been observed"
	if m.mapepireResolution.AuthenticationPending {
		message = "[OK] Mapepire detected — authentication pending"
	} else if m.mapepireResolution.Class != configuration.FailureNone {
		message = "[INFO] Mapepire unavailable; authentication pending at Step 8"
	}
	lines = append(lines, t.metadata.Render(message), t.metadata.Render("No credentials, authentication, SSH fallback, JAR, Java, upload, or query is used here."), "")
	lines = append(lines, "▸ [ CONTINUAR ]")
	return panel.render(width, lines)
}

func runProfileStep8Proof(ctx context.Context, client profileProofClient) error {
	if client == nil {
		return fmt.Errorf("Step 8 client is required")
	}
	if err := client.Connect(ctx); err != nil {
		return err
	}
	if _, err := client.Query(ctx); err != nil {
		return err
	}
	return nil
}

func (m Model) mapepirePanelText() string {
	return strings.TrimSpace(m.renderProfileMapepirePanel(80, 24, newHomeTheme(true)))
}
