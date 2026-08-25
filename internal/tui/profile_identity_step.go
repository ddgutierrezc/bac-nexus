package tui

import (
	"context"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"bac-nexus/internal/hostidentity"
	"golang.org/x/text/language"
)

type profileIdentityFocus uint8

const (
	profileIdentityFocusInspect profileIdentityFocus = iota
	profileIdentityFocusBack
	profileIdentityFocusTrust
)

const identityInspectionTimeout = 10 * time.Second

func (m *Model) cancelIdentityInspection() {
	if m.identityCancel != nil {
		m.identityCancel()
		m.identityCancel = nil
	}
	m.identityRequest++
}

func (m Model) identityInspectionCmd(request uint64, host string, port int, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		candidate, err := m.identityInspector.InspectHostKey(ctx, host, port)
		return profileIdentityInspectionMsg{request: request, host: host, port: port, candidate: candidate, err: err}
	}
}

func (m *Model) startIdentityInspection() tea.Cmd {
	if m.identityPhase == profileIdentityLoading {
		return nil
	}
	if m.identityInspector == nil {
		m.identityPhase, m.status = profileIdentityError, wizardFeedbackRow(wizardFeedback{kind: wizardFeedbackError, message: m.text("wizard.identity.error", nil)})
		return nil
	}
	m.cancelIdentityInspection()
	m.identityPhase, m.identityCandidate, m.status, m.err = profileIdentityLoading, hostidentity.Candidate{}, wizardFeedbackRow(wizardFeedback{kind: wizardFeedbackNeutral, message: m.text("wizard.identity.loading", nil)}), nil
	m.identityFocus = profileIdentityFocusInspect
	request, host, port := m.identityRequest, m.connectionDraft.host, m.connectionDraft.port
	parent := m.identityParent
	if parent == nil {
		parent = context.Background()
	}
	timeout := m.identityTimeout
	if timeout <= 0 {
		timeout = identityInspectionTimeout
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	m.identityCancel = cancel
	return m.identityInspectionCmd(request, host, port, ctx)
}

func (m Model) updateProfileIdentityStep(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		if m.identityPhase == profileIdentityLoading {
			m.cancelIdentityInspection()
			m.identityPhase, m.identityCandidate, m.status, m.err, m.identityFocus = profileIdentityAuthorize, hostidentity.Candidate{}, "", nil, profileIdentityFocusInspect
			return m, nil
		}
		if m.identityPhase == profileIdentityReview {
			m.cancelIdentityInspection()
			m.identityPhase, m.identityCandidate, m.status, m.err, m.identityFocus = profileIdentityAuthorize, hostidentity.Candidate{}, "", nil, profileIdentityFocusInspect
			return m, nil
		}
		if m.identityPhase == profileIdentityCompleted {
			m.screen = screenProfileConnection
			return m, nil
		}
		if m.identityPhase == profileIdentityError {
			m.identityPhase, m.status, m.err, m.identityFocus = profileIdentityAuthorize, "", nil, profileIdentityFocusInspect
			return m, nil
		}
		m.screen = screenProfileConnection
		return m, nil
	case "?":
		m.status, m.err = m.text("wizard.identity.help", nil), nil
		return m, nil
	case "tab":
		m.identityFocus = m.nextIdentityFocus(1)
		return m, nil
	case "shift+tab":
		m.identityFocus = m.nextIdentityFocus(-1)
		return m, nil
	case "enter":
		switch m.identityFocus {
		case profileIdentityFocusBack:
			if m.identityPhase == profileIdentityReview || m.identityPhase == profileIdentityError {
				m.cancelIdentityInspection()
				m.identityPhase, m.identityCandidate, m.status, m.err, m.identityFocus = profileIdentityAuthorize, hostidentity.Candidate{}, "", nil, profileIdentityFocusInspect
				return m, nil
			}
			m.screen = screenProfileConnection
			return m, nil
		case profileIdentityFocusInspect:
			if m.identityPhase == profileIdentityAuthorize || m.identityPhase == profileIdentityError {
				return m, m.startIdentityInspection()
			}
		case profileIdentityFocusTrust:
			if m.identityPhase == profileIdentityReview {
				candidate, request := m.identityCandidate, m.identityRequest
				return m, func() tea.Msg { return profileIdentityAcceptedMsg{request: request, candidate: candidate} }
			}
		}
	}
	return m, nil
}

func (m Model) nextIdentityFocus(delta int) profileIdentityFocus {
	focuses := []profileIdentityFocus{profileIdentityFocusInspect, profileIdentityFocusBack}
	if m.identityPhase == profileIdentityReview {
		focuses = []profileIdentityFocus{profileIdentityFocusBack, profileIdentityFocusTrust}
	}
	for i, focus := range focuses {
		if focus == m.identityFocus {
			return focuses[(i+delta+len(focuses))%len(focuses)]
		}
	}
	return focuses[0]
}

func (m Model) renderProfileIdentityStep() string {
	fw, fh := m.shellFrameDimensions()
	w, h := m.shellInnerWidth(fw), m.shellInnerHeight(fh)
	t := newHomeTheme(m.noColor)
	return m.renderWizardShell(m.renderProfileConnectionHeader(w, t), renderFooterText(w, t, m.identityFooter(), m.buildInfo), m.renderProfileIdentityPanel(w, h, t))
}

func (m Model) identityFooter() string {
	if m.identityPhase == profileIdentityReview || m.identityPhase == profileIdentityCompleted {
		return m.text("wizard.identity.footer_review", nil)
	}
	return m.text("wizard.identity.footer_authorize", nil)
}

