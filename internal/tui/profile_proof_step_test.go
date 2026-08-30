package tui

import (
	"context"
	"testing"

	"bac-nexus/internal/configuration"
	tea "github.com/charmbracelet/bubbletea"
)

type profileProofRunnerStub struct{ requests []configuration.Step8Request }

func (s *profileProofRunnerStub) Run(_ context.Context, request configuration.Step8Request) configuration.Step8Result {
	s.requests = append(s.requests, request)
	return configuration.Step8Result{RequestID: request.RequestID, Class: configuration.ResultProofSuccess, Cleanup: true}
}

func TestProfileProofCancelsAndOmitsWithoutRemoteCall(t *testing.T) {
	runner := &profileProofRunnerStub{}
	m := NewModelWithStep8Runner(&profileStoreStub{}, runner)
	m.screen, m.step8Action.request.Profile, m.profileProof.enabled = screenProfileStep8Action, testProfile("saved"), true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || !updated.(Model).profileProof.running {
		t.Fatal("explicit proof consent did not start bounded work")
	}
	cancelled, _ := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	if got := cancelled.(Model); got.screen != screenProfileCompletion || got.profileCompletion != profileCompletionCancelled {
		t.Fatalf("cancelled proof = screen %d outcome %q", got.screen, got.profileCompletion)
	}
	if len(runner.requests) != 0 {
		t.Fatalf("cancelled before command execution invoked runner %d times", len(runner.requests))
	}

	omit := NewModelWithStep8Runner(&profileStoreStub{}, runner)
	omit.screen, omit.step8Action.request.Profile, omit.profileProof.enabled = screenProfileStep8Action, testProfile("saved"), true
	updated, _ = omit.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	if got := updated.(Model); got.screen != screenProfileCompletion || got.profileCompletion != profileCompletionOmitted {
		t.Fatalf("omitted proof = screen %d outcome %q", got.screen, got.profileCompletion)
	}
}

func TestProfileProofRejectsStaleTimeoutAndSupersededResults(t *testing.T) {
	runner := &profileProofRunnerStub{}
	m := NewModelWithStep8Runner(&profileStoreStub{}, runner)
	m.screen, m.step8Action.request.Profile, m.profileProof.enabled = screenProfileStep8Action, testProfile("saved"), true
	updated, first := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	firstResult := first().(profileProofMsg)
	updated, retry := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = updated.(Model)
	if retry == nil || m.profileProof.generation != 2 {
		t.Fatal("retry did not supersede the first generation")
	}
	updated, _ = m.Update(firstResult)
	if got := updated.(Model); got.profileProof.generation != 2 || got.screen != screenProfileStep8Action {
		t.Fatalf("stale result changed current proof: generation=%d screen=%d", got.profileProof.generation, got.screen)
	}
	timeout := profileProofMsg{request: m.profileProof.request, generation: m.profileProof.generation, result: configuration.Step8Result{Class: configuration.ResultOperationTimeout}}
	updated, _ = m.Update(timeout)
	if got := updated.(Model); got.screen != screenProfileCompletion || got.profileCompletion != profileCompletionFailed {
		t.Fatalf("timeout completion = screen %d outcome %q", got.screen, got.profileCompletion)
	}
}

func TestProfileProofRequiresSeparateSSHConsent(t *testing.T) {
	runner := &profileProofRunnerStub{}
	m := NewModelWithStep8Runner(&profileStoreStub{}, runner)
	m.screen, m.step8Action.request.Profile, m.profileProof = screenProfileStep8Action, testProfile("saved"), profileProofState{enabled: true, request: "wss-1", generation: 1, ticket: "ticket", class: configuration.ReasonDaemonUnavailable}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || !updated.(Model).profileProof.running {
		t.Fatal("separate SSH consent did not start bounded work")
	}
	_ = cmd()
	if got := runner.requests[0]; !got.SSHConsent || got.WSSConsent || got.FallbackTicket != "ticket" {
		t.Fatalf("SSH request did not retain distinct consent and ticket: %#v", got)
	}
}
