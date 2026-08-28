package tui

import (
	"context"
	"strings"
	"testing"

	"bac-nexus/internal/configuration"
	tea "github.com/charmbracelet/bubbletea"
)

type blockingStep8Runner struct {
	started chan configuration.Step8Request
	release chan configuration.Step8Result
}

func (r *blockingStep8Runner) Run(ctx context.Context, request configuration.Step8Request) configuration.Step8Result {
	r.started <- request
	select {
	case result := <-r.release:
		return result
	case <-ctx.Done():
		return configuration.Step8Result{RequestID: request.RequestID, Class: configuration.ResultCancelled}
	}
}

func TestStep8ActionCancelRetryRejectsStaleResult(t *testing.T) {
	runner := &blockingStep8Runner{started: make(chan configuration.Step8Request, 2), release: make(chan configuration.Step8Result, 2)}
	m := NewModelWithStep8Runner(&profileStoreStub{}, runner)
	m.screen = screenProfileStep8Action

	started, first := m.startStep8Action(testProfile("saved"), true)
	m = started
	firstMsg := make(chan tea.Msg, 1)
	go func() { firstMsg <- first() }()
	firstRequest := <-runner.started

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = updated.(Model)
	if m.step8Action.phase != step8ActionPhaseCancelled || !strings.Contains(m.View(), "Cancelled") {
		t.Fatalf("cancel state = %#v; view=%q", m.step8Action, m.View())
	}

	updated, retry := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = updated.(Model)
	if retry == nil || m.step8Action.phase != step8ActionPhaseRunning || m.step8Action.request.RequestID == firstRequest.RequestID {
		t.Fatalf("retry did not start a new request: %#v", m.step8Action)
	}
	retryMsg := make(chan tea.Msg, 1)
	go func() { retryMsg <- retry() }()
	secondRequest := <-runner.started

	runner.release <- configuration.Step8Result{RequestID: secondRequest.RequestID, Class: configuration.ResultProofSuccess}
	updated, _ = m.Update(<-retryMsg)
	m = updated.(Model)
	if m.step8Action.phase != step8ActionPhaseSuccess {
		t.Fatalf("retry result phase = %v", m.step8Action.phase)
	}

	updated, _ = m.Update(<-firstMsg)
	if updated.(Model).step8Action.phase != step8ActionPhaseSuccess {
		t.Fatal("stale cancelled request changed current success")
	}
}

func TestStep8ActionViewIsResponsiveAndDoesNotRun(t *testing.T) {
	for _, tt := range []struct {
		name   string
		width  int
		height int
		phase  step8ActionPhase
		want   string
	}{
		{"idle", 120, 40, step8ActionPhaseIdle, "Ready to validate"},
		{"running", 80, 24, step8ActionPhaseRunning, "Validation is running"},
		{"success", 40, 16, step8ActionPhaseSuccess, "Validation complete"},
		{"terminal", 80, 24, step8ActionPhaseTerminal, "could not be completed"},
		{"cancelled", 40, 16, step8ActionPhaseCancelled, "Cancelled"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runner := &countingStep8Runner{}
			m := NewModelWithStep8Runner(&profileStoreStub{}, runner)
			m.screen, m.noColor, m.step8Action.phase = screenProfileStep8Action, true, tt.phase
			updated, _ := m.Update(tea.WindowSizeMsg{Width: tt.width, Height: tt.height})
			view := updated.(Model).View()
			if !strings.Contains(view, tt.want) || strings.Contains(view, "ibmi.example.test") || runner.calls != 0 {
				t.Fatalf("view=%q calls=%d", view, runner.calls)
			}
		})
	}
}
