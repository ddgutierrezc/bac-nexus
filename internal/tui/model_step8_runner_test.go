package tui

import (
	"context"
	"strings"
	"testing"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
)

type countingStep8Runner struct{ calls int }

func (r *countingStep8Runner) Run(context.Context, configuration.Step8Request) configuration.Step8Result {
	r.calls++
	return configuration.Step8Result{}
}

type fixedPreAuthProbe struct{ resolution configuration.Resolution }

func (p fixedPreAuthProbe) Probe(context.Context) (configuration.Resolution, error) {
	return p.resolution, nil
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

func TestStepThreeAndFourRemainPreAuthWithInjectedStep8Runner(t *testing.T) {
	for _, tt := range []struct {
		name    string
		width   int
		height  int
		noColor bool
	}{
		{name: "wide", width: 120, height: 40},
		{name: "standard no color", width: 80, height: 24, noColor: true},
		{name: "narrow", width: 40, height: 16},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runner := &countingStep8Runner{}
			m := NewModelWithStep8Runner(&profileStoreStub{}, runner)
			m.noColor = tt.noColor
			m.connectionDraft = profileConnectionDraft{host: "ibmi.example.test", username: "USER", port: 22}
			m.screen = screenProfileIdentity
			m.identityPhase = profileIdentityCompleted
			m.identityFocus = profileIdentityFocusInspect

			updated, _ := m.Update(tea.WindowSizeMsg{Width: tt.width, Height: tt.height})
			m = updated.(Model)
			_ = m.View()

			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd != nil {
				t.Fatal("completed Step 3 navigation returned an unexpected command")
			}
			m = updated.(Model)
			if m.screen != screenProfileMapepire {
				t.Fatalf("Step 3 did not advance to Step 4: got %d", m.screen)
			}

			m.mapepireProbe = fixedPreAuthProbe{resolution: configuration.Resolution{Transport: configuration.TransportWSS, AuthenticationPending: true}}
			updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd == nil {
				t.Fatal("Step 4 did not issue its pre-auth observation command")
			}
			m = updated.(Model)
			updated, _ = m.Update(cmd())
			m = updated.(Model)
			view := m.View()
			if !m.mapepireResolution.AuthenticationPending || view == "" {
				t.Fatalf("Step 4 did not retain its pending-authentication state: %+v", m.mapepireResolution)
			}
			if tt.width > 40 && !strings.Contains(view, "authentication pending") {
				t.Fatalf("Step 4 did not retain pending-authentication copy:\n%s", view)
			}
			if runner.calls != 0 {
				t.Fatalf("Steps 3 and 4 invoked the Step 8 runner %d times", runner.calls)
			}
		})
	}
}

func TestInjectedStep8RunnerIsReachableFromWizardAfterPreAuth(t *testing.T) {
	runner := &countingStep8Runner{}
	saved := testProfile("saved")
	m := NewModelWithStep8Runner(&profileStoreStub{}, runner)
	m.profiles = []profile.Profile{saved}
	m.profileDraftName = saved.Name
	m.screen = screenProfileMapepire
	m.mapepireResolution = configuration.Resolution{Transport: configuration.TransportWSS, AuthenticationPending: true}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("authenticated-pending Step 4 should only navigate to the explicit Step 8 action")
	}
	m = updated.(Model)
	if m.screen != screenProfileStep8Action {
		t.Fatalf("Step 4 continuation screen = %d, want Step 8 action", m.screen)
	}
	if runner.calls != 0 {
		t.Fatalf("Step 4 navigation invoked the Step 8 runner %d times", runner.calls)
	}

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
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

func TestPreAuthContinuationRequiresSavedProfile(t *testing.T) {
	runner := &countingStep8Runner{}
	m := NewModelWithStep8Runner(&profileStoreStub{}, runner)
	m.profileDraftName = "unsaved"
	m.screen = screenProfileMapepire
	m.mapepireResolution = configuration.Resolution{Transport: configuration.TransportWSS, AuthenticationPending: true}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("unsaved Step 4 continuation returned an unexpected action command")
	}
	m = updated.(Model)
	if m.screen != screenProfileMapepire {
		t.Fatalf("unsaved continuation screen = %d, want Step 4", m.screen)
	}
	if runner.calls != 0 {
		t.Fatalf("unsaved Step 4 continuation invoked the Step 8 runner %d times", runner.calls)
	}
}
