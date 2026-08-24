package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"bac-nexus/internal/profile"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func newProfileStepTestModel(store *profileStoreStub, width, height int) Model {
	m := NewModel(store)
	m.noColor = true
	updated, _ := m.Update(profilesMsg{profiles: store.profiles})
	updated, _ = updated.(Model).Update(tea.WindowSizeMsg{Width: width, Height: height})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	return updated.(Model)
}

func typeProfileName(t *testing.T, m Model, name string) Model {
	t.Helper()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)})
	return updated.(Model)
}

func TestHomeCreateOpensProfileStepWithFocusedRealInput(t *testing.T) {
	m := newProfileStepTestModel(&profileStoreStub{}, 120, 40)
	if m.screen != screenProfileStep || !m.profileName.Focused() || m.profileName.CursorMode() != textinput.CursorStatic {
		t.Fatalf("profile step input state = screen:%v focused:%v cursor:%v", m.screen, m.profileName.Focused(), m.profileName.CursorMode())
	}
	view := m.View()
	for _, want := range []string{"BAC NEXUS", "PERFIL: NUEVO", "ESTADO: CONFIGURANDO", "Crear perfil IBM i", "Paso 1 de 9 — Perfil", "Nombre del perfil", "Usa 1–64 caracteres ASCII; inicia con letra o número.", "Luego usa solo letras, números, guion (-) o guion bajo (_).", "Sin espacios, puntos, tildes ni otros símbolos.", "Ej: CRI400F, CRI400FDev, CRI400FProd", "▸", "< CANCELAR >", "[ CONTINUAR ]"} {
		if !strings.Contains(view, want) {
			t.Fatalf("profile step omitted %q: %q", want, view)
		}
	}
	if strings.Contains(view, "⢀⣀") || strings.Contains(view, detailedLogo) {
		t.Fatalf("wizard must not render a lion: %q", view)
	}
	if cursor := m.profileName.Cursor.View(); !strings.Contains(cursor, "█") {
		t.Fatalf("focused input did not render its configured block cursor: %q", cursor)
	}
}

func TestProfileStepInputUsesBracketsValueCursorAndTrailingSurface(t *testing.T) {
	m := newProfileStepTestModel(&profileStoreStub{}, 120, 40)
	m = typeProfileName(t, m, "CRI400FDev")
	input := m.renderProfileInput(68, newHomeTheme(true))
	if !strings.Contains(input, "▸ Nombre  [ CRI400FDev") || !strings.Contains(input, " ]") {
		t.Fatalf("focused input lacks label, brackets, or real value: %q", input)
	}
	if cursor := m.profileName.Cursor.View(); !strings.Contains(cursor, "█") {
		t.Fatalf("focused Bubbles cursor is not visibly rendered: %q", cursor)
	}
	open, close := strings.Index(input, "["), strings.LastIndex(input, "]")
	if open < 0 || close-open < len("CRI400FDev█")+5 {
		t.Fatalf("focused input lacks editable trailing surface: %q", input)
	}
	m.profileFocus = profileFocusCancel
	unfocused := m.renderProfileInput(68, newHomeTheme(true))
	if strings.HasPrefix(unfocused, "▸") || !strings.Contains(unfocused, "Nombre  [ CRI400FDev") {
		t.Fatalf("unfocused input lost neutral bracketed structure: %q", unfocused)
	}
}

