package tui

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"bac-nexus/internal/profile"
	"github.com/charmbracelet/lipgloss"
)

var ansiEscape = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

type profileStoreStub struct {
	profiles []profile.Profile
	deleted  string
}

func (s *profileStoreStub) Save(profile.Profile) (string, error) { return "profile", nil }
func (s *profileStoreStub) List(int) ([]profile.Profile, error)  { return s.profiles, nil }
func (s *profileStoreStub) Read(name string) (profile.Profile, error) {
	for _, p := range s.profiles {
		if p.Name == name {
			return p, nil
		}
	}
	return profile.Profile{}, errors.New("not found")
}
func (s *profileStoreStub) Update(p profile.Profile, _ string) (profile.ProfileUpdateResult, error) {
	return profile.ProfileUpdateResult{ReplacementCommitted: true}, nil
}
func (s *profileStoreStub) Delete(name string, _ profile.DeleteConfirmation) (profile.ProfileDeleteResult, error) {
	s.deleted = name
	return profile.ProfileDeleteResult{Deleted: true}, nil
}
func (s *profileStoreStub) Restore(string) error { return nil }
func testProfile(name string) profile.Profile {
	return profile.Profile{Name: name, Host: "ibmi.example", Port: 22, Username: "operator", HostKeyFingerprint: "SHA256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", HostKeyTrust: profile.HostKeyTrustVerified, CredentialMode: profile.CredentialModePrompt}
}

func assertProfileFrameBounds(t *testing.T, view string, width, height int) {
	t.Helper()
	if lipgloss.Height(view) > height {
		t.Fatalf("frame height %d exceeds %d", lipgloss.Height(view), height)
	}
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > width {
			t.Fatalf("frame width %d exceeds %d: %q", lipgloss.Width(line), width, line)
		}
	}
}

func alphaNumericOnly(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, value)
}
