package tui

import (
	"testing"

	"bac-nexus/internal/credential"
	"bac-nexus/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
)

type credentialStatusStub struct{ presence credential.Presence }

func (s credentialStatusStub) Status(string) (credential.Presence, error) { return s.presence, nil }

func TestProfileCredentialsBlocksUnavailableKeyring(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.screen, m.profileDraftName = screenProfileCredentials, "CRI400F"
	m.credentialMode, m.credentialFocus = profile.CredentialModeKeyring, profileCredentialsFocusContinue
	m.credentialStatus = credentialStatusStub{presence: credential.PresenceUnavailable}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("keyring status command is missing")
	}
	result, _ := updated.(Model).Update(cmd())
	got := result.(Model)
	if got.screen != screenProfileCredentials || got.credentialMode != profile.CredentialModeKeyring || got.status == "" {
		t.Fatalf("unavailable keyring advanced or lost selection: screen=%v mode=%q status=%q", got.screen, got.credentialMode, got.status)
	}
}

func TestProfileCredentialsPromptAdvancesWithoutCredentialMaterial(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.screen, m.profileDraftName = screenProfileCredentials, "CRI400F"
	m.credentialMode, m.credentialFocus = profile.CredentialModePrompt, profileCredentialsFocusContinue
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || updated.(Model).screen != screenProfileReview {
		t.Fatalf("prompt selection = screen %v, command %v; want review without command", updated.(Model).screen, cmd)
	}
}

func TestProfileCredentialsEscapeReturnsHome(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.screen = screenProfileCredentials

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil || updated.(Model).screen != screenHome {
		t.Fatalf("escape = screen %v, command %v; want home without command", updated.(Model).screen, cmd)
	}
}