func TestProfileStepFocusOrderAndInputIsolation(t *testing.T) {
	m := newProfileStepTestModel(&profileStoreStub{}, 120, 40)
	for _, want := range []profileStepFocus{profileFocusCancel, profileFocusContinue, profileFocusName} {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = updated.(Model)
		if m.profileFocus != want {
			t.Fatalf("tab focus = %v, want %v", m.profileFocus, want)
		}
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated.(Model)
	if m.profileFocus != profileFocusContinue || !strings.Contains(m.View(), "▸ [ CONTINUAR ]") {
		t.Fatalf("shift-tab focus is not visibly continue: %v %q", m.profileFocus, m.View())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ignored")})
	m = updated.(Model)
	if m.profileName.Value() != "" {
		t.Fatalf("non-input focus edited the name: %q", m.profileName.Value())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	m = typeProfileName(t, m, "dev")
	if m.profileName.Value() != "dev" {
		t.Fatalf("input focus did not edit the name: %q", m.profileName.Value())
	}
}

func TestProfileStepValidationIsExclusiveAndCaseInsensitive(t *testing.T) {
	for _, tt := range []struct {
		name     string
		profiles []profile.Profile
		input    string
		want     string
	}{
		{"empty", nil, "", ""},
		{"available", nil, "prueba", "[OK] Nombre disponible"},
		{"hyphen and underscore available", nil, "CRI-400_F", "[OK] Nombre disponible"},
		{"invalid internal whitespace", nil, "CRI400F A", "[ERR] Nombre de perfil inválido"},
		{"invalid trailing whitespace", nil, "prueba ", "[ERR] Nombre de perfil inválido"},
		{"invalid leading whitespace", nil, " prueba", "[ERR] Nombre de perfil inválido"},
		{"invalid whitespace only", nil, "   ", "[ERR] Nombre de perfil inválido"},
		{"invalid tab", nil, "prueba\t", "[ERR] Nombre de perfil inválido"},
		{"invalid newline", nil, "prueba\n", "[ERR] Nombre de perfil inválido"},
		{"invalid dot", nil, "CRI400F.Dev", "[ERR] Nombre de perfil inválido"},
		{"invalid unicode", nil, "CRIñ400F", "[ERR] Nombre de perfil inválido"},
		{"exact duplicate", []profile.Profile{testProfile("CRI400F")}, "CRI400F", "[ERR] Ya existe un perfil con ese nombre"},
		{"different full name", []profile.Profile{testProfile("CRI400F")}, "CRI400FDev", "[OK] Nombre disponible"},
		{"case insensitive duplicate", []profile.Profile{testProfile("CRI400F")}, "cri400f", "[ERR] Ya existe un perfil con ese nombre"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newProfileStepTestModel(&profileStoreStub{profiles: tt.profiles}, 120, 40)
			m.profileName.SetValue(tt.input)
			view := m.View()
			state := m.profileNameState()
			if state != tt.want || strings.Count(view, "[OK]")+strings.Count(view, "[ERR]") != boolCount(tt.want != "") {
				t.Fatalf("state=%q markers=%q", state, view)
			}
			if tt.input == "CRI400F A" && strings.Contains(state, "Ya existe") {
				t.Fatalf("internal whitespace was mislabeled duplicate: %q", state)
			}
		})
	}
}

func TestProfileStepWhitespaceCannotReachAcceptedDraft(t *testing.T) {
	for _, raw := range []string{"prueba ", " prueba", "   ", "prueba\t", "prueba\n", "prueba interna"} {
		t.Run(fmt.Sprintf("%q", raw), func(t *testing.T) {
			m := newProfileStepTestModel(&profileStoreStub{}, 120, 40)
			m.profileName.SetValue(raw)
			if state := m.profileNameState(); state != "[ERR] Nombre de perfil inválido" {
				t.Fatalf("raw name %q state = %q", raw, state)
			}
			m.profileFocus = profileFocusContinue
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd != nil || updated.(Model).profileDraftName != "" {
				t.Fatalf("invalid raw name %q reached accepted draft", raw)
			}
		})
	}

	m := newProfileStepTestModel(&profileStoreStub{}, 120, 40)
	m.profileName.SetValue("prueba")
	m.profileFocus = profileFocusContinue
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("valid raw name did not reach accepted-draft seam")
	}
	accepted, _ := updated.(Model).Update(cmd())
	if accepted.(Model).profileDraftName != "prueba" {
		t.Fatalf("accepted draft = %q, want exact valid name", accepted.(Model).profileDraftName)
	}
}

func TestProfileStepTitleAndStepAlignmentFallback(t *testing.T) {
	theme := newHomeTheme(true)
	full := renderProfileStepTitleRow(66, theme)
	if len(full) != 1 || !strings.Contains(full[0], "Crear perfil IBM i") || !strings.Contains(full[0], "Paso 1 de 9 — Perfil") {
		t.Fatalf("full title row = %#v", full)
	}
	if title, step := strings.Index(full[0], "Crear perfil IBM i"), strings.Index(full[0], "Paso 1 de 9 — Perfil"); title != 0 || step <= title+len("Crear perfil IBM i")+1 {
		t.Fatalf("full title/step are not left/right aligned: %q", full[0])
	}
	fallback := renderProfileStepTitleRow(30, theme)
	if len(fallback) != 2 || !strings.Contains(fallback[0], "Crear perfil IBM i") || !strings.Contains(fallback[1], "Paso 1 de 9 — Perfil") {
		t.Fatalf("constrained fallback lost title or step: %#v", fallback)
	}
	for _, line := range fallback {
		if lipgloss.Width(line) > 30 {
			t.Fatalf("fallback line overflowed: %q", line)
		}
	}
	for _, line := range renderProfileStepTitleRow(12, theme) {
		if lipgloss.Width(line) > 12 {
			t.Fatalf("narrow fallback line overflowed: %q", line)
		}
	}
}

func TestProfileStepValidationWaitsForProfilesAndFailsClosedOnLoadError(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.noColor = true
	m.beginProfileStep()
	m.profileName.SetValue("dev")
	if state := m.profileNameState(); state != "" || m.profileNameValid() {
		t.Fatalf("pre-load validation = %q, valid=%v", state, m.profileNameValid())
	}
	m.profileFocus = profileFocusContinue
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || updated.(Model).profileDraftName != "" {
		t.Fatal("continue activated before profiles loaded")
	}
	loaded, _ := m.Update(profilesMsg{})
	m = loaded.(Model)
	if state := m.profileNameState(); state != "[OK] Nombre disponible" || !m.profileNameValid() {
		t.Fatalf("post-load validation = %q, valid=%v", state, m.profileNameValid())
	}
	failed, _ := m.Update(operationMsg{text: "Unable to load profiles", err: errors.New("offline")})
	m = failed.(Model)
	if state := m.profileNameState(); state != "" || m.profileNameValid() {
		t.Fatalf("failed-load validation = %q, valid=%v", state, m.profileNameValid())
	}
}

func TestProfileStepContinueBlockedFeedbackFocusAndCorrection(t *testing.T) {
	for _, tt := range []struct {
		name     string
		profiles []profile.Profile
		value    string
		feedback string
	}{
		{"empty", nil, "", "[WARN] Ingresa un nombre de perfil antes de continuar"},
		{"invalid", nil, "bad name", "[ERR] Nombre de perfil inválido"},
		{"duplicate", []profile.Profile{testProfile("CRI400F")}, "cri400f", "[ERR] Ya existe un perfil con ese nombre"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := newProfileStepTestModel(&profileStoreStub{profiles: tt.profiles}, 120, 40)
			m.profileName.SetValue(tt.value)
			m.profileFocus = profileFocusContinue
			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = updated.(Model)
			if cmd != nil || m.screen != screenProfileStep || m.profileDraftName != "" || m.status != tt.feedback || m.profileFocus != profileFocusName || !m.profileName.Focused() {
				t.Fatalf("blocked continue = screen:%v draft:%q status:%q focus:%v real-focus:%v", m.screen, m.profileDraftName, m.status, m.profileFocus, m.profileName.Focused())
			}
			if !strings.Contains(m.View(), tt.feedback) {
				t.Fatalf("blocked feedback is not rendered: %q", m.View())
			}
		})
	}

	m := newProfileStepTestModel(&profileStoreStub{}, 120, 40)
	m.profileName.SetValue("bad name")
	m.profileFocus = profileFocusContinue
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	m.profileName.SetValue("")
	m = typeProfileName(t, m, "valid")
	if m.status != "" || m.profileNameState() != "[OK] Nombre disponible" {
		t.Fatalf("editing correction did not clear blocked feedback: status=%q state=%q", m.status, m.profileNameState())
	}
	m.profileFocus = profileFocusContinue
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || updated.(Model).screen != screenProfileStep {
		t.Fatal("valid corrected name did not expose the existing local seam")
	}
}

