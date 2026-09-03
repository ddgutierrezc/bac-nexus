package tui

import (
	"strings"
	"testing"

	"bac-nexus/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
)

func TestProfileScreenPrimitivesRenderFieldsActionsAndSemanticFeedback(t *testing.T) {
	m := NewModel(nil)
	theme := newHomeTheme(true)
	body := strings.Join([]string{
		m.profileField("▸", "Host", "ibmi.example.test"),
		m.profileAction("▸", "CONNECT AND SAVE"),
		m.profileFeedback("[ERR]", "Host is invalid", theme),
	}, "\n")
	for _, want := range []string{"▸ Host: ibmi.example.test", "▸ [ CONNECT AND SAVE ]", "[ERR] Host is invalid"} {
		if !strings.Contains(body, want) {
			t.Fatalf("primitive output missing %q: %q", want, body)
		}
	}
}

func TestValidateEditProfileMapsAuthoritativeValidationOrderToSemanticFields(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*profile.Profile)
		field  string
	}{
		{"name", func(p *profile.Profile) { p.Name = "!" }, profileValidationFieldName},
		{"endpoint", func(p *profile.Profile) { p.Host = "" }, profileValidationFieldEndpoint},
		{"username", func(p *profile.Profile) { p.Username = "invalid user" }, profileValidationFieldUsername},
		{"host key", func(p *profile.Profile) { p.HostKeyFingerprint = "invalid" }, profileValidationFieldHostKey},
		{"java home", func(p *profile.Profile) { p.JavaHome = "/invalid" }, profileValidationFieldJavaHome},
		{"mapepire", func(p *profile.Profile) { p.MapepireJAR = "relative.jar" }, profileValidationFieldMapepireJAR},
		{"credential mode", func(p *profile.Profile) { p.CredentialMode = "invalid" }, profileValidationFieldCredentialMode},
	} {
		t.Run(tt.name, func(t *testing.T) {
			original := testProfile("edit-profile")
			if tt.field != profileValidationFieldHostKey {
				original.SchemaVersion = profile.SchemaVersionV3
			}
			candidate := original
			tt.mutate(&candidate)

			validation := validateEditProfile(original, candidate)
			if validation == nil || validation.FieldID != tt.field || validation.Cause == nil {
				t.Fatalf("validation = %#v, want field %q with cause", validation, tt.field)
			}
		})
	}
}

func TestEditInvalidSaveIsFocusableBlockedAndRevealsFirstInvalidField(t *testing.T) {
	store := &recordingProfileStore{}
	m := NewModel(store)
	original := testProfile("edit-profile")
	original.SchemaVersion = profile.SchemaVersionV3
	m.beginForm(original, screenDetail)
	m.form[0].input.SetValue("!")
	for range len(m.form) {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = updated.(Model)
	}

	view := m.View()
	if !strings.Contains(view, "▸ [ GUARDAR ]") {
		t.Fatalf("Save is not focusable: %q", view)
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || store.updates != 0 {
		t.Fatalf("invalid Save started persistence: cmd=%v updates=%d", cmd, store.updates)
	}
	if m.formValidation == nil || m.formValidation.FieldID != profileValidationFieldName || m.focusIndex() != 0 {
		t.Fatalf("invalid Save did not focus name validation: %#v focus=%d", m.formValidation, m.focusIndex())
	}
	if view = m.View(); !strings.Contains(view, "[ERR]") {
		t.Fatalf("invalid Save feedback is not semantic: %q", view)
	}
}

func TestEditAuthoritativeValidationOrderWinsOverSyntacticParsing(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*Model)
		field  string
		focus  int
	}{
		{
			name: "invalid name before non-numeric port",
			mutate: func(m *Model) {
				m.form[0].input.SetValue("!")
				m.form[2].input.SetValue("not-a-port")
			},
			field: profileValidationFieldName,
			focus: 0,
		},
		{
			name: "invalid username before unsupported trust",
			mutate: func(m *Model) {
				m.form[3].input.SetValue("invalid user")
				m.form[5].input.SetValue("unsupported")
			},
			field: profileValidationFieldUsername,
			focus: 3,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := &recordingProfileStore{}
			m := NewModel(store)
			original := testProfile("edit-profile")
			original.SchemaVersion = profile.SchemaVersionV3
			m.beginForm(original, screenDetail)
			tt.mutate(&m)
			m.formFocus = len(m.form)

			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = updated.(Model)
			if cmd != nil || store.updates != 0 {
				t.Fatalf("invalid Save started persistence: cmd=%v updates=%d", cmd, store.updates)
			}
			if m.formValidation == nil || m.formValidation.FieldID != tt.field || m.focusIndex() != tt.focus {
				t.Fatalf("invalid Save did not prioritize %q feedback: %#v focus=%d", tt.field, m.formValidation, m.focusIndex())
			}
		})
	}
}

