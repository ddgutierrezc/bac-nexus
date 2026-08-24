package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestWizardRenderThemeHierarchy(t *testing.T) {
	theme := newHomeTheme(false)
	for _, tt := range []struct {
		name, got, want string
	}{
		{"active marker", fmt.Sprint(theme.selectedMarker.GetForeground()), bacRed},
		{"active label", fmt.Sprint(theme.selectedLabel.GetForeground()), textPrimary},
		{"inactive label", fmt.Sprint(theme.fieldsetTitle.GetForeground()), salmonMuted},
		{"inactive value", fmt.Sprint(theme.metadata.GetForeground()), mutedMetadata},
		{"content heading", fmt.Sprint(theme.wizardContentHeading.GetForeground()), textPrimary},
		{"step indicator", fmt.Sprint(theme.fieldsetTitle.GetForeground()), salmonMuted},
		{"divider", fmt.Sprint(theme.fieldsetBorder.GetForeground()), borderSurface},
		{"selected surface", fmt.Sprint(theme.selectedRow.GetBackground()), insetSurface},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}

	input := textinput.New()
	input.Focus()
	active := renderWizardInputRow("Host", input, true, 40, theme, wizardInputOptions{})
	inactive := renderWizardInputRow("Host", input, false, 40, theme, wizardInputOptions{})
	if !strings.Contains(active, theme.selectedMarker.Render("▸ ")) || !strings.Contains(active, theme.selectedLabel.Render("Host")) {
		t.Fatalf("active hierarchy omitted shared marker or label role: %q", active)
	}
	if strings.Contains(inactive, "▸") || !strings.Contains(inactive, theme.fieldsetTitle.Render("Host")) {
		t.Fatalf("inactive hierarchy did not use shared secondary label role: %q", inactive)
	}
}

func TestWizardTitleAndDividerHelpersAreShared(t *testing.T) {
	theme := newHomeTheme(true)
	if got, want := renderProfileStepTitleRow(66, theme), renderWizardTitleRow(66, theme, "Crear perfil IBM i", "Paso 1 de 9 — Perfil"); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("Step 1 title helper drifted: %q != %q", got, want)
	}
	if got, want := renderProfileConnectionTitleRow(66, theme), renderWizardTitleRow(66, theme, connectionPanelTitle, connectionStepIndicator); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("Step 2 title helper drifted: %q != %q", got, want)
	}
	for _, width := range []int{66, 30, 12} {
		for _, line := range renderWizardTitleRow(width, theme, connectionPanelTitle, connectionStepIndicator) {
			if lipgloss.Width(line) > width {
				t.Fatalf("title fallback overflowed %d: %q", width, line)
			}
		}
		if divider := renderWizardDivider(width, theme); lipgloss.Width(divider) != width {
			t.Fatalf("divider width = %d, want %d", lipgloss.Width(divider), width)
		}
	}
}

