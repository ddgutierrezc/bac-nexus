package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"bac-nexus/internal/hostidentity"
	"bac-nexus/internal/localization"
	"bac-nexus/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var testCandidate = hostidentity.Candidate{Algorithm: "ssh-ed25519", Fingerprint: "SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}

type blockingInspector struct {
	started chan context.Context
	release chan hostidentity.Candidate
	failure error
}

func (s *blockingInspector) InspectHostKey(ctx context.Context, _ string, _ int) (hostidentity.Candidate, error) {
	s.started <- ctx
	select {
	case candidate := <-s.release:
		return candidate, s.failure
	case <-ctx.Done():
		failure := hostidentity.FailureCancelled
		if ctx.Err() == context.DeadlineExceeded {
			failure = hostidentity.FailureTimeout
		}
		return hostidentity.Candidate{}, &hostidentity.FailureError{Failure: failure}
	}
}

type fixedInspector struct {
	candidate hostidentity.Candidate
	err       error
	calls     int
}

func (s *fixedInspector) InspectHostKey(context.Context, string, int) (hostidentity.Candidate, error) {
	s.calls++
	return s.candidate, s.err
}

func newProfileIdentityTestModelWithInspector(t *testing.T, inspector hostidentity.Inspector, parent context.Context, timeout time.Duration, w, h int) Model {
	t.Helper()
	m := newModelWithIdentityInspector(&profileStoreStub{}, BuildInfo{}, inspector, parent, timeout)
	m.connectionDraft = profileConnectionDraft{host: "ibmi.example.test", username: "USER", port: 22}
	m.resetProfileIdentityStep()
	m.beginProfileIdentityStep()
	u, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return u.(Model)
}
func newProfileIdentityTestModel(t *testing.T, w, h int) Model {
	return newProfileIdentityTestModelWithInspector(t, &fixedInspector{candidate: testCandidate}, context.Background(), time.Second, w, h)
}
func startInspection(t *testing.T, m Model) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("inspection command missing")
	}
	return updated.(Model), cmd
}
func finishInspection(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	updated, _ := m.Update(cmd())
	return updated.(Model)
}
func acceptCandidate(t *testing.T, m Model) Model {
	t.Helper()
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("trust command missing")
	}
	updated, _ = updated.(Model).Update(cmd())
	return updated.(Model)
}

