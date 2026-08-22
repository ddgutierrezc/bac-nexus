package tui

import (
	"errors"
	"strings"
	"testing"

	"bac-nexus/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
)

type profileStoreStub struct {
	profiles []profile.Profile
	deleted  string
}

func (s *profileStoreStub) Save(p profile.Profile) (string, error) {
	s.profiles = append(s.profiles, p)
	return p.Name + ".json", nil
}
func (s *profileStoreStub) List(limit int) ([]profile.Profile, error) {
	if limit > len(s.profiles) {
		limit = len(s.profiles)
	}
	return append([]profile.Profile(nil), s.profiles[:limit]...), nil
}
func (s *profileStoreStub) Read(name string) (profile.Profile, error) {
	for _, p := range s.profiles {
		if p.Name == name {
			return p, nil
		}
	}
	return profile.Profile{}, errors.New("not found")
}
func (s *profileStoreStub) Update(p profile.Profile, previous string) (profile.ProfileUpdateResult, error) {
	return profile.ProfileUpdateResult{ReplacementCommitted: true}, nil
}
func (s *profileStoreStub) Delete(name string, confirmation profile.DeleteConfirmation) (profile.ProfileDeleteResult, error) {
	s.deleted = name
	return profile.ProfileDeleteResult{Deleted: true}, nil
}
func (s *profileStoreStub) Restore(string) error { return nil }

func testProfile(name string) profile.Profile {
	return profile.Profile{Name: name, Host: "ibmi.example", Port: 22, Username: "operator", HostKeyFingerprint: "SHA256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", HostKeyTrust: profile.HostKeyTrustVerified, CredentialMode: profile.CredentialModePrompt}
}

func TestModelNavigationAndEmptyState(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	if got := m.View(); !strings.Contains(got, "No profiles configured") {
		t.Fatalf("empty view = %q", got)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if updated.(Model).screen != screenForm {
		t.Fatalf("screen = %v, want form", updated.(Model).screen)
	}
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEscape})
	if updated.(Model).screen != screenList {
		t.Fatalf("escape did not return to list")
	}
}

func TestModelDetailDeleteAndBack(t *testing.T) {
	store := &profileStoreStub{profiles: []profile.Profile{testProfile("dev")}}
	m := NewModel(store)
	updated, cmd := m.Update(profilesMsg{profiles: store.profiles})
	if cmd != nil {
		t.Fatal("list message unexpectedly scheduled a command")
	}
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.(Model).screen != screenDetail {
		t.Fatalf("enter screen = %v", updated.(Model).screen)
	}
	if !strings.Contains(updated.(Model).View(), "dev") {
		t.Fatal("detail omitted profile name")
	}
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEscape})
	if updated.(Model).screen != screenList {
		t.Fatal("back did not return to list")
	}
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if updated.(Model).screen != screenConfirm {
		t.Fatalf("delete screen = %v, want confirmation", updated.(Model).screen)
	}
}

func TestModelDeleteRequiresExactOperatorConfirmation(t *testing.T) {
	store := &profileStoreStub{profiles: []profile.Profile{testProfile("dev")}}
	m := NewModel(store)
	updated, _ := m.Update(profilesMsg{profiles: store.profiles})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(Model)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if updated.(Model).screen != screenConfirm || store.deleted != "" {
		t.Fatal("single-key confirmation triggered deletion")
	}
	m = NewModel(store)
	updated, _ = m.Update(profilesMsg{profiles: store.profiles})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(Model)
	m.confirmInput.SetValue("delete dev")
	updated = m
	updated, cmd = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || cmd() == nil || store.deleted != "dev" {
		t.Fatalf("exact confirmation did not delete profile: cmd=%v deleted=%q", cmd != nil, store.deleted)
	}
}

func TestModelCreateReloadsCommittedProfile(t *testing.T) {
	store := &profileStoreStub{}
	m := NewModel(store)
	p := testProfile("created")
	store.profiles = append(store.profiles, p)
	updated, cmd := m.Update(operationMsg{text: "Profile created"})
	if cmd == nil || updated.(Model).screen != screenList {
		t.Fatal("create completion did not schedule a list reload")
	}
	updated, _ = updated.(Model).Update(cmd())
	if !strings.Contains(updated.(Model).View(), "created") {
		t.Fatalf("reloaded profile missing from view: %q", updated.(Model).View())
	}
}

func TestModelResizeAndNoColorView(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 20, Height: 8})
	m = updated.(Model)
	if m.width != 20 || m.height != 8 {
		t.Fatalf("size = %dx%d", m.width, m.height)
	}
	view := m.View()
	if strings.Contains(view, "\x1b[") {
		t.Fatalf("narrow view contains ANSI escape: %q", view)
	}
}