func TestWizardActionStatesPreserveProgressSemantics(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	for _, noColor := range []bool{false, true} {
		for _, tt := range []struct {
			name            string
			options         []wizardActionOptions
			wantMarker      bool
			wantUnavailable bool
			wantFocused     bool
		}{
			{name: "ready default", wantMarker: true, wantFocused: true},
			{name: "ready explicit", options: []wizardActionOptions{{rightState: wizardProgressReady}}, wantMarker: true, wantFocused: true},
			{name: "blocked", options: []wizardActionOptions{{rightState: wizardProgressBlocked}}, wantMarker: true},
			{name: "disabled", options: []wizardActionOptions{{rightState: wizardProgressDisabled}}, wantUnavailable: true},
		} {
			t.Run(fmt.Sprintf("%s/no-color=%t", tt.name, noColor), func(t *testing.T) {
				theme := newHomeTheme(noColor)
				output := renderWizardActionsBlock(80, theme, "< BACK >", "[ NEXT ]", false, true, noColor, tt.options...).text
				plain := ansiEscape.ReplaceAllString(output, "")
				if got := strings.Contains(plain, "▸ [ NEXT ]"); got != tt.wantMarker {
					t.Fatalf("focused marker = %t, want %t: %q", got, tt.wantMarker, output)
				}
				if got := strings.Contains(plain, "[--]"); got != tt.wantUnavailable {
					t.Fatalf("unavailable cue = %t, want %t: %q", got, tt.wantUnavailable, output)
				}
				if noColor {
					if ansiEscape.MatchString(output) {
						t.Fatalf("NO_COLOR action contains ANSI: %q", output)
					}
					return
				}
				focusedStyle := strings.TrimSuffix(lipgloss.NewStyle().Background(lipgloss.Color(bacRed)).Foreground(lipgloss.Color(white)).Bold(true).Render("x"), "x\x1b[0m")
				if got := strings.Contains(output, focusedStyle); got != tt.wantFocused {
					t.Fatalf("focused style = %t, want %t: %q", got, tt.wantFocused, output)
				}
				if tt.wantUnavailable {
					neutralStyle := strings.TrimSuffix(theme.statusNeutral.Render("x"), "x\x1b[0m")
					if !strings.Contains(output, neutralStyle) {
						t.Fatalf("disabled action lost neutral style: %q", output)
					}
				}
			})
		}
	}
}

func TestWizardSemanticFeedbackRendering(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
	for _, tt := range []struct {
		kind   wizardFeedbackKind
		marker string
		style  func(homeTheme) lipgloss.Style
	}{
		{wizardFeedbackOK, "[OK]", func(t homeTheme) lipgloss.Style { return t.statusOK }},
		{wizardFeedbackInfo, "[INFO]", func(t homeTheme) lipgloss.Style { return t.statusInfo }},
		{wizardFeedbackWarning, "[WARN]", func(t homeTheme) lipgloss.Style { return t.statusWarning }},
		{wizardFeedbackError, "[ERR]", func(t homeTheme) lipgloss.Style { return t.statusError }},
		{wizardFeedbackNeutral, "[--]", func(t homeTheme) lipgloss.Style { return t.statusNeutral }},
	} {
		t.Run(tt.marker, func(t *testing.T) {
			message := "mensaje completo con palabras suficientes para comprobar continuidad"
			colored := renderWizardFeedback(20, newHomeTheme(false), wizardFeedback{kind: tt.kind, message: message})
			plain := ansiEscape.ReplaceAllString(colored, "")
			styleCode := strings.TrimSuffix(tt.style(newHomeTheme(false)).Render("x"), "x\x1b[0m")
			if !strings.Contains(plain, tt.marker) || !strings.Contains(colored, styleCode) {
				t.Fatalf("semantic marker or style role missing: %q", colored)
			}
			lines := strings.Split(plain, "\n")
			for i, line := range lines {
				if lipgloss.Width(line) > 20 {
					t.Fatalf("line %d overflowed: %q", i, line)
				}
				if i > 0 && !strings.HasPrefix(line, strings.Repeat(" ", lipgloss.Width(tt.marker+" "))) {
					t.Fatalf("continuation indentation drifted: %q", line)
				}
			}
			if got := reconstructWrappedParagraphs(lines, message, tt.marker+" "); got != message {
				t.Fatalf("message was truncated or reconstructed incorrectly: %q", got)
			}
			noColor := renderWizardFeedback(20, newHomeTheme(true), wizardFeedback{kind: tt.kind, message: message})
			if ansiEscape.MatchString(noColor) || !strings.Contains(noColor, tt.marker) {
				t.Fatalf("NO_COLOR feedback drifted: %q", noColor)
			}
		})
	}
}

func TestWizardFeedbackPrecedence(t *testing.T) {
	m := Model{status: "[WARN] aviso", err: fmt.Errorf("fallo")}
	if got, _ := m.wizardFeedbackFor("[OK] válido"); got.kind != wizardFeedbackError || got.message != "fallo" {
		t.Fatalf("error did not win: %#v", got)
	}
	m.err = nil
	if got, _ := m.wizardFeedbackFor("[OK] válido"); got.kind != wizardFeedbackWarning || got.message != "aviso" {
		t.Fatalf("explicit status did not win: %#v", got)
	}
	m.status = ""
	if got, _ := m.wizardFeedbackFor("[OK] válido"); got.kind != wizardFeedbackOK || got.message != "válido" {
		t.Fatalf("validation was not revealed: %#v", got)
	}
}

