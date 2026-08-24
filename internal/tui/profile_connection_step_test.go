package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func newProfileConnectionTestModel(t *testing.T, store *profileStoreStub, width, height int) Model {
	t.Helper()
	m := newProfileStepTestModel(store, width, height)
	m.profileName.SetValue("CRI400F")
	m.profileFocus = profileFocusContinue
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Step 1 did not expose its local accepted seam")
	}
	accepted, _ := updated.(Model).Update(cmd())
	return accepted.(Model)
}

func connectionKey(t *testing.T, m Model, key tea.KeyType) Model {
	t.Helper()
	updated, _ := m.Update(tea.KeyMsg{Type: key})
	return updated.(Model)
}

func TestProfileConnectionOpensWithAcceptedNameAndDefaultPort(t *testing.T) {
	m := newProfileConnectionTestModel(t, &profileStoreStub{}, 120, 40)
	if m.screen != screenProfileConnection || m.profileDraftName != "CRI400F" || m.connectionPort.Value() != "22" {
		t.Fatalf("Step 2 state = screen:%v name:%q port:%q", m.screen, m.profileDraftName, m.connectionPort.Value())
	}
	view := m.View()
	for _, want := range []string{"BAC NEXUS", "PERFIL: CRI400F", "ESTADO: CONFIGURANDO", connectionPanelTitle, connectionStepIndicator, "Conexión con IBM i", "Indica cómo localizar el IBM i y qué usuario utilizará Nexus.", "Nexus todavía no se conectará al servidor en este paso.", "Host", "Usuario", "Puerto SSH", "Predeterminado: 22", "< VOLVER >", "[ CONTINUAR ]", profileConnectionStepFooter} {
		if !strings.Contains(view, want) {
			t.Fatalf("Step 2 omitted %q: %q", want, view)
		}
	}
	if strings.Contains(view, "CRI400FDev") || strings.Contains(view, "BCOINFDDGC") {
		t.Fatalf("rendered reference sample data: %q", view)
	}
	if !m.connectionHost.Focused() || !strings.Contains(view, "█") {
		t.Fatal("host must use focused native Bubbles block cursor")
	}
}

func TestProfileConnectionVisibleCursorMovesBetweenInputs(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })

	for _, tt := range []struct{ width, height int }{{120, 40}, {80, 24}} {
		t.Run(fmt.Sprintf("%dx%d", tt.width, tt.height), func(t *testing.T) {
			m := newProfileConnectionTestModel(t, &profileStoreStub{}, tt.width, tt.height)
			if view := m.View(); !strings.Contains(view, "█") {
				t.Fatalf("focused empty host lacks a visible block cursor: %q", view)
			}
			m.connectionHost.SetValue("ibmi.example.test")
			if view := m.View(); !strings.Contains(view, "\x1b[7m") {
				t.Fatalf("focused non-empty host lacks its native visible cursor: %q", view)
			}
			m = connectionKey(t, m, tea.KeyTab)
			view := m.View()
			if m.connectionHost.Focused() || !m.connectionUsername.Focused() || !strings.Contains(view, "█") {
				t.Fatalf("cursor did not transfer from host to username: %q", view)
			}
		})
	}
}

func TestProfileConnectionFocusCyclesAllControls(t *testing.T) {
	m := newProfileConnectionTestModel(t, &profileStoreStub{}, 120, 40)
	for _, want := range []profileConnectionFocus{profileConnectionFocusUsername, profileConnectionFocusPort, profileConnectionFocusBack, profileConnectionFocusContinue, profileConnectionFocusHost} {
		m = connectionKey(t, m, tea.KeyTab)
		if m.connectionFocus != want {
			t.Fatalf("tab focus = %v, want %v", m.connectionFocus, want)
		}
	}
	m = connectionKey(t, m, tea.KeyShiftTab)
	if m.connectionFocus != profileConnectionFocusContinue {
		t.Fatalf("shift-tab focus = %v, want continue", m.connectionFocus)
	}
}