func TestProfileIdentityLifecycleCancellationCancelsRealCommandAndIgnoresLateResult(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	inspector := &blockingInspector{started: make(chan context.Context, 1), release: make(chan hostidentity.Candidate, 1)}
	m := newProfileIdentityTestModelWithInspector(t, inspector, parent, time.Second, 80, 24)
	m, cmd := startInspection(t, m)
	result := make(chan tea.Msg, 1)
	go func() { result <- cmd() }()
	ctx := <-inspector.started
	cancelParent()
	<-ctx.Done()
	msg := <-result
	updated, _ := m.Update(msg)
	m = updated.(Model)
	if m.identityPhase != profileIdentityLoading {
		t.Fatal("parent cancellation mutated active request before user cancellation")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(Model)
	if m.identityPhase != profileIdentityAuthorize {
		t.Fatal("escape did not restore authorize")
	}
	updated, _ = m.Update(msg)
	if updated.(Model).identityPhase != profileIdentityAuthorize {
		t.Fatal("late result was accepted")
	}
}

func TestProfileIdentityTimeoutAndRetrySupersessionUseRealCommands(t *testing.T) {
	blocking := &blockingInspector{started: make(chan context.Context, 2), release: make(chan hostidentity.Candidate, 2)}
	m := newProfileIdentityTestModelWithInspector(t, blocking, context.Background(), time.Millisecond, 80, 24)
	m, cmd := startInspection(t, m)
	msg := cmd()
	updated, _ := m.Update(msg)
	m = updated.(Model)
	if m.identityPhase != profileIdentityError || !strings.Contains(m.View(), "agotó el tiempo") {
		t.Fatal("timeout did not become safe retryable error")
	}
	// Start a blocking request, cancel it through Escape, then complete a newer request.
	m.identityTimeout = time.Second
	m, first := startInspection(t, m)
	firstMsg := make(chan tea.Msg, 1)
	go func() { firstMsg <- first() }()
	<-blocking.started
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(Model)
	m, second := startInspection(t, m)
	secondMsg := make(chan tea.Msg, 1)
	go func() { secondMsg <- second() }()
	<-blocking.started
	blocking.release <- testCandidate
	updated, _ = m.Update(<-secondMsg)
	m = updated.(Model)
	if m.identityPhase != profileIdentityReview {
		t.Fatal("new request did not reach review")
	}
	updated, _ = m.Update(<-firstMsg)
	if updated.(Model).identityPhase != profileIdentityReview {
		t.Fatal("superseded completion changed review")
	}
}

func TestProfileIdentityBackRetentionAndExactTypedAcceptance(t *testing.T) {
	inspector := &fixedInspector{candidate: testCandidate}
	m := newProfileIdentityTestModelWithInspector(t, inspector, context.Background(), time.Second, 120, 40)
	m, cmd := startInspection(t, m)
	m = finishInspection(t, m, cmd)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(Model)
	if m.identityPhase != profileIdentityAuthorize || m.identityCandidate != (hostidentity.Candidate{}) {
		t.Fatal("review back retained unaccepted candidate")
	}
	m, cmd = startInspection(t, m)
	m = finishInspection(t, m, cmd)
	m = acceptCandidate(t, m)
	if m.identityDraft.trustMethod != profile.HostKeyTrustTOFU || m.identityDraft.fingerprint != testCandidate.Fingerprint {
		t.Fatal("acceptance did not retain typed exact TOFU evidence")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(Model)
	if m.screen != screenProfileConnection || m.identityPhase != profileIdentityCompleted {
		t.Fatal("completed back did not retain accepted draft on Step 2")
	}
	updated, _ = m.Update(profileConnectionAcceptedMsg{host: "ibmi.example.test", username: "NEWUSER", port: 22})
	m = updated.(Model)
	if m.identityPhase != profileIdentityCompleted {
		t.Fatal("username-only change invalidated identity")
	}
	updated, _ = m.Update(profileConnectionAcceptedMsg{host: "other.example.test", username: "NEWUSER", port: 22})
	if updated.(Model).identityPhase != profileIdentityAuthorize {
		t.Fatal("endpoint change did not reset identity")
	}
}

func TestProfileIdentityAcceptanceFocusesForwardActionAndEnterReachesStep4(t *testing.T) {
	m := newProfileIdentityTestModel(t, 80, 24)
	m, cmd := startInspection(t, m)
	m = finishInspection(t, m, cmd)
	m = acceptCandidate(t, m)

	if m.identityPhase != profileIdentityCompleted || m.identityFocus != profileIdentityFocusInspect {
		t.Fatalf("accepted identity phase/focus=%v/%v, want completed/forward", m.identityPhase, m.identityFocus)
	}
	if m.screen != screenProfileIdentity {
		t.Fatal("acceptance auto-transitioned before the explicit forward action")
	}
	if strings.TrimSpace(m.View()) == "" {
		t.Fatal("completed identity view was empty")
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || updated.(Model).screen != screenProfileMapepire {
		t.Fatal("Enter after acceptance did not reach Step 4 without focus navigation")
	}
}

func TestProfileIdentityRuntimeMatrixUsesActualTransitions(t *testing.T) {
	for _, noColor := range []bool{false, true} {
		for _, size := range []struct{ w, h int }{{120, 40}, {80, 24}, {40, 16}} {
			previous := lipgloss.ColorProfile()
			if noColor {
				lipgloss.SetColorProfile(termenv.Ascii)
			} else {
				lipgloss.SetColorProfile(termenv.TrueColor)
			}
			t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
			for _, phase := range []string{"authorize", "loading", "error", "review", "completed"} {
				m, cancel := identityRuntimePhase(t, phase, size.w, size.h)
				m.noColor = noColor
				m.refreshWizardViewport()
				pages := identityRuntimePages(t, m)
				assertIdentityRuntimePages(t, pages, m, phase, size.w, size.h, noColor)
				cancel()
			}
		}
	}
}

func identityRuntimePhase(t *testing.T, phase string, width, height int) (Model, func()) {
	t.Helper()
	if phase == "loading" {
		inspector := &blockingInspector{started: make(chan context.Context, 1), release: make(chan hostidentity.Candidate, 1)}
		m := newProfileIdentityTestModelWithInspector(t, inspector, context.Background(), time.Second, width, height)
		m, cmd := startInspection(t, m)
		go cmd()
		<-inspector.started
		return m, func() { updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape}); _ = updated }
	}
	inspector := hostidentity.Inspector(&fixedInspector{candidate: testCandidate})
	if phase == "error" {
		inspector = &fixedInspector{err: &hostidentity.FailureError{Failure: hostidentity.FailureNoKey}}
	}
	m := newProfileIdentityTestModelWithInspector(t, inspector, context.Background(), time.Second, width, height)
	if phase == "authorize" {
		return m, func() {}
	}
	m, cmd := startInspection(t, m)
	m = finishInspection(t, m, cmd)
	if phase == "completed" {
		m = acceptCandidate(t, m)
	}
	return m, func() {}
}

func identityRuntimePages(t *testing.T, m Model) []string {
	t.Helper()
	for range 128 {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
		m = updated.(Model)
	}
	pages := []string{m.View()}
	for range 256 {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(Model)
		view := m.View()
		if view == pages[len(pages)-1] {
			break
		}
		pages = append(pages, view)
	}
	return pages
}

func assertIdentityRuntimePages(t *testing.T, pages []string, m Model, phase string, width, height int, noColor bool) {
	t.Helper()
	all := strings.Join(pages, "\n")
	panel := runtimePanelFragments(pages)
	for _, page := range pages {
		if lipgloss.Width(page) > width || lipgloss.Height(page) > height {
			t.Fatalf("%s overflow %dx%d", phase, width, height)
		}
		for _, line := range strings.Split(page, "\n") {
			if lipgloss.Width(line) > width {
				t.Fatalf("%s horizontal overflow: %q", phase, line)
			}
		}
		if noColor && strings.Contains(page, "\x1b[") {
			t.Fatal("NO_COLOR ANSI")
		}
	}
	if m.wizardViewport.TotalLineCount() > m.wizardViewport.Height && !strings.Contains(all, "▼ más") && !strings.Contains(all, "▲ más") {
		t.Fatalf("%s hides content without a paging indicator", phase)
	}
	footer, action := "Tab siguiente", "[ INSPECCIONAR IDENTIDAD ]"
	if phase == "error" {
		action = "[ REINTENTAR ]"
	}
	if phase == "loading" {
		action = "[ CLAVE CONFIADA ]"
	}
	if phase == "review" || phase == "completed" {
		footer, action = "Tab siguiente", "[ CONFIAR EN ESTA CLAVE ]"
		if phase == "completed" {
			action = "[ CLAVE CONFIADA ]"
		}
	}
	for _, want := range []string{footer, action} {
		if !strings.Contains(runtimeIdentityText(all), runtimeIdentityText(want)) {
			t.Fatalf("%s pages omitted %q", phase, want)
		}
	}
	if phase == "review" || phase == "completed" {
		for _, want := range []string{testCandidate.Algorithm, testCandidate.Fingerprint, "Esta es la primera vez"} {
			if !strings.Contains(runtimeIdentityText(panel), runtimeIdentityText(want)) {
				t.Fatalf("%s pages omitted %q", phase, want)
			}
		}
	}
}

func runtimeIdentityText(text string) string {
	return strings.Join(strings.Fields(ansiEscape.ReplaceAllString(text, "")), "")
}

func TestProfileIdentityEnglishReviewAndCompletedFramesHaveNoSpanishLabels(t *testing.T) {
	for _, complete := range []bool{false, true} {
		m := newProfileIdentityTestModelWithInspector(t, &fixedInspector{candidate: testCandidate}, context.Background(), time.Second, 120, 40)
		m.localizer = localization.English()
		m, cmd := startInspection(t, m)
		m = finishInspection(t, m, cmd)
		if complete {
			m = acceptCandidate(t, m)
		}
		frames := []string{}
		for range 64 {
			frames = append(frames, m.View())
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
			m = updated.(Model)
		}
		all := strings.Join(frames, "\n")
		for _, forbidden := range []string{"Puerto", "Tipo de clave", "Identidad observada", "Fingerprint      "} {
			if strings.Contains(all, forbidden) {
				t.Fatalf("English frame leaked %q: %s", forbidden, all)
			}
		}
		for _, required := range []string{"Port", "Key type", "Observed identity", testCandidate.Fingerprint} {
			if !strings.Contains(all, required) {
				t.Fatalf("English frame omitted %q", required)
			}
		}
	}
}

func alphaNumericOnly(text string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, text)
}
func reconstructWrappedParagraphs(lines []string, _ string, prefix string) string {
	indent := strings.Repeat(" ", lipgloss.Width(prefix))
	for i := range lines {
		if i == 0 {
			lines[i] = strings.TrimPrefix(lines[i], prefix)
		} else {
			lines[i] = strings.TrimPrefix(lines[i], indent)
		}
	}
	return strings.Join(lines, "")
}