func TestWizardRuntimeRendersOneFeedbackBlockWithExplicitPrecedence(t *testing.T) {
	countMarkers := func(view string) int {
		plain := ansiEscape.ReplaceAllString(view, "")
		count := 0
		for _, marker := range []string{"[OK]", "[INFO]", "[WARN]", "[ERR]", "[--]"} {
			count += strings.Count(plain, marker)
		}
		return count
	}

	step1 := newProfileStepTestModel(&profileStoreStub{}, 80, 24)
	step1.profileName.SetValue("dev")
	step1.status = "[INFO] Ayuda de perfil"
	step1.refreshWizardViewport()
	if got := countMarkers(step1.View()); got != 1 {
		t.Fatalf("Step 1 rendered %d feedback markers, want one", got)
	}

	step2 := newProfileConnectionTestModel(t, &profileStoreStub{}, 80, 24)
	step2.connectionHost.SetValue("ibmi.example.test")
	step2.connectionUsername.SetValue("USER")
	step2.connectionValidate = true
	step2.status = "[WARN] Revisa la conexión"
	step2.refreshWizardViewport()
	if got := countMarkers(step2.View()); got != 1 {
		t.Fatalf("Step 2 rendered %d feedback markers, want one", got)
	}
}

func TestWizardPanelsRenderResponsiveGeometry(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
	for _, tt := range []struct {
		name                                  string
		width, height, minPercent, maxPercent int
	}{
		{"large", 120, 40, 70, 75},
		{"medium", 80, 24, 85, 90},
		{"narrow", 40, 16, 89, 90},
	} {
		for _, noColor := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/no-color=%t", tt.name, noColor), func(t *testing.T) {
				panels := make([]runtimePanel, 0, 3)
				for _, wizard := range runtimeWizardCases(t, tt.width, tt.height, noColor) {
					_, panel := runtimeViewAtPanelTop(t, wizard.model)
					panels = append(panels, panel)
				}
				for i, panel := range panels {
					if panel.width != panels[0].width {
						t.Fatalf("step %d visible panel width = %d, want shared %d", i+1, panel.width, panels[0].width)
					}
					usefulWidth := tt.width - 2 // The outer shell border owns one cell per side.
					percent := panel.width * 100 / usefulWidth
					if percent < tt.minPercent || percent > tt.maxPercent {
						t.Fatalf("step %d rendered panel ratio = %d%%, want %d%%..%d%%", i+1, percent, tt.minPercent, tt.maxPercent)
					}
					leftMargin := panel.left - 1
					rightMargin := usefulWidth - panel.width - leftMargin
					if leftMargin < 0 || rightMargin < 0 || abs(leftMargin-rightMargin) > 1 {
						t.Fatalf("step %d rendered panel margins = %d/%d, want centered", i+1, leftMargin, rightMargin)
					}
				}
			})
		}
	}
}