func TestProfileStepContinueDistinguishesProfileLoadingFromLoadFailure(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.noColor = true
	m.beginProfileStep()
	m.profileName.SetValue("valid")
	m.profileFocus = profileFocusContinue
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || m.status != "[INFO] Cargando perfiles" || !m.profileName.Focused() || !strings.Contains(m.View(), "[INFO] Cargando perfiles") {
		t.Fatalf("pending load feedback = status:%q focused:%v view:%q", m.status, m.profileName.Focused(), m.View())
	}

	loadErr := errors.New("offline")
	updated, _ = m.Update(operationMsg{text: "Unable to load profiles", err: loadErr})
	m = updated.(Model)
	m.profileFocus = profileFocusContinue
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || !errors.Is(m.err, loadErr) || !strings.Contains(m.View(), "[ERR] offline") || strings.Contains(m.View(), "Cargando perfiles") {
		t.Fatalf("load failure was not distinguishable from loading: err=%v status=%q view=%q", m.err, m.status, m.View())
	}
}

func TestProfileStepEnterCancelAndFutureSeamDoNotSave(t *testing.T) {
	store := &profileStoreStub{}
	m := newProfileStepTestModel(store, 120, 40)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.profileFocus != profileFocusName || m.screen != screenProfileStep {
		t.Fatal("empty enter must remain on focused name")
	}
	m = typeProfileName(t, m, "dev")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.profileFocus != profileFocusContinue {
		t.Fatal("valid enter must move to continue")
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || len(store.profiles) != 0 {
		t.Fatal("valid continue must only expose its transition seam")
	}
	accepted := cmd()
	updated, _ = m.Update(accepted)
	m = updated.(Model)
	if m.screen != screenProfileConnection || m.profileDraftName != "dev" || len(store.profiles) != 0 {
		t.Fatalf("accepted seam persisted or navigated: screen=%v draft=%q profiles=%d", m.screen, m.profileDraftName, len(store.profiles))
	}
	m = newProfileStepTestModel(store, 120, 40)
	m.profileName.SetValue("bad name")
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil || updated.(Model).profileDraftName != "" {
		t.Fatal("invalid continue must be a no-op")
	}
	m.profileFocus = profileFocusCancel
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.(Model).screen != screenHome || len(store.profiles) != 0 {
		t.Fatal("cancel must return Home without saving")
	}
	m = newProfileStepTestModel(store, 120, 40)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if updated.(Model).screen != screenHome || len(store.profiles) != 0 {
		t.Fatal("escape must return Home without saving")
	}
}

func TestProfileStepHelpDoesNotChangeValidation(t *testing.T) {
	m := newProfileStepTestModel(&profileStoreStub{}, 120, 40)
	m = typeProfileName(t, m, "dev")
	before := m.profileNameState()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(Model)
	if m.profileNameState() != before || strings.Count(m.View(), "[OK]") != 0 || !strings.Contains(m.View(), "Ayuda:") {
		t.Fatalf("help changed validation state: %q", m.View())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !strings.Contains(updated.(Model).View(), "[OK]") {
		t.Fatal("clearing help did not reveal validation")
	}
}

func TestProfileStepHelpIsVisibleBoundedAndClears(t *testing.T) {
	for _, tt := range []struct{ width, height int }{{120, 40}, {80, 24}, {40, 16}} {
		t.Run(fmt.Sprintf("%dx%d", tt.width, tt.height), func(t *testing.T) {
			m := newProfileStepTestModel(&profileStoreStub{}, tt.width, tt.height)
			m = typeProfileName(t, m, "dev")
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
			m = updated.(Model)
			if !strings.Contains(m.View(), "Ayuda:") {
				t.Fatalf("help is not visible: %q", m.View())
			}
			for _, line := range strings.Split(m.View(), "\n") {
				if lipgloss.Width(line) > tt.width {
					t.Fatalf("help view exceeded width %d: %q", tt.width, line)
				}
			}
			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
			m = updated.(Model)
			if m.status != "" {
				t.Fatalf("focus move retained stale help: %q", m.status)
			}
			m.profileFocus = profileFocusName
			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
			if updated.(Model).status != "" {
				t.Fatalf("input editing retained stale help: %q", updated.(Model).status)
			}
		})
	}
}

func TestStatusOKUsesGreenSemanticToken(t *testing.T) {
	theme := newHomeTheme(false)
	if got := fmt.Sprint(theme.statusOK.GetForeground()); got != successGreen || got == bacRed || got == tertiaryFixedDim {
		t.Fatalf("status OK color = %q, want green %q", got, successGreen)
	}
	if got := NewModel(&profileStoreStub{}).renderStatusRow("[OK] ready", 20, "", newHomeTheme(true)); !strings.Contains(got, "[OK]") {
		t.Fatalf("no-color status lost marker: %q", got)
	}
}

func TestProfileStepFooterAndResponsiveBounds(t *testing.T) {
	for _, tt := range []struct{ width, height int }{{120, 40}, {80, 24}, {40, 16}} {
		t.Run(fmt.Sprintf("%dx%d", tt.width, tt.height), func(t *testing.T) {
			m := newProfileStepTestModel(&profileStoreStub{}, tt.width, tt.height)
			view := m.View()
			if lipgloss.Width(view) > tt.width || lipgloss.Height(view) > tt.height {
				t.Fatalf("view exceeds %dx%d: %dx%d", tt.width, tt.height, lipgloss.Width(view), lipgloss.Height(view))
			}
			for _, line := range strings.Split(view, "\n") {
				if lipgloss.Width(line) > tt.width {
					t.Fatalf("line exceeds %d: %q", tt.width, line)
				}
			}
			if !strings.Contains(view, "Tab siguiente") && tt.width >= 80 {
				t.Fatalf("full footer missing: %q", view)
			}
			if strings.Contains(view, "⢀⣀") {
				t.Fatal("wizard rendered a hero")
			}
		})
	}
}

func TestProfileStepVisualRhythmAndSharedPanelGeometry(t *testing.T) {
	m := newProfileStepTestModel(&profileStoreStub{}, 120, 40)
	view := m.View()
	panel := m.renderProfileStepPanel(114, 36, newHomeTheme(true))
	layout := newWizardPanelLayout(114, 36, newHomeTheme(true))
	panelWidth := 0
	for _, line := range strings.Split(panel, "\n") {
		left, right := strings.Index(line, "┌"), strings.LastIndex(line, "┐")
		if left >= 0 && right > left {
			panelWidth = lipgloss.Width(line[left : right+len("┐")])
			break
		}
	}
	visibleWidth := layout.panelWidth + 4 // Border and horizontal panel padding are visible cells.
	if panelWidth != visibleWidth {
		t.Fatalf("panel width = %d, want visible shared geometry %d", panelWidth, visibleWidth)
	}
	fullLines := strings.Split(panel, "\n")
	titleAndStepSameLine := false
	for _, line := range fullLines {
		if strings.Contains(line, "Crear perfil IBM i") && strings.Contains(line, "Paso 1 de 9 — Perfil") {
			titleAndStepSameLine = true
			break
		}
	}
	if !titleAndStepSameLine {
		t.Fatalf("full panel did not compose title and step together: %q", panel)
	}
	top := -1
	for i, line := range fullLines {
		if strings.Contains(line, "┌") {
			top = i
			break
		}
	}
	if top < 0 || top+2 >= len(fullLines) || !profilePanelInteriorBlank(fullLines[top+1]) {
		t.Fatalf("full panel lacks derived vertical padding: %q", panel)
	}
	fullTitle := strings.Index(fullLines[top+2], "Crear perfil IBM i")
	fullLeft := strings.Index(fullLines[top+2], "│")
	if fullTitle-fullLeft <= 2 {
		t.Fatalf("full panel did not increase horizontal padding: %q", fullLines[top+2])
	}
	compactPanel := m.renderProfileStepPanel(74, 18, newHomeTheme(true))
	compactLines := strings.Split(compactPanel, "\n")
	for _, line := range compactLines {
		if strings.Contains(line, "Crear perfil IBM i") {
			compactTitle, compactLeft := strings.Index(line, "Crear perfil IBM i"), strings.Index(line, "│")
			if compactTitle-compactLeft >= fullTitle-fullLeft {
				t.Fatalf("compact panel did not return to shared padding: %q", line)
			}
			break
		}
	}
	if got := lipgloss.Height(panel); got < 17 {
		t.Fatalf("full panel height = %d, want deliberate vertical rhythm", got)
	}
	if strings.Contains(view, "Ej: Desarrollo, Pruebas, Producción") || !strings.Contains(view, "Ej: CRI400F, CRI400FDev, CRI400FProd") {
		t.Fatalf("IBM i examples drifted: %q", view)
	}
	for _, tt := range []struct{ width, height int }{{120, 40}, {80, 24}, {40, 16}} {
		t.Run(fmt.Sprintf("%dx%d", tt.width, tt.height), func(t *testing.T) {
			model := newProfileStepTestModel(&profileStoreStub{}, tt.width, tt.height)
			compact := model.View()
			if lipgloss.Height(compact) > tt.height || lipgloss.Width(compact) > tt.width {
				t.Fatalf("responsive input/actions failed at %dx%d: %q", tt.width, tt.height, compact)
			}
			if tt.width < 120 && tt.height >= 24 && !strings.Contains(compact, "▼ más") {
				t.Fatalf("constrained wizard must disclose hidden controls: %q", compact)
			}
			for _, line := range strings.Split(compact, "\n") {
				if lipgloss.Width(line) > tt.width {
					t.Fatalf("line exceeds %d: %q", tt.width, line)
				}
			}
		})
	}
}

func profilePanelInteriorBlank(line string) bool {
	left, right := strings.Index(line, "│"), strings.LastIndex(line, "│")
	return left >= 0 && right > left && strings.TrimSpace(line[left+len("│"):right]) == ""
}

func TestProfileStepUsesCenteredPanelAndSharedPinnedFooter(t *testing.T) {
	m := newProfileStepTestModel(&profileStoreStub{}, 120, 40)
	view := m.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if !strings.Contains(lines[len(lines)-2], profileStepFooter) || !strings.Contains(lines[len(lines)-3], "─") {
		t.Fatalf("footer lost its pinned separator or approved text: %q", view)
	}
	panelLine := ""
	for _, line := range lines {
		if strings.Contains(line, "Crear perfil IBM i") {
			panelLine = line
			break
		}
	}
	if panelLine == "" || strings.Index(panelLine, "Crear perfil IBM i") < 20 {
		t.Fatalf("desktop panel was not centered: %q", panelLine)
	}
	frameWidth, _ := m.shellFrameDimensions()
	inner := m.shellInnerWidth(frameWidth)
	footer := renderFooterText(inner, newHomeTheme(true), profileStepFooter, m.buildInfo)
	commandStart := strings.Index(footer, profileStepFooter)
	if commandStart < 0 || lipgloss.Width(footer[:commandStart]) != (inner-lipgloss.Width(profileStepFooter))/2 {
		t.Fatalf("footer commands are not centered: %q", footer)
	}
	if versionEnd := strings.Index(footer, "BAC NEXUS ") + lipgloss.Width("BAC NEXUS "+m.buildInfo.Version); versionEnd > commandStart {
		t.Fatalf("footer version overlaps commands: %q", footer)
	}
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}

func TestWizardInitialFocusIsVisibleAt40x16(t *testing.T) {
	for _, noColor := range []bool{true, false} {
		t.Run("render", func(t *testing.T) {
			m := newProfileStepTestModel(&profileStoreStub{}, 40, 16)
			m.noColor = noColor
			m.profilesLoaded = true
			m.refreshWizardViewport()
			if !strings.Contains(m.View(), "Nombre") {
				t.Fatalf("Step 1 focused input is clipped:\n%s", m.View())
			}
			m.profileName.SetValue("dev")
			u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = u.(Model)
			u, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd == nil {
				t.Fatal("Step 1 transition seam missing")
			}
			u, _ = u.(Model).Update(cmd())
			m = u.(Model)
			if !strings.Contains(m.View(), "Host") {
				t.Fatalf("Step 2 focused input is clipped:\n%s", m.View())
			}
		})
	}
}
