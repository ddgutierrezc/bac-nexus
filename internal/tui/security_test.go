package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/credential"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type securityServiceStub struct {
	status    credential.Presence
	set       func(context.Context, string) (configuration.CredentialOutcome, error)
	deleted   string
	enrolled  string
	inspected bool
	evidence  string
}

func (s *securityServiceStub) Status(context.Context, string) (credential.Presence, error) {
	return s.status, nil
}
func (s *securityServiceStub) Set(ctx context.Context, name string) (configuration.CredentialOutcome, error) {
	if s.set != nil {
		return s.set(ctx, name)
	}
	return configuration.CredentialOutcomeStored, nil
}
func (s *securityServiceStub) Rotate(context.Context, string) (configuration.CredentialOutcome, error) {
	return configuration.CredentialOutcomeRotated, nil
}
func (s *securityServiceStub) Delete(_ context.Context, name, confirmation string) (configuration.CredentialOutcome, error) {
	if confirmation != "delete credential "+name {
		return "", configuration.ErrConfirmationRequired
	}
	s.deleted = name
	return configuration.CredentialOutcomeDeleted, nil
}
func (s *securityServiceStub) Migrate(context.Context, string, bool) (configuration.CredentialOutcome, error) {
	return configuration.CredentialOutcomeMigrated, nil
}
func (s *securityServiceStub) EnrollManual(_ context.Context, name, fingerprint, provenance, confirmation string) (configuration.TrustOutcome, error) {
	if confirmation != "enroll "+fingerprint || provenance == "" {
		return "", configuration.ErrConfirmationRequired
	}
	s.enrolled = name
	return configuration.TrustOutcomeEnrolled, nil
}
func (s *securityServiceStub) InspectAndEnroll(_ context.Context, name string, warned bool, confirmation string) (configuration.TrustOutcome, error) {
	if !warned {
		return "", configuration.ErrWarningRequired
	}
	if confirmation != "enroll inspected" {
		return "", configuration.ErrConfirmationRequired
	}
	s.inspected = true
	return configuration.TrustOutcomeEnrolled, nil
}

func (s *securityServiceStub) InspectTOFU(context.Context, string) (string, error) {
	if s.evidence == "" {
		s.evidence = "SHA256:observed"
	}
	return s.evidence, nil
}
func (s *securityServiceStub) EnrollTOFU(_ context.Context, name, evidence, confirmation string) (configuration.TrustOutcome, error) {
	if confirmation != "enroll "+evidence {
		return "", configuration.ErrConfirmationRequired
	}
	s.inspected, s.enrolled = true, name
	return configuration.TrustOutcomeEnrolled, nil
}

func TestSecurityModelKeepsCredentialMaterialOutOfStateMessagesAndView(t *testing.T) {
	const sentinel = "sentinel-secret-123"
	services := &securityServiceStub{status: credential.PresenceAbsent}
	services.set = func(context.Context, string) (configuration.CredentialOutcome, error) {
		return configuration.CredentialOutcomeStored, nil
	}
	m := NewSecurityModel(context.Background(), "dev", services)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil || updated.(SecurityModel).screen != securityProgress {
		t.Fatalf("set did not enter progress screen: screen=%v cmd=%v", updated.(SecurityModel).screen, cmd != nil)
	}
	msg := cmd()
	if strings.Contains(msgText(msg), sentinel) || strings.Contains(updated.(SecurityModel).View(), sentinel) {
		t.Fatal("credential material crossed the TUI boundary")
	}
	updated, _ = updated.(SecurityModel).Update(msg)
	if !strings.Contains(updated.(SecurityModel).View(), "Credential stored") {
		t.Fatalf("opaque outcome was not rendered: %q", updated.(SecurityModel).View())
	}
}