func TestWizardInputFocusMarkerIsSharedAndMovesWithCursor(t *testing.T) {
	stepOne := newProfileStepTestModel(&profileStoreStub{}, 120, 40)
	stepOneView := stepOne.View()
	if line := wizardLine(stepOneView, "Nombre  ["); !strings.Contains(line, "▸ Nombre") || strings.Count(line, "▸") != 1 {
		t.Fatalf("Step 1 input marker = %q", line)
	}
	if line := wizardLine(stepOneView, "Nombre del perfil"); strings.Contains(line, "▸") {
		t.Fatalf("Step 1 section label retained duplicate marker: %q", line)
	}

	m := newProfileConnectionTestModel(t, &profileStoreStub{}, 120, 40)
	for _, tt := range []struct {
		label string
		focus profileConnectionFocus
	}{
		{"Host", profileConnectionFocusHost},
		{"Usuario", profileConnectionFocusUsername},
		{"Puerto SSH", profileConnectionFocusPort},
	} {
		t.Run(tt.label, func(t *testing.T) {
			if m.connectionFocus != tt.focus {
				t.Fatalf("focus = %v, want %v", m.connectionFocus, tt.focus)
			}
			if tt.focus == profileConnectionFocusPort {
				m.connectionPort.SetValue("")
			}
			view := m.View()
			line := wizardLine(view, tt.label+" ")
			if !strings.Contains(line, "▸ "+tt.label) || strings.Count(view, "▸") != 1 || !strings.Contains(line, "█") {
				t.Fatalf("Step 2 %s focus marker/cursor = %q", tt.label, line)
			}
			if tt.focus != profileConnectionFocusPort {
				m = connectionKey(t, m, tea.KeyTab)
			}
		})
	}
}

func TestWizardActionFocusUsesSharedMarkerSemantics(t *testing.T) {
	for _, tt := range []struct {
		name  string
		model Model
		focus func(*Model)
		label string
	}{
		{
			name:  "step one cancel",
			model: newProfileStepTestModel(&profileStoreStub{}, 120, 40),
			focus: func(m *Model) { m.profileFocus = profileFocusCancel },
			label: "< CANCELAR >",
		},
		{
			name:  "step two back",
			model: newProfileConnectionTestModel(t, &profileStoreStub{}, 120, 40),
			focus: func(m *Model) { m.connectionFocus = profileConnectionFocusBack; m.focusProfileConnectionInput() },
			label: "< VOLVER >",
		},
		{
			name:  "step one continue",
			model: newProfileStepTestModel(&profileStoreStub{}, 120, 40),
			focus: func(m *Model) { m.profileFocus = profileFocusContinue },
			label: "[ CONTINUAR ]",
		},
		{
			name:  "step two continue",
			model: newProfileConnectionTestModel(t, &profileStoreStub{}, 120, 40),
			focus: func(m *Model) { m.connectionFocus = profileConnectionFocusContinue; m.focusProfileConnectionInput() },
			label: "[ CONTINUAR ]",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.model
			m.noColor = true
			tt.focus(&m)
			view := m.View()
			if !strings.Contains(view, "▸ "+tt.label) || strings.Count(view, "▸") != 1 {
				t.Fatalf("action marker = %q", view)
			}
			if strings.Contains(wizardLine(view, "Nombre  ["), "▸") || strings.Contains(wizardLine(view, "Host  ["), "▸") || strings.Contains(wizardLine(view, "Usuario  ["), "▸") || strings.Contains(wizardLine(view, "Puerto SSH  ["), "▸") {
				t.Fatalf("action focus retained input marker: %q", view)
			}
		})
	}
}

func wizardLine(view, contains string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, contains) {
			return line
		}
	}
	return ""
}

