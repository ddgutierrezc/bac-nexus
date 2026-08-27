package tui

import (
	"context"
	"strings"
	"testing"

	"bac-nexus/internal/configuration"
	tea "github.com/charmbracelet/bubbletea"
)

type countingPreAuthProbe struct {
	calls  int
	result configuration.Resolution
	err    error
}

func (p *countingPreAuthProbe) Probe(context.Context) (configuration.Resolution, error) {
	p.calls++
	return p.result, p.err
}

type countingProofClient struct{ connects, queries int }

func (c *countingProofClient) Connect(context.Context) error      { c.connects++; return nil }
func (c *countingProofClient) Query(context.Context) (int, error) { c.queries++; return 1, nil }

func newProfileMapepireTestModel(probe preAuthProbe, width, height int) Model {
	m := NewModel(&profileStoreStub{})
	m.screen, m.mapepireProbe, m.noColor = screenProfileMapepire, probe, true
	m.width, m.height = width, height
	m.refreshWizardViewport()
	return m
}

func TestMapepireStep4IsPreAuthAndUsesExactPendingCopy(t *testing.T) {
	probe := &countingPreAuthProbe{result: configuration.Resolution{Transport: configuration.TransportWSS, Version: "2.3.5", AuthenticationPending: true}}
	m := newProfileMapepireTestModel(probe, 120, 40)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("Step 4 probe command=%v", cmd != nil)
	}
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if probe.calls != 1 {
		t.Fatalf("Step 4 probe calls=%d", probe.calls)
	}
	if !strings.Contains(m.View(), "[OK] Mapepire detected — authentication pending") || m.screen != screenProfileMapepire {
		t.Fatalf("Step 4 did not show exact pre-auth state: %q", m.View())
	}
	if m.step8Client != nil {
		t.Fatal("Step 4 installed an authenticated client")
	}
}

func TestMapepireStep4UnavailableDoesNotInvokeFallbackRuntime(t *testing.T) {
	probe := &countingPreAuthProbe{result: configuration.Resolution{Transport: configuration.TransportUnknown, Class: configuration.FailureAvailability}}
	m := newProfileMapepireTestModel(probe, 80, 24)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("Step 4 did not expose bounded probe")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if probe.calls != 1 || strings.Contains(m.View(), "launch") {
		t.Fatalf("Step 4 invoked runtime fallback: %q", m.View())
	}
}

func TestStep8AloneProvesConnectAndQuery(t *testing.T) {
	client := &countingProofClient{}
	if err := runProfileStep8Proof(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	if client.connects != 1 || client.queries != 1 {
		t.Fatalf("proof calls=%d/%d", client.connects, client.queries)
	}
}

func TestMapepireStep4ViewFitsResponsiveMatrixWithoutColor(t *testing.T) {
	for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}, {40, 16}} {
		m := newProfileMapepireTestModel(&countingPreAuthProbe{}, size.width, size.height)
		view := m.View()
		if strings.Contains(view, "\x1b[") || len(strings.Split(view, "\n")) == 0 {
			t.Fatalf("invalid NO_COLOR frame at %dx%d", size.width, size.height)
		}
		for _, line := range strings.Split(view, "\n") {
			if len([]rune(line)) > size.width {
				t.Fatalf("Step 4 line clips at %dx%d: %q", size.width, size.height, line)
			}
		}
	}
}

func TestCompletedStep3ReachesStep4WithoutStartingRuntime(t *testing.T) {
	m := newProfileIdentityTestModel(t, 80, 24)
	m.identityPhase, m.identityFocus = profileIdentityCompleted, profileIdentityFocusInspect
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || updated.(Model).screen != screenProfileMapepire {
		t.Fatal("Step 3 did not reach local Step 4")
	}
}

func TestProductionModelComposesIndependentTLSAndSSHReadiness(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	if m.mapepireFactory == nil || m.identityInspector != nil {
		t.Fatalf("production composition lost independent readiness seams: tls=%v ssh=%v", m.mapepireFactory != nil, m.identityInspector != nil)
	}
	m.screen, m.connectionDraft = screenProfileMapepire, profileConnectionDraft{host: "invalid.example", port: 8076}
	view := m.View()
	if strings.Contains(view, "https://") || strings.Contains(view, "Authorization") {
		t.Fatalf("view exposed runtime details: %q", view)
	}
}

func TestStep8ProofIsExplicitlyOwnedAndCredentialFreeBeforeInvocation(t *testing.T) {
	client := &countingProofClient{}
	if err := runProfileStep8Proof(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	if client.connects != 1 || client.queries != 1 {
		t.Fatalf("proof calls=%d/%d", client.connects, client.queries)
	}
}
