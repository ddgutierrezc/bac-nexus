package tui

import (
	"strings"
	"testing"

	"bac-nexus/internal/configuration"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestProfileCompletionMapsTerminalOutcomesTruthfully(t *testing.T) {
	for _, tt := range []struct {
		name    string
		result  configuration.Step8Result
		outcome profileCompletionOutcome
		copy    string
	}{
		{"success", configuration.Step8Result{Class: configuration.ResultProofSuccess, Cleanup: true}, profileCompletionSuccessful, "ready for controlled validation"},
		{"failed cleanup", configuration.Step8Result{Class: configuration.ResultProofSuccess}, profileCompletionFailed, "Proof did not complete"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(&profileStoreStub{})
			m.screen = screenProfileStep8Action
			m.profileProof = profileProofState{running: true, request: "current", generation: 1}
			updated, _ := m.Update(profileProofMsg{request: "current", generation: 1, result: tt.result})
			got := updated.(Model)
			if got.profileCompletion != tt.outcome || !strings.Contains(got.View(), tt.copy) {
				t.Fatalf("completion = %q view=%q", got.profileCompletion, got.View())
			}
		})
	}
}

func TestProfileCompletionRendersEveryOutcomeWithoutColor(t *testing.T) {
	for _, outcome := range []profileCompletionOutcome{profileCompletionOmitted, profileCompletionCancelled, profileCompletionFailed, profileCompletionSuccessful} {
		m := NewModel(&profileStoreStub{})
		m.screen, m.profileCompletion, m.noColor = screenProfileCompletion, outcome, true
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		view := updated.(Model).View()
		if !strings.Contains(view, string(outcome)) || strings.Contains(view, "\x1b[") {
			t.Fatalf("outcome %q not accessible without color: %q", outcome, view)
		}
	}
}

func TestProfileProofAndCompletionViewportsRemainBounded(t *testing.T) {
	for _, tt := range []struct {
		name    string
		width   int
		height  int
		noColor bool
	}{
		{"wide", 120, 40, false},
		{"standard no color", 80, 24, true},
		{"narrow", 40, 16, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(&profileStoreStub{})
			m.screen, m.step8Action.request.Profile, m.profileProof.enabled, m.noColor = screenProfileStep8Action, testProfile("saved"), true, tt.noColor
			updated, _ := m.Update(tea.WindowSizeMsg{Width: tt.width, Height: tt.height})
			proof := updated.(Model).View()
			assertProfileFrameBounds(t, proof, tt.width, tt.height)
			if tt.noColor && (strings.Contains(proof, "\x1b[") || !strings.Contains(proof, "[INFO]")) {
				t.Fatalf("NO_COLOR proof lost semantic feedback: %q", proof)
			}

			m = updated.(Model)
			m.screen, m.profileCompletion = screenProfileCompletion, profileCompletionSuccessful
			completion := m.View()
			reachable := strings.Contains(completion, "successful")
			for range 12 {
				updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
				m = updated.(Model)
				completion = m.View()
				reachable = reachable || strings.Contains(completion, "successful")
			}
			assertProfileFrameBounds(t, completion, tt.width, tt.height)
			if !reachable {
				t.Fatalf("completion outcome is not reachable: %q", completion)
			}
		})
	}
}

func assertProfileFrameBounds(t *testing.T, view string, width, height int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) > height {
		t.Fatalf("frame has %d lines, limit %d", len(lines), height)
	}
	for _, line := range lines {
		if lipgloss.Width(line) > width {
			t.Fatalf("frame line width %d exceeds %d: %q", lipgloss.Width(line), width, line)
		}
	}
}