func TestProfileConnectionInputEnterAdvancesOnlyWhenValid(t *testing.T) {
	m := newProfileConnectionTestModel(t, &profileStoreStub{}, 120, 40)
	m = connectionKey(t, m, tea.KeyEnter)
	if m.connectionFocus != profileConnectionFocusHost || !strings.Contains(m.View(), "Host requerido") {
		t.Fatal("empty host enter must remain focused with its single error")
	}
	m.connectionHost.SetValue("ibmi.example.test")
	m = connectionKey(t, m, tea.KeyEnter)
	if m.connectionFocus != profileConnectionFocusUsername {
		t.Fatal("valid host enter did not advance to username")
	}
	m.connectionUsername.SetValue("USER")
	m = connectionKey(t, m, tea.KeyEnter)
	if m.connectionFocus != profileConnectionFocusPort {
		t.Fatal("valid username enter did not advance to port")
	}
	m = connectionKey(t, m, tea.KeyEnter)
	if m.connectionFocus != profileConnectionFocusBack {
		t.Fatal("default valid port enter did not advance to back")
	}
}

func TestProfileConnectionBackAndEscapePreserveDrafts(t *testing.T) {
	for _, action := range []struct {
		name  string
		apply func(*Model)
	}{
		{"escape", func(m *Model) {}},
		{"back", func(m *Model) { m.connectionFocus = profileConnectionFocusBack; m.focusProfileConnectionInput() }},
	} {
		t.Run(action.name, func(t *testing.T) {
			m := newProfileConnectionTestModel(t, &profileStoreStub{}, 120, 40)
			m.connectionHost.SetValue("ibmi.example.test")
			m.connectionUsername.SetValue("NEXUS$USER")
			m.connectionPort.SetValue("2222")
			action.apply(&m)
			key := tea.KeyEscape
			if action.name == "back" {
				key = tea.KeyEnter
			}
			m = connectionKey(t, m, key)
			if m.screen != screenProfileStep || m.profileName.Value() != "CRI400F" || m.connectionHost.Value() != "ibmi.example.test" || m.connectionUsername.Value() != "NEXUS$USER" || m.connectionPort.Value() != "2222" {
				t.Fatalf("back navigation lost draft: %#v", m)
			}
			m.profileFocus = profileFocusContinue
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd == nil {
				t.Fatal("returning Step 1 could not re-enter Step 2")
			}
			accepted, _ := updated.(Model).Update(cmd())
			m = accepted.(Model)
			if m.connectionPort.Value() != "2222" {
				t.Fatalf("port reset after round trip: %q", m.connectionPort.Value())
			}
		})
	}
}

func TestProfileConnectionReentryUpdatesAcceptedNameWithoutResettingInputs(t *testing.T) {
	m := newProfileConnectionTestModel(t, &profileStoreStub{}, 120, 40)
	m.connectionHost.SetValue("ibmi.example.test")
	m.connectionUsername.SetValue("USER")
	m.connectionPort.SetValue("2222")
	m.screen = screenProfileStep
	m.profileName.SetValue("CRI400FProd")
	m.profileFocus = profileFocusContinue
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("updated valid Step 1 name did not produce its local seam")
	}
	accepted, _ := updated.(Model).Update(cmd())
	m = accepted.(Model)
	if m.profileDraftName != "CRI400FProd" || m.connectionHost.Value() != "ibmi.example.test" || m.connectionUsername.Value() != "USER" || m.connectionPort.Value() != "2222" {
		t.Fatalf("reentry state = name:%q host:%q user:%q port:%q", m.profileDraftName, m.connectionHost.Value(), m.connectionUsername.Value(), m.connectionPort.Value())
	}
}