func (m Model) identityFailureText(failure hostidentity.Failure) string {
	cause := ""
	if m.localizer != nil && m.localizer.Locale() == language.English {
		switch failure {
		case hostidentity.FailureTimeout:
			cause = "inspection timed out"
		case hostidentity.FailureNegotiation:
			cause = "secure SSH negotiation failed"
		case hostidentity.FailureNoKey:
			cause = "no server key was observed"
		default:
			cause = "inspection is unavailable"
		}
	} else {
		switch failure {
		case hostidentity.FailureTimeout:
			cause = "la inspección agotó el tiempo"
		case hostidentity.FailureNegotiation:
			cause = "falló la negociación SSH segura"
		case hostidentity.FailureNoKey:
			cause = "no se observó una clave del servidor"
		default:
			cause = "la inspección no está disponible"
		}
	}
	return m.text("wizard.identity.error", nil) + ": " + cause
}

func (m Model) renderProfileIdentityPanel(w, h int, t homeTheme) string {
	return m.renderProfileIdentityPanelContent(w, h, t).text
}

type wizardPanel struct {
	text   string
	ranges map[profileIdentityFocus]wizardLineRange
}
type wizardLineRange struct{ start, end int }

func (m Model) renderProfileIdentityPanelContent(w, h int, t homeTheme) wizardPanel {
	panel := newWizardPanelLayout(w, h, t)
	cw, rhythm := panel.contentWidth, newWizardRhythm(h)
	title := m.text("wizard.identity.title", nil)
	if m.identityPhase == profileIdentityReview || m.identityPhase == profileIdentityCompleted {
		title = m.text("wizard.identity.observed_title", nil)
	}
	lines := renderWizardTitleRow(cw, t, title, m.text("wizard.step.identity", nil))
	ranges := make(map[profileIdentityFocus]wizardLineRange)
	lines = appendWizardGap(lines, rhythm.titleDivider)
	lines = append(lines, renderWizardDivider(cw, t))
	if m.identityPhase == profileIdentityReview || m.identityPhase == profileIdentityCompleted {
		lines = append(lines, t.wizardContentHeading.Render(m.text("wizard.identity.observed_title", nil)))
		lines = appendWizardGap(lines, rhythm.sectionDescription)
		for _, row := range []string{m.text("form.label.host", nil) + "             " + m.connectionDraft.host, m.text("form.label.port", nil) + "           " + strconv.Itoa(m.connectionDraft.port), m.text("wizard.identity.key_type", nil) + "    " + m.identityCandidate.Algorithm, m.text("form.label.fingerprint", nil) + "      " + m.identityCandidate.Fingerprint} {
			for _, x := range wrapWizardText(row, cw, "") {
				lines = append(lines, t.fieldsetContent.Render(x))
			}
		}
		lines = appendWizardGap(lines, rhythm.questionSupporting)
		lines = append(lines, renderWizardFeedback(cw, t, wizardFeedback{kind: wizardFeedbackWarning, message: m.text("wizard.identity.warning_observed", nil)}))
		for _, text := range []string{m.text("wizard.identity.observed_description_1", nil), m.text("wizard.identity.observed_description_2", nil)} {
			for _, x := range wrapWizardText(text, cw, "") {
				lines = append(lines, t.metadata.Render(x))
			}
		}
	} else {
		lines = append(lines, t.wizardContentHeading.Render(m.text("wizard.identity.section", nil)))
		lines = appendWizardGap(lines, rhythm.sectionDescription)
		for _, text := range []string{m.text("wizard.identity.authorize", nil), "Host      " + m.connectionDraft.host, "Puerto    " + strconv.Itoa(m.connectionDraft.port), m.text("wizard.identity.notice_1", nil), m.text("wizard.identity.notice_2", nil)} {
			for _, x := range wrapWizardText(text, cw, "") {
				lines = append(lines, t.fieldsetContent.Render(x))
			}
			lines = appendWizardGap(lines, rhythm.questionSupporting)
		}
	}
	if feedback, ok := m.wizardFeedbackFor(""); ok {
		lines = appendWizardGap(lines, rhythm.feedback)
		lines = append(lines, renderWizardFeedback(cw, t, feedback))
	}
	lines = appendWizardGap(lines, rhythm.actions)
	primary, primaryState := m.text("wizard.identity.inspect", nil), wizardProgressReady
	if m.identityPhase == profileIdentityError {
		primary = m.text("wizard.identity.retry", nil)
	}
	if m.identityPhase == profileIdentityReview {
		primary = m.text("wizard.identity.trust", nil)
	}
	if m.identityPhase == profileIdentityCompleted || m.identityPhase == profileIdentityLoading {
		primary, primaryState = m.text("wizard.identity.trusted", nil), wizardProgressDisabled
	}
	leftFocus := m.identityFocus == profileIdentityFocusBack
	rightFocus := primaryState != wizardProgressDisabled && (m.identityFocus == profileIdentityFocusInspect || m.identityFocus == profileIdentityFocusTrust)
	actions := renderWizardActionsBlock(cw, t, m.text("action.back", nil), primary, leftFocus, rightFocus, m.noColor, wizardActionOptions{rightState: primaryState})
	start := len(lines) + actions.start
	lines = append(lines, strings.Split(actions.text, "\n")...)
	ranges[profileIdentityFocusBack] = wizardLineRange{start: start, end: start}
	if primaryState != wizardProgressDisabled {
		ranges[m.identityFocus] = wizardLineRange{start: start + actions.end, end: start + actions.end}
	}
	for focus, r := range ranges {
		ranges[focus] = wizardLineRange{start: r.start + panel.contentTopOffset, end: r.end + panel.contentTopOffset}
	}
	return wizardPanel{text: panel.render(w, lines), ranges: ranges}
}