func TestEditEndpointValidationClearsWhenEitherEndpointFieldChanges(t *testing.T) {
	for _, tt := range []struct {
		name       string
		fieldIndex int
		wantClear  bool
	}{
		{"host", 1, true},
		{"port", 2, true},
		{"unrelated username", 3, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(&profileStoreStub{})
			original := testProfile("edit-profile")
			original.SchemaVersion = profile.SchemaVersionV3
			m.beginForm(original, screenDetail)
			m.formValidation = &profileValidation{FieldID: profileValidationFieldEndpoint, MessageID: "profile.validation.endpoint"}
			m.formFocus = tt.fieldIndex

			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
			m = updated.(Model)
			if got := m.formValidation == nil; got != tt.wantClear {
				t.Fatalf("endpoint validation cleared = %t, want %t", got, tt.wantClear)
			}
			if got := strings.Contains(m.View(), m.text("profile.validation.endpoint", nil)); got == tt.wantClear {
				t.Fatalf("endpoint feedback visible = %t, want %t", got, !tt.wantClear)
			}
		})
	}
}

func TestEditSaveUpdatesMetadataAndCancelDiscardsDraft(t *testing.T) {
	original := testProfile("edit-profile")
	original.SchemaVersion = profile.SchemaVersionV3
	store := &recordingProfileStore{}
	m := NewModel(store)
	m.beginForm(original, screenDetail)
	m.form[3].input.SetValue("NEWUSER")
	m.formFocus = len(m.form)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("valid Save did not return a store command")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if store.updates != 1 || store.updated.Username != "NEWUSER" || m.screen != screenList {
		t.Fatalf("Save did not update only metadata: updates=%d profile=%+v screen=%d", store.updates, store.updated, m.screen)
	}

	cancelStore := &recordingProfileStore{}
	cancel := NewModel(cancelStore)
	cancel.beginForm(original, screenDetail)
	cancel.form[1].input.SetValue("discarded.example")
	cancel.formFocus = len(cancel.form) + 1
	updated, cmd = cancel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	cancel = updated.(Model)
	if cmd != nil || cancelStore.updates != 0 || cancel.screen != screenDetail {
		t.Fatalf("Cancel persisted or changed screen: cmd=%v updates=%d screen=%d", cmd, cancelStore.updates, cancel.screen)
	}
}

func TestProfileScreenEditRuntimeFramesPreserveActionsValidationAndOverflow(t *testing.T) {
	original := testProfile("edit-profile")
	original.SchemaVersion = profile.SchemaVersionV3

	for _, frame := range []struct {
		name          string
		width, height int
	}{
		{name: "validation focus", width: 80, height: 24},
		{name: "narrow overflow", width: 40, height: 16},
	} {
		t.Run(frame.name, func(t *testing.T) {
			m := NewModel(&recordingProfileStore{})
			m.beginForm(original, screenDetail)
			updated, _ := m.Update(tea.WindowSizeMsg{Width: frame.width, Height: frame.height})
			m = updated.(Model)

			if frame.name == "validation focus" {
				m.form[0].input.SetValue("!")
				m.formFocus = len(m.form)
				updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
				m = updated.(Model)
				if cmd != nil || m.formValidation == nil || m.focusIndex() != 0 {
					t.Fatalf("invalid Save did not remain blocked and focus name: cmd=%v validation=%#v focus=%d", cmd, m.formValidation, m.focusIndex())
				}
				if !strings.Contains(m.View(), "[ERR]") {
					t.Fatalf("validation frame did not render semantic feedback: %q", m.View())
				}
			} else {
				view := m.View()
				if !strings.Contains(view, "▼ más") {
					t.Fatalf("narrow Edit frame omitted overflow disclosure: %q", view)
				}
				for range len(m.form) {
					updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
					m = updated.(Model)
				}
				view = m.View()
				if !strings.Contains(view, "▸ [ GUARDAR ]") {
					t.Fatalf("narrow Edit frame did not reveal Save: %q", view)
				}
				assertProfileFrameBounds(t, view, frame.width, frame.height)
			}
		})
	}
}

type recordingProfileStore struct {
	profileStoreStub
	updates int
	updated profile.Profile
}

func (s *recordingProfileStore) Update(p profile.Profile, previous string) (profile.ProfileUpdateResult, error) {
	s.updates++
	s.updated = p
	return s.profileStoreStub.Update(p, previous)
}