func TestWizardDescriptionSpacingIsSharedAndCompact(t *testing.T) {
	for _, size := range []struct{ width, height int }{{120, 40}, {80, 24}} {
		for _, noColor := range []bool{false, true} {
			for _, tt := range []struct {
				name, title, descriptionStart, supporting, description, control string
				model                                                           func() Model
			}{
				{"step-1", "Nombre del perfil", "Usa 1–64", "", "Usa 1–64 caracteres ASCII; inicia con letra o número. Luego usa solo letras, números, guion (-) o guion bajo (_). Sin espacios, puntos, tildes ni otros símbolos. Ej: CRI400F, CRI400FDev, CRI400FProd", "Nombre  [", func() Model { return newProfileStepTestModel(&profileStoreStub{}, size.width, size.height) }},
				{"step-2", "Conexión con IBM i", "Indica cómo", "", "Indica cómo localizar el IBM i y qué usuario utilizará Nexus. Nexus todavía no se conectará al servidor en este paso.", "Host", func() Model { return newProfileConnectionTestModel(t, &profileStoreStub{}, size.width, size.height) }},
				{"step-3", "Identidad del servidor", "¿Cómo quieres", "Elige cómo", "¿Cómo quieres comprobar que este IBM i es el servidor correcto? Elige cómo Nexus debe establecer la confianza SSH de este perfil. Esta decisión solo se registra localmente en el asistente; no conecta con el servidor ni guarda credenciales o perfiles todavía.", "Verificar un fingerprint", func() Model { return newProfileIdentityTestModel(t, size.width, size.height) }},
			} {
				t.Run(fmt.Sprintf("%s/%dx%d/no-color=%t", tt.name, size.width, size.height, noColor), func(t *testing.T) {
					m := tt.model()
					m.noColor = noColor
					updated, _ := m.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
					m = updated.(Model)
					frames := make([]string, 0, 81)
					for range 40 {
						updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
						m = updated.(Model)
					}
					frames = append(frames, m.View())
					for range 40 {
						updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
						m = updated.(Model)
						frames = append(frames, m.View())
					}
					find := func(lines []string, needle string) int {
						for i, line := range lines {
							if strings.Contains(line, needle) {
								return i
							}
						}
						return -1
					}
					blank := func(line string) bool { return strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "│")) == "" }
					titleGap, questionSupportingGap, controlGap, fullDescription := false, tt.supporting == "", false, false
					for _, frame := range frames {
						lines := strings.Split(runtimePanelFragments([]string{frame}), "\n")
						title, description, control := find(lines, tt.title), find(lines, tt.descriptionStart), find(lines, tt.control)
						if title >= 0 && description >= 0 {
							if description != title+2 || !blank(lines[title+1]) {
								t.Fatal("title-to-description gap is not exactly one visible row")
							}
							titleGap = true
						}
						blockStart := description
						if tt.supporting != "" {
							supporting := find(lines, tt.supporting)
							if description >= 0 && supporting >= 0 {
								if supporting < 2 || !blank(lines[supporting-1]) || blank(lines[supporting-2]) {
									t.Fatal("question-to-supporting gap is not exactly one visible row")
								}
								blockStart = supporting
								questionSupportingGap = true
							}
						}
						if blockStart >= 0 && control >= 0 && control > blockStart {
							if !blank(lines[control-1]) || control < 2 || blank(lines[control-2]) {
								t.Fatal("description-to-control gap is not exactly one visible row")
							}
							for _, line := range lines[blockStart : control-1] {
								if blank(line) {
									t.Fatal("description block gained an internal visible blank row")
								}
							}
							controlGap = true
						}
						if strings.Contains(alphaNumericOnly(strings.Join(lines, " ")), alphaNumericOnly(tt.description)) {
							fullDescription = true
						}
					}
					if !titleGap || !questionSupportingGap || !controlGap || !fullDescription {
						t.Fatalf("runtime spacing proof incomplete: title=%t question-supporting=%t control=%t description=%t", titleGap, questionSupportingGap, controlGap, fullDescription)
					}
				})
			}
		}
	}
}

func TestWizardPanelsRenderContentDrivenHeightAndActionGrouping(t *testing.T) {
	panels := make([]runtimePanel, 0, 3)
	actions := make([]runtimeAction, 0, 3)
	for _, wizard := range runtimeWizardCases(t, 120, 40, true) {
		model, panel := runtimeViewAtPanelTop(t, wizard.model)
		panel.height = runtimePanelHeight(t, model, panel)
		panels = append(panels, panel)
		actions = append(actions, runtimeActionAtPanelBottom(t, model, panel, wizard.leftAction))
	}
	if panels[0].height >= panels[2].height {
		t.Fatalf("rendered panel heights were equalized: step1=%d step3=%d", panels[0].height, panels[2].height)
	}
	for i, action := range actions {
		if !strings.Contains(action.line, action.left) || !strings.Contains(action.line, "[ CONTINUAR ]") || !strings.Contains(action.line, action.left+"    [ CONTINUAR ]") {
			t.Fatalf("step %d actions are not grouped on one runtime line: %q", i+1, action.line)
		}
		if action.rowsToBottom != actions[0].rowsToBottom || action.rightEdge != actions[0].rightEdge {
			t.Fatalf("step %d action baseline drifted: %#v versus %#v", i+1, action, actions[0])
		}
	}
}

