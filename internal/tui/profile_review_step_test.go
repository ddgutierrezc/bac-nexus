package tui

import (
	"context"
	"testing"

	"bac-nexus/internal/configuration"
	"bac-nexus/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
)

type profileCreatorStub struct {
	requests []configuration.CreateProfileRequest
}

func (s *profileCreatorStub) Create(_ context.Context, request configuration.CreateProfileRequest) (configuration.CreateProfileResult, error) {
	s.requests = append(s.requests, request)
	return configuration.CreateProfileResult{RequestID: request.RequestID, Generation: request.Generation, DraftDigest: request.DraftDigest, Profile: request.Profile}, nil
}

func TestProfileReviewIgnoresStaleSaveResult(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.screen, m.createPending, m.createRequest, m.createGeneration = screenProfileReview, true, "current", 2
	updated, _ := m.Update(profileCreateMsg{request: "stale", generation: 1})
	got := updated.(Model)
	if got.screen != screenProfileReview || !got.createPending {
		t.Fatalf("stale result changed pending save: screen=%v pending=%t", got.screen, got.createPending)
	}
}

func TestProfileReviewSavesOnceAndHandsOffExactProfile(t *testing.T) {
	creator := &profileCreatorStub{}
	m := NewModel(&profileStoreStub{})
	m.screen, m.profileCreator, m.profileDraftName = screenProfileReview, creator, "CRI400F"
	m.connectionDraft = profileConnectionDraft{host: "ibmi.example.test", username: "USER", port: 22}
	m.identityDraft = profileIdentityDraft{algorithm: "ssh-ed25519", fingerprint: testCandidate.Fingerprint, trustMethod: profile.HostKeyTrustTOFU}
	m.credentialMode, m.profileReviewFocus = profile.CredentialModePrompt, profileReviewFocusSave
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("save command is missing")
	}
	if !updated.(Model).createPending || updated.(Model).createCancel == nil {
		t.Fatal("save did not retain loading state and child cancellation")
	}
	again, duplicate := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if duplicate != nil {
		t.Fatal("pending save emitted a duplicate command")
	}
	finished, _ := again.(Model).Update(cmd())
	got := finished.(Model)
	if len(creator.requests) != 1 || got.screen != screenProfileStep8Action || got.step8Action.request.Profile != creator.requests[0].Profile {
		t.Fatalf("save/handoff = requests %d screen %v profile %#v", len(creator.requests), got.screen, got.step8Action.request.Profile)
	}
}