func TestNewProfileWizardResetsConnectionStateAfterReturningHome(t *testing.T) {
	for _, returnToStepOne := range []struct {
		name string
		key  tea.KeyType
	}{
		{"escape", tea.KeyEscape},
		{"back", tea.KeyEnter},
	} {
		t.Run(returnToStepOne.name, func(t *testing.T) {
			m := newProfileConnectionTestModel(t, &profileStoreStub{}, 120, 40)
			m.connectionHost.SetValue("ibmi.example.test")
			m.connectionUsername.SetValue("USER")
			m.connectionPort.SetValue("2222")
			m.connectionDraft = profileConnectionDraft{host: "ibmi.example.test", username: "USER", port: 2222}
			if returnToStepOne.name == "back" {
				m.connectionFocus = profileConnectionFocusBack
				m.focusProfileConnectionInput()
			}
			m = connectionKey(t, m, returnToStepOne.key)
			m.profileFocus = profileFocusCancel
			m = connectionKey(t, m, tea.KeyEnter)
			if m.screen != screenHome {
				t.Fatal("Step 1 cancel did not return Home")
			}
			m = connectionKey(t, m, tea.KeyEnter)
			m.profileName.SetValue("CRI400FNEW")
			m.profileFocus = profileFocusContinue
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd == nil {
				t.Fatal("new valid Step 1 name did not enter the Step 2 seam")
			}
			accepted, _ := updated.(Model).Update(cmd())
			m = accepted.(Model)
			if m.connectionHost.Value() != "" || m.connectionUsername.Value() != "" || m.connectionPort.Value() != "22" || m.connectionDraft != (profileConnectionDraft{}) || m.connectionValidate || !m.connectionHost.Focused() {
				t.Fatalf("new wizard retained Step 2 state: host=%q user=%q port=%q draft=%#v validate=%v focus=%v", m.connectionHost.Value(), m.connectionUsername.Value(), m.connectionPort.Value(), m.connectionDraft, m.connectionValidate, m.connectionFocus)
			}
		})
	}
}

func TestProfileConnectionContinueIsLocalAndValidatesFields(t *testing.T) {
	store := &profileStoreStub{}
	m := newProfileConnectionTestModel(t, store, 120, 40)
	m.connectionHost.SetValue("ibmi.example.test")
	m.connectionUsername.SetValue("NEXUS$USER")
	m.connectionPort.SetValue("2222")
	m.connectionFocus = profileConnectionFocusContinue
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || len(store.profiles) != 0 || store.saveCalls != 0 {
		t.Fatal("valid Step 2 continue must emit only a local message")
	}
	msg := cmd()
	if _, ok := msg.(profileConnectionAcceptedMsg); !ok {
		t.Fatalf("continue emitted %T, want profileConnectionAcceptedMsg", msg)
	}
	accepted, _ := updated.(Model).Update(msg)
	m = accepted.(Model)
	if m.screen != screenProfileIdentity || m.connectionDraft != (profileConnectionDraft{host: "ibmi.example.test", username: "NEXUS$USER", port: 2222}) || len(store.profiles) != 0 || store.saveCalls != 0 {
		t.Fatalf("accepted local draft = %#v, persisted=%d saves=%d", m.connectionDraft, len(store.profiles), store.saveCalls)
	}
}

func TestProfileConnectionInvalidValuesBlockContinueWithOneError(t *testing.T) {
	for _, tt := range []struct {
		name, host, user, port, want string
	}{
		{"empty host", "", "USER", "22", "Host requerido"},
		{"invalid host", "bad host", "USER", "22", "Host inválido"},
		{"IPv6 host", "::1", "USER", "22", "Host inválido"},
		{"empty user", "ibmi.example.test", "", "22", "Usuario requerido"},
		{"invalid user", "ibmi.example.test", "bad user", "22", "Usuario inválido"},
		{"non-numeric port", "ibmi.example.test", "USER", "abc", "Puerto SSH debe ser un número"},
		{"zero port", "ibmi.example.test", "USER", "0", "Puerto SSH debe estar entre 1 y 65535"},
		{"large port", "ibmi.example.test", "USER", "65536", "Puerto SSH debe estar entre 1 y 65535"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newProfileConnectionTestModel(t, &profileStoreStub{}, 120, 40)
			m.connectionHost.SetValue(tt.host)
			m.connectionUsername.SetValue(tt.user)
			m.connectionPort.SetValue(tt.port)
			m.connectionFocus = profileConnectionFocusContinue
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = updated.(Model)
			if cmd != nil || m.connectionDraft != (profileConnectionDraft{}) {
				t.Fatal("invalid data reached accepted seam")
			}
			view := m.View()
			if !strings.Contains(view, tt.want) || strings.Count(view, "[ERR]") != 1 || m.connectionFocus != firstInvalidConnectionFocus(tt.host, tt.user, tt.port) {
				t.Fatalf("invalid state = %q", view)
			}
			if !connectionInputFocused(m) {
				t.Fatalf("first invalid field did not retain real input focus: %#v", m)
			}
		})
	}
}