func TestWizardNarrowActionsRemainReachableAndFocused(t *testing.T) {
	for _, wizard := range runtimeWizardCases(t, 40, 16, true) {
		t.Run(wizard.name, func(t *testing.T) {
			model := wizard.model
			for range wizard.tabsToLeftAction {
				updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
				model = updated.(Model)
			}
			if view := model.View(); !strings.Contains(view, "▸ "+wizard.leftAction) {
				t.Fatalf("left action focus is not visible through runtime View: %q", view)
			}
			views := []string{model.View()}
			updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
			model = updated.(Model)
			if view := model.View(); !strings.Contains(view, "▸ [ CONTINUAR ]") {
				t.Fatalf("continue focus is not visible through runtime View: %q", view)
			}

			views = append(views, model.View())
			for range 100 {
				updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
				model = updated.(Model)
				views = append(views, model.View())
			}
			observed := runtimePanelFragments(views)
			left, right := strings.Index(observed, wizard.leftAction), strings.Index(observed, "[ CONTINUAR ]")
			if left < 0 || right < 0 || left >= right {
				t.Fatalf("stacked runtime actions lost ordering: %q", observed)
			}
		})
	}
}

type runtimeWizardCase struct {
	name             string
	leftAction       string
	tabsToLeftAction int
	model            Model
}

type runtimePanel struct {
	left, right, top, bottom, width, height int
}

type runtimeAction struct {
	left                    string
	line                    string
	rightEdge, rowsToBottom int
}

func runtimeWizardCases(t *testing.T, width, height int, noColor bool) []runtimeWizardCase {
	t.Helper()
	step1 := newProfileStepTestModel(&profileStoreStub{}, width, height)
	step2 := newProfileConnectionTestModel(t, &profileStoreStub{}, width, height)
	step3 := newProfileIdentityTestModel(t, width, height)
	step3.identityDecision = profileIdentityKnownFingerprint
	cases := []runtimeWizardCase{
		{name: "step-1", leftAction: "< CANCELAR >", tabsToLeftAction: 1, model: step1},
		{name: "step-2", leftAction: "< VOLVER >", tabsToLeftAction: 3, model: step2},
		{name: "step-3", leftAction: "< VOLVER >", tabsToLeftAction: 2, model: step3},
	}
	for i := range cases {
		cases[i].model.noColor = noColor
		cases[i].model.refreshWizardViewport()
	}
	return cases
}

func runtimeViewAtPanelTop(t *testing.T, model Model) (Model, runtimePanel) {
	t.Helper()
	for range 40 {
		if panel, ok := runtimePanelFromView(model.View()); ok {
			return model, panel
		}
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyPgUp})
		model = updated.(Model)
	}
	t.Fatalf("central panel top was not observable through runtime View")
	return Model{}, runtimePanel{}
}

func runtimeActionAtPanelBottom(t *testing.T, model Model, panel runtimePanel, leftAction string) runtimeAction {
	t.Helper()
	for range 120 {
		view := ansiEscape.ReplaceAllString(model.View(), "")
		lines := strings.Split(view, "\n")
		for i, line := range lines {
			if !strings.Contains(line, leftAction) || !strings.Contains(line, "[ CONTINUAR ]") {
				continue
			}
			for bottom := i + 1; bottom < len(lines); bottom++ {
				if strings.Index(lines[bottom], "└") == panel.left {
					right := strings.Index(line, "[ CONTINUAR ]") + len("[ CONTINUAR ]")
					return runtimeAction{left: leftAction, line: line, rightEdge: lipgloss.Width(line[:right]), rowsToBottom: bottom - i}
				}
			}
		}
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
	}
	t.Fatalf("runtime scrolling did not reveal %q and the panel bottom together", leftAction)
	return runtimeAction{}
}

