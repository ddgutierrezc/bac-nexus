package tui

import (
	"context"
	"testing"

	"bac-nexus/internal/configuration"
	tea "github.com/charmbracelet/bubbletea"
)

type countingStep8Runner struct{ calls int }

func (r *countingStep8Runner) Run(context.Context, configuration.Step8Request) configuration.Step8Result {
	r.calls++
	return configuration.Step8Result{}
}

func TestNewModelWithStep8RunnerStoresOnlyTheRunnerSeam(t *testing.T) {
	runner := &countingStep8Runner{}
	m := NewModelWithStep8Runner(&profileStoreStub{}, runner)

	if m.step8Runner != runner {
		t.Fatal("model did not retain the injected Step 8 runner")
	}
	if cmd := m.Init(); cmd != nil {
		_ = cmd()
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_ = updated.(Model).View()
	if runner.calls != 0 {
		t.Fatalf("constructor, Init, Update, and View invoked the Step 8 runner %d times", runner.calls)
	}
}

func TestInjectedStep8RunnerIsReachableFromStep8Action(t *testing.T) {
	runner := &countingStep8Runner{}
	saved := testProfile("saved")
	m := NewModelWithStep8Runner(&profileStoreStub{}, runner)
	m.screen = screenProfileStep8Action
	m.step8Action = step8Action{request: configuration.Step8Request{Profile: saved}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Step 8 action did not create an explicit runner command")
	}
	m = updated.(Model)
	_ = cmd()
	if runner.calls != 1 {
		t.Fatalf("explicit Step 8 action invoked the runner %d times, want 1", runner.calls)
	}
	if m.step8Action.request.Profile != saved {
		t.Fatalf("Step 8 action profile = %#v, want saved profile %#v", m.step8Action.request.Profile, saved)
	}
}