func TestProfileConnectionContinuePrioritizesFirstInvalidField(t *testing.T) {
	for _, tt := range []struct {
		name                 string
		host, username, port string
		wantFocus            profileConnectionFocus
		wantFeedback         string
		inputFocused         func(Model) bool
	}{
		{
			name:         "invalid host wins over invalid username and port",
			host:         "bad host",
			username:     "bad user",
			port:         "not-a-port",
			wantFocus:    profileConnectionFocusHost,
			wantFeedback: "[ERR] Host inválido",
			inputFocused: func(m Model) bool { return m.connectionHost.Focused() },
		},
		{
			name:         "invalid username wins over invalid port",
			host:         "ibmi.example.test",
			username:     "bad user",
			port:         "not-a-port",
			wantFocus:    profileConnectionFocusUsername,
			wantFeedback: "[ERR] Usuario inválido",
			inputFocused: func(m Model) bool { return m.connectionUsername.Focused() },
		},
		{
			name:         "invalid port is selected after valid host and username",
			host:         "ibmi.example.test",
			username:     "USER",
			port:         "not-a-port",
			wantFocus:    profileConnectionFocusPort,
			wantFeedback: "[ERR] Puerto SSH debe ser un número",
			inputFocused: func(m Model) bool { return m.connectionPort.Focused() },
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newProfileConnectionTestModel(t, &profileStoreStub{}, 120, 40)
			m.connectionHost.SetValue(tt.host)
			m.connectionUsername.SetValue(tt.username)
			m.connectionPort.SetValue(tt.port)
			m.connectionFocus = profileConnectionFocusContinue

			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = updated.(Model)
			if cmd != nil || m.screen != screenProfileConnection || m.connectionFocus != tt.wantFocus || !tt.inputFocused(m) {
				t.Fatalf("blocked Continue state = screen:%v focus:%v real-focus:%v command:%v", m.screen, m.connectionFocus, tt.inputFocused(m), cmd != nil)
			}
			if m.status != tt.wantFeedback || strings.Count(m.View(), "[ERR]") != 1 || !strings.Contains(m.View(), tt.wantFeedback) {
				t.Fatalf("blocked Continue feedback = status:%q view:%q", m.status, m.View())
			}
		})
	}
}

func TestProfileConnectionContinueCorrectsBlockedFeedbackAndReachesLocalSeam(t *testing.T) {
	m := newProfileConnectionTestModel(t, &profileStoreStub{}, 120, 40)
	m.connectionHost.SetValue("bad host")
	m.connectionUsername.SetValue("USER")
	m.connectionFocus = profileConnectionFocusContinue
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || m.status != "[ERR] Host inválido" || m.connectionFocus != profileConnectionFocusHost || !m.connectionHost.Focused() {
		t.Fatalf("blocked host feedback/focus = status:%q focus:%v real-focus:%v", m.status, m.connectionFocus, m.connectionHost.Focused())
	}
	m.connectionHost.SetValue("")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ibmi.example.test")})
	m = updated.(Model)
	if m.status != "" || m.connectionValidate || m.profileConnectionState() != "" {
		t.Fatalf("editing correction retained blocked feedback: status=%q validate=%v state=%q", m.status, m.connectionValidate, m.profileConnectionState())
	}
	m.connectionFocus = profileConnectionFocusContinue
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("valid corrected connection did not emit the existing local seam")
	}
	if _, ok := cmd().(profileConnectionAcceptedMsg); !ok || updated.(Model).screen != screenProfileConnection {
		t.Fatalf("valid corrected connection emitted the wrong seam: %T", cmd())
	}
}

func firstInvalidConnectionFocus(host, user, port string) profileConnectionFocus {
	m := Model{}
	m.connectionHost.SetValue(host)
	m.connectionUsername.SetValue(user)
	m.connectionPort.SetValue(port)
	_, focus := m.connectionProgressGuard()
	return focus
}