func runtimePanelHeight(t *testing.T, model Model, panel runtimePanel) int {
	t.Helper()
	for moves := 0; moves < 120; moves++ {
		lines := strings.Split(ansiEscape.ReplaceAllString(model.View(), ""), "\n")
		for bottom, line := range lines {
			if strings.Index(line, "└") == panel.left {
				return moves + bottom - panel.top + 1
			}
		}
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
	}
	t.Fatalf("runtime scrolling did not reveal the central panel bottom")
	return 0
}

func runtimePanelFromView(view string) (runtimePanel, bool) {
	lines := strings.Split(ansiEscape.ReplaceAllString(view, ""), "\n")
	for top, line := range lines {
		left, right := strings.Index(line, "┌"), strings.LastIndex(line, "┐")
		if left <= 0 || right <= left {
			continue
		}
		panel := runtimePanel{left: left, right: right, top: top, bottom: -1, width: lipgloss.Width(line[left : right+len("┐")])}
		for bottom := top + 1; bottom < len(lines); bottom++ {
			if strings.Index(lines[bottom], "└") == left {
				panel.bottom, panel.height = bottom, bottom-top+1
				break
			}
		}
		return panel, true
	}
	return runtimePanel{}, false
}

// runtimePanelFragments extracts only the central panel's visible content from
// real View frames. It intentionally has no access to internal render helpers.
func runtimePanelFragments(views []string) string {
	var fragments []string
	for _, view := range views {
		for _, line := range strings.Split(ansiEscape.ReplaceAllString(view, ""), "\n") {
			positions := make([]int, 0, 4)
			for offset := 0; ; {
				index := strings.Index(line[offset:], "│")
				if index < 0 {
					break
				}
				positions = append(positions, offset+index)
				offset += index + len("│")
			}
			if len(positions) >= 4 {
				fragments = append(fragments, line[positions[1]+len("│"):positions[2]])
			}
		}
	}
	return strings.Join(fragments, "\n")
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func TestProfileConnectionGridReservesSharedLabelColumn(t *testing.T) {
	m := newProfileConnectionTestModel(t, &profileStoreStub{}, 120, 40)
	theme := newHomeTheme(true)
	rows := []string{
		m.renderConnectionInputRow("Host", m.connectionHost, profileConnectionFocusHost, 66, theme),
		m.renderConnectionInputRow("Usuario", m.connectionUsername, profileConnectionFocusUsername, 66, theme),
		m.renderConnectionPortRow(66, theme),
	}
	firstBracket := strings.Index(rows[0], "[")
	bracket := lipgloss.Width(rows[0][:firstBracket])
	for _, row := range rows {
		index := strings.Index(row, "[")
		if column := lipgloss.Width(row[:index]); column != bracket {
			t.Fatalf("input bracket column = %d, want %d: %q", column, bracket, row)
		}
	}
	if !strings.Contains(rows[2], " ]  Predeterminado: 22") {
		t.Fatalf("port metadata gap drifted: %q", rows[2])
	}
}

func TestProfileConnectionColorRenderPreservesHierarchyAndBounds(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })

	m := newProfileConnectionTestModel(t, &profileStoreStub{}, 120, 40)
	m.noColor = false
	view := m.View()
	t.Logf("Step 2 color render 120x40:\n%s", view)
	if !strings.Contains(view, "\x1b[") || !strings.Contains(view, "Paso 2 de 9 — Conexión") || !strings.Contains(view, "▸") {
		t.Fatalf("color render lost hierarchy: %q", view)
	}
	if lipgloss.Width(view) > 120 || lipgloss.Height(view) > 40 {
		t.Fatalf("color render bounds = %dx%d", lipgloss.Width(view), lipgloss.Height(view))
	}
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > 120 {
			t.Fatalf("color line overflowed: %q", line)
		}
	}
}
