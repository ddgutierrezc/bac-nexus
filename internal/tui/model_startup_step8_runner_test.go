package tui

import (
	"context"
	"testing"
	"time"

	"bac-nexus/internal/hostidentity"
	tea "github.com/charmbracelet/bubbletea"
)

type startupInspector struct{}

func (startupInspector) InspectHostKey(context.Context, string, int) (hostidentity.Candidate, error) {
	return hostidentity.Candidate{}, nil
}

func TestStartupModelComposesInspectorAndStep8RunnerWithoutInvocation(t *testing.T) {
	runner := &countingStep8Runner{}
	inspector := startupInspector{}
	m := newModelWithIdentityInspectorAndStep8Runner(&profileStoreStub{}, BuildInfo{Version: "v1", Revision: "r1"}, inspector, runner, context.Background(), time.Second)

	if m.identityInspector != inspector {
		t.Fatal("startup model did not retain the host identity inspector")
	}
	if m.step8Runner != runner {
		t.Fatal("startup model did not retain the Step 8 runner")
	}
	if cmd := m.Init(); cmd != nil {
		_ = cmd()
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if view := updated.(Model).View(); view == "" {
		t.Fatal("startup model rendered an empty view")
	}
	if runner.calls != 0 {
		t.Fatalf("startup model construction, Init, Update, and View invoked the Step 8 runner %d times", runner.calls)
	}
}