func connectionInputFocused(m Model) bool {
	switch m.connectionFocus {
	case profileConnectionFocusHost:
		return m.connectionHost.Focused()
	case profileConnectionFocusUsername:
		return m.connectionUsername.Focused()
	case profileConnectionFocusPort:
		return m.connectionPort.Focused()
	default:
		return false
	}
}

func TestProfileConnectionRenderHarnessAndBounds(t *testing.T) {
	for _, tt := range []struct{ width, height int }{{120, 40}, {80, 24}, {40, 16}} {
		t.Run(fmt.Sprintf("%dx%d", tt.width, tt.height), func(t *testing.T) {
			m := newProfileConnectionTestModel(t, &profileStoreStub{}, tt.width, tt.height)
			view := m.View()
			t.Logf("Step 2 render %dx%d:\n%s", tt.width, tt.height, view)
			if lipgloss.Width(view) > tt.width || lipgloss.Height(view) > tt.height {
				t.Fatalf("view exceeds %dx%d: %dx%d", tt.width, tt.height, lipgloss.Width(view), lipgloss.Height(view))
			}
			for _, line := range strings.Split(view, "\n") {
				if lipgloss.Width(line) > tt.width {
					t.Fatalf("line exceeds %d: %q", tt.width, line)
				}
			}
			if tt.width >= 80 && !strings.Contains(view, profileConnectionStepFooter) {
				t.Fatal("full shared footer missing")
			}
			if tt.width == 120 && (!strings.Contains(view, connectionPanelTitle) || !strings.Contains(view, connectionStepIndicator)) {
				t.Fatal("desktop title row omitted title or indicator")
			}
		})
	}
}

func TestProfileConnectionContentTextBlockUsesCompactRelatedSpacing(t *testing.T) {
	m := newProfileConnectionTestModel(t, &profileStoreStub{}, 120, 40)
	lines := strings.Split(m.View(), "\n")
	heading := wizardLineIndex(lines, "Conexión con IBM i")
	description := wizardLineIndex(lines, "Indica cómo localizar el IBM i y qué usuario utilizará Nexus.")
	information := wizardLineIndex(lines, "Nexus todavía no se conectará al servidor en este paso.")
	host := wizardLineIndex(lines, "▸ Host")
	if heading < 0 || description != heading+2 || information != description+1 || strings.TrimSpace(strings.ReplaceAll(lines[heading+1], "│", "")) != "" {
		t.Fatalf("related text block is not compact: heading=%d description=%d information=%d", heading, description, information)
	}
	if host != information+2 || strings.TrimSpace(strings.ReplaceAll(lines[information+1], "│", "")) != "" {
		t.Fatalf("information-to-host gap = %d lines: %q", host-information, m.View())
	}
}

func wizardLineIndex(lines []string, contains string) int {
	for index, line := range lines {
		if strings.Contains(line, contains) {
			return index
		}
	}
	return -1
}

func TestProfileConnectionTitleRowAvoidsOverlap(t *testing.T) {
	theme := newHomeTheme(true)
	full := renderProfileConnectionTitleRow(66, theme)
	if len(full) != 1 || !strings.Contains(full[0], connectionPanelTitle) || !strings.Contains(full[0], connectionStepIndicator) {
		t.Fatalf("full title row = %#v", full)
	}
	fallback := renderProfileConnectionTitleRow(30, theme)
	if len(fallback) != 2 {
		t.Fatalf("narrow title row did not stack: %#v", fallback)
	}
	for _, line := range fallback {
		if lipgloss.Width(line) > 30 {
			t.Fatalf("fallback overflowed: %q", line)
		}
	}
}

func TestProfileConnectionHelpKeepsNativeCursorAndDraft(t *testing.T) {
	m := newProfileConnectionTestModel(t, &profileStoreStub{}, 120, 40)
	m.connectionHost.SetValue("ibmi.example.test")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(Model)
	if !strings.Contains(m.View(), "Ayuda:") || !m.connectionHost.Focused() || !strings.Contains(m.connectionHost.Cursor.View(), "█") || m.connectionHost.Value() != "ibmi.example.test" {
		t.Fatalf("help changed native input behavior: %#v", m)
	}
}