func TestSecurityModelRequiresExactCredentialConfirmationAndSupportsCancellation(t *testing.T) {
	services := &securityServiceStub{status: credential.PresencePresent}
	m := NewSecurityModel(context.Background(), "dev", services)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if updated.(SecurityModel).screen != securityConfirmCredential {
		t.Fatalf("delete did not enter credential confirmation: %v", updated.(SecurityModel).screen)
	}
	updated, cmd := updated.(SecurityModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if updated.(SecurityModel).screen != securityConfirmCredential || services.deleted != "" {
		t.Fatal("single-key credential confirmation triggered deletion")
	}
	updated, cmd = updated.(SecurityModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd != nil || updated.(SecurityModel).screen != securityMenu {
		t.Fatalf("cancel did not return safely: screen=%v cmd=%v", updated.(SecurityModel).screen, cmd != nil)
	}

	started := make(chan struct{})
	canceled := make(chan struct{})
	services.set = func(ctx context.Context, _ string) (configuration.CredentialOutcome, error) {
		close(started)
		<-ctx.Done()
		close(canceled)
		return "", ctx.Err()
	}
	updated, cmd = updated.(SecurityModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil || updated.(SecurityModel).screen != securityProgress {
		t.Fatal("set did not enter cancellable progress state")
	}
	go cmd()
	<-started
	updated, _ = updated.(SecurityModel).Update(tea.KeyMsg{Type: tea.KeyEscape})
	if updated.(SecurityModel).screen != securityMenu {
		t.Fatalf("escape did not cancel safely: %v", updated.(SecurityModel).screen)
	}
	<-canceled
}

func TestSecurityModelShowsStatusAndTrustActionsWithoutPersistingInspection(t *testing.T) {
	services := &securityServiceStub{status: credential.PresenceUnavailable}
	m := NewSecurityModel(context.Background(), "dev", services)
	updated, cmd := m.Update(credentialStatusMsg{presence: credential.PresenceUnavailable})
	if cmd != nil || !strings.Contains(updated.(SecurityModel).View(), "unavailable") {
		t.Fatalf("status classification missing: %q", updated.(SecurityModel).View())
	}
	updated, cmd = updated.(SecurityModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if cmd == nil || updated.(SecurityModel).screen != securityProgress {
		t.Fatalf("TOFU inspection did not start: screen=%v cmd=%v", updated.(SecurityModel).screen, cmd != nil)
	}
	msg := cmd()
	updated, _ = updated.(SecurityModel).Update(msg)
	if services.inspected || !strings.Contains(updated.(SecurityModel).View(), services.evidence) {
		t.Fatal("inspection must show evidence without persisting trust")
	}
	if _, err := services.InspectAndEnroll(context.Background(), "dev", false, "enroll inspected"); !errors.Is(err, configuration.ErrWarningRequired) {
		t.Fatalf("unwarned inspection error = %v", err)
	}
}

func TestSecurityModelTOFUShowsEvidenceBeforeExactEnrollment(t *testing.T) {
	services := &securityServiceStub{status: credential.PresenceAbsent}
	m := NewSecurityModel(context.Background(), "dev", services)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if cmd == nil || updated.(SecurityModel).screen != securityProgress {
		t.Fatal("TOFU inspection did not start as a separate step")
	}
	updated, _ = updated.(SecurityModel).Update(cmd())
	if !strings.Contains(updated.(SecurityModel).View(), services.evidence) || services.inspected {
		t.Fatalf("evidence was not displayed before enrollment: %q", updated.(SecurityModel).View())
	}
	model := updated.(SecurityModel)
	model.confirm.SetValue("enroll " + services.evidence)
	updated = model
	updated, cmd = updated.(SecurityModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("exact TOFU confirmation did not start enrollment")
	}
	if _, ok := cmd().(securityOutcomeMsg); !ok || !services.inspected {
		t.Fatal("TOFU enrollment did not complete with opaque outcome")
	}
}

func TestSecurityViewportKeepsFunctionalContentReachableAtNarrowSizes(t *testing.T) {
	for _, size := range []struct{ width, height int }{{80, 24}, {40, 16}} {
		t.Run("size", func(t *testing.T) {
			services := &securityServiceStub{status: credential.PresenceAbsent, evidence: strings.Repeat("SHA256:observed-evidence ", 60)}
			m := NewSecurityModel(context.Background(), "profile-with-a-long-name", services)
			updated, _ := m.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
			m = updated.(SecurityModel)
			view := m.View()
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
			if cmd == nil {
				t.Fatal("TOFU inspection did not start")
			}
			updated, _ = updated.(SecurityModel).Update(cmd())
			m = updated.(SecurityModel)
			view = m.View()
			if !strings.Contains(view, "▼ más") {
				t.Fatalf("initial hidden content has no disclosure:\n%s", view)
			}
			for _, line := range strings.Split(view, "\n") {
				if lipgloss.Width(line) > size.width {
					t.Fatalf("line overflowed %d: %q", size.width, line)
				}
			}
			seen := view
			for i := 0; i < 120; i++ {
				updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
				m = updated.(SecurityModel)
				seen += "\n" + m.View()
			}
			view = m.View()
			normalizeRuntimeViews := func(views string) string {
				return strings.Join(strings.Fields(ansiEscape.ReplaceAllString(views, "")), " ")
			}
			reachable := normalizeRuntimeViews(seen)
			for _, want := range []string{"Type enroll", "then press enter."} {
				if !strings.Contains(reachable, want) {
					t.Fatalf("menu instruction %q is unreachable:\n%s", want, view)
				}
			}
			if !strings.Contains(view, "▲ más") || strings.Contains(view, "▼ más") {
				t.Fatalf("bottom indicators incorrect:\n%s", view)
			}
			trust := NewSecurityModel(context.Background(), "profile-with-a-long-name", services)
			updated, _ = trust.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
			updated, _ = updated.(SecurityModel).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
			trust = updated.(SecurityModel)
			trustSeen := trust.View()
			for i := 0; i < 120; i++ {
				updated, _ = trust.Update(tea.KeyMsg{Type: tea.KeyPgDown})
				trust = updated.(SecurityModel)
				trustSeen += "\n" + trust.View()
			}
			trustReachable := normalizeRuntimeViews(trustSeen)
			for _, want := range []string{"Manual verified host-key enrollment", "Fingerprint:", "Provenance:", "Exact confirmation:"} {
				if !strings.Contains(trustReachable, want) {
					t.Fatalf("trust behavior changed or content was unreachable: %q", want)
				}
			}
		})
	}
}

func TestSecurityConfirmationInputsAcceptSpaces(t *testing.T) {
	services := &securityServiceStub{status: credential.PresencePresent, evidence: "SHA256:observed"}
	for _, tt := range []struct{ key, confirmation string }{{"d", "delete credential dev"}, {"o", "enroll SHA256:observed"}} {
		t.Run(tt.key, func(t *testing.T) {
			m := NewSecurityModel(context.Background(), "dev", services)
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})
			if tt.key == "o" {
				updated, _ = updated.(SecurityModel).Update(cmd())
			}
			m = updated.(SecurityModel)
			for _, r := range tt.confirmation {
				updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
				m = updated.(SecurityModel)
			}
			updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd == nil || updated.(SecurityModel).screen != securityProgress {
				t.Fatalf("confirmation with spaces was not accepted: %q", m.confirm.Value())
			}
		})
	}
}

func msgText(msg tea.Msg) string {
	if outcome, ok := msg.(securityOutcomeMsg); ok {
		return outcome.text
	}
	return ""
}
