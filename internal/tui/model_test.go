package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bac-nexus/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

func TestHomeIsInitialScreen(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	if m.screen != screenHome {
		t.Fatalf("screen = %v, want home", m.screen)
	}
	view := m.View()
	if !strings.Contains(view, "BAC NEXUS") || !strings.Contains(view, "Perfiles IBM i") {
		t.Fatalf("home view = %q", view)
	}
	if !strings.Contains(view, "Preparación local: no evaluada") {
		t.Fatalf("home readiness was not truthful: %q", view)
	}
}

func TestHomeDesktopCompositionAndCompleteMenu(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.noColor = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	view := updated.(Model).View()
	for _, want := range []string{
		"Resumen de preparación",
		"Contexto IBM i seguro para desarrolladores y agentes de IA",
		"Perfiles IBM i",
		"Crear un perfil",
		"Verificar preparación",
		"Diagnósticos",
		"Integraciones MCP",
		"Configuración",
		"Salir",
		"↑/↓ navegar  •  Enter seleccionar  •  ? ayuda  •  q salir",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("desktop composition omitted %q", want)
		}
	}
	for _, line := range strings.Split(detailedLogo, "\n") {
		if !strings.Contains(view, line) {
			t.Fatalf("desktop composition omitted detailed logo line %q", line)
		}
	}
	if m.homeSelected != actionCreate {
		t.Fatalf("empty-state home selected = %q, want create", m.homeSelected)
	}
}

func TestHomeUsesSubtleSingleBorderAndFullWidthShell(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.noColor = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	view := updated.(Model).View()
	if strings.Contains(view, "╭") || strings.Contains(view, "╮") || strings.Contains(view, "╔") || strings.Contains(view, "╗") {
		t.Fatalf("outer shell retained heavy geometry: %q", view)
	}
	if !strings.Contains(view, "┌") || !strings.Contains(view, "┐") {
		t.Fatalf("outer shell must use subtle single border: %q", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > 120 {
			t.Fatalf("line width = %d, exceeds shell width 120: %q", got, line)
		}
	}
	if height := lipgloss.Height(view); height > 36 {
		t.Fatalf("shell height = %d, exceeds terminal height 36: %q", height, view)
	}
	if height := lipgloss.Height(view); height < 30 {
		t.Fatalf("shell height = %d, expected to occupy most of the 36-row viewport: %q", height, view)
	}
}

func TestHomeInitialFocusIsCreateAndHeaderTransitionsTruthfully(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	if updated.(Model).homeSelected != actionCreate {
		t.Fatalf("initial selection = %q, want create", updated.(Model).homeSelected)
	}
	if !strings.Contains(updated.(Model).View(), "PERFIL: NO EVALUADO") {
		t.Fatalf("header must show NO EVALUADO before profile load: %q", updated.(Model).View())
	}
	loaded, _ := updated.(Model).Update(profilesMsg{profiles: nil})
	if !strings.Contains(loaded.(Model).View(), "PERFIL: NINGUNO") {
		t.Fatalf("zero-profiles header must show NINGUNO: %q", loaded.(Model).View())
	}
	withProfiles, _ := loaded.(Model).Update(profilesMsg{profiles: []profile.Profile{testProfile("dev")}})
	if !strings.Contains(withProfiles.(Model).View(), "PERFIL: SIN SELECCIONAR") {
		t.Fatalf("present-profiles header must show SIN SELECCIONAR: %q", withProfiles.(Model).View())
	}
}

func TestHomeConditionalManageProfileAction(t *testing.T) {
	store := &profileStoreStub{profiles: []profile.Profile{testProfile("dev")}}
	m := NewModel(store)
	updated, _ := m.Update(profilesMsg{profiles: store.profiles})
	updated, _ = updated.(Model).Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	if !strings.Contains(updated.(Model).View(), "Administrar perfiles") {
		t.Fatalf("populated state must include manage action: %q", updated.(Model).View())
	}
	first := updated.(Model)
	for _, action := range first.visibleHomeActions() {
		if action.id == actionManage {
			first.homeSelected = actionManage
			break
		}
	}
	moved, cmd := first.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if moved.(Model).screen != screenList {
		t.Fatalf("manage action did not route to list: screen = %v, selected = %v, layout = %v", moved.(Model).screen, moved.(Model).homeSelected, moved.(Model).homeLayout())
	}
	if cmd == nil {
		t.Fatal("manage action did not request reload")
	}
}

func TestHomeTransitionsToCreateProfile(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.(Model).screen != screenProfileStep {
		t.Fatalf("initial create focus did not enter profile step: screen = %v", updated.(Model).screen)
	}
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEscape})
	if updated.(Model).screen != screenHome {
		t.Fatalf("escape did not return to home")
	}
}

func TestHomeMenuTransitionsToProfileListWhenPopulated(t *testing.T) {
	store := &profileStoreStub{profiles: []profile.Profile{testProfile("dev")}}
	m := NewModel(store)
	loaded, _ := m.Update(profilesMsg{profiles: store.profiles})
	loaded, _ = loaded.(Model).Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	model := loaded.(Model)
	model.homeSelected = actionManage
	enter, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if enter.(Model).screen != screenList || cmd == nil {
		t.Fatalf("populated home action did not transition to the list: screen = %v cmd = %v", enter.(Model).screen, cmd)
	}
}

func TestHomeUnavailableActionDoesNotMutateState(t *testing.T) {
	store := &profileStoreStub{profiles: []profile.Profile{testProfile("dev")}}
	m := NewModel(store)
	m.homeSelected = actionIntegrations
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if cmd != nil || got.screen != screenHome || len(store.profiles) != 1 {
		t.Fatal("unavailable action changed state or scheduled domain work")
	}
	if !strings.Contains(got.View(), "todavía no está disponible") {
		t.Fatalf("unavailable feedback = %q", got.View())
	}
}

func TestHomeExitIsSelectable(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.homeSelected = actionExit
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("quit did not schedule a command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("quit message = %T, want tea.QuitMsg", msg)
	}
}

func TestHomeResponsiveLayouts(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		height   int
		contains []string
		omits    string
	}{
		{"compact", 60, 40, []string{"Resumen de preparación", "Perfiles IBM i", "Crear un perfil"}, detailedLogo},
		{"very small", 20, 8, []string{"BAC NEXUS", "Crear", "Salir", "↑/↓"}, "Verificar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel(&profileStoreStub{})
			m.noColor = true
			updated, _ := m.Update(tea.WindowSizeMsg{Width: tt.width, Height: tt.height})
			view := updated.(Model).View()
			for _, want := range tt.contains {
				if !strings.Contains(view, want) {
					t.Fatalf("layout omitted %q: %q", want, view)
				}
			}
			if strings.Contains(view, tt.omits) {
				t.Fatalf("layout retained optional content %q: %q", tt.omits, view)
			}
			for _, line := range strings.Split(view, "\n") {
				if got := lipgloss.Width(line); got > tt.width {
					t.Fatalf("line width = %d, exceeds terminal width %d: %q", got, tt.width, line)
				}
			}
		})
	}
}

func TestMinimalHomeFeedbackFitsContentWidth(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.width, m.height = 20, 8
	m.noColor = true
	m.status = strings.Repeat("status ", 8)
	m.err = errors.New("connection\nfailed after an unexpectedly long response")

	frameWidth, _ := m.shellFrameDimensions()
	contentWidth := m.shellInnerWidth(frameWidth)
	view := m.renderMinimalHome()
	var statusLine, errorLine string
	for _, line := range strings.Split(strings.TrimSuffix(view, "\n"), "\n") {
		if got := lipgloss.Width(line); got > contentWidth {
			t.Fatalf("line width = %d, exceeds content width %d: %q", got, contentWidth, line)
		}
		if strings.HasPrefix(line, "[--]") {
			statusLine = line
		}
		if strings.HasPrefix(line, "[ERR]") {
			errorLine = line
		}
	}
	if !strings.HasPrefix(statusLine, "[--] ") {
		t.Fatalf("status prefix = %q, want deterministic [--] prefix", statusLine)
	}
	if !strings.HasPrefix(errorLine, "[ERR] ") {
		t.Fatalf("error prefix = %q, want deterministic [ERR] prefix", errorLine)
	}
}

func TestHomeResponsiveSelectionUsesStableActionIDs(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.homeSelected = actionExit
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 20, Height: 8})
	m = updated.(Model)
	if m.homeSelected != actionExit {
		t.Fatalf("selection = %q, want stable exit action", m.homeSelected)
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("responsive exit selection did not route to quit")
	}
}

func TestHomeNoColorAndThemeColors(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	m := NewModel(&profileStoreStub{})
	if !m.noColor {
		t.Fatal("NO_COLOR did not disable Home colors")
	}
	m.profilesLoaded = true
	view := m.View()
	if strings.Contains(view, "\x1b[") {
		t.Fatalf("no-color home contains ANSI escape: %q", view)
	}
	if !strings.Contains(view, "[WARN] Perfil IBM i") || !strings.Contains(view, "[--] Preparación local: no evaluada") {
		t.Fatalf("status markers are not readable: %q", view)
	}
	t.Setenv("NO_COLOR", "")
	theme := newHomeTheme(false)
	if got := fmt.Sprint(theme.logo.GetForeground()); got != bacRed {
		t.Fatalf("logo foreground = %q, want BAC red %q", got, bacRed)
	}
	if got := fmt.Sprint(theme.selectedMarker.GetForeground()); got != bacRed {
		t.Fatalf("selected marker foreground = %q, want BAC red %q", got, bacRed)
	}
	if got := fmt.Sprint(theme.selectedLabel.GetForeground()); got != textPrimary {
		t.Fatalf("selected label foreground = %q, want primary text %q", got, textPrimary)
	}
	if got := fmt.Sprint(theme.selectedRow.GetBackground()); got != insetSurface {
		t.Fatalf("selected row surface = %q, want inset surface %q", got, insetSurface)
	}
}

func TestHomeDetailedAndRecommendedLogoSelection(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.noColor = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	frameWidth, _ := updated.(Model).shellFrameDimensions()
	if got, art := updated.(Model).heroLogo(updated.(Model).shellInnerWidth(frameWidth)); got != "detailed" {
		t.Fatalf("hero label = %q, want detailed with art %q", got, art)
	}
	compact, _ := updated.(Model).Update(tea.WindowSizeMsg{Width: 50, Height: 30})
	frameWidth, _ = compact.(Model).shellFrameDimensions()
	if got, _ := compact.(Model).heroLogo(compact.(Model).shellInnerWidth(frameWidth)); got == "detailed" {
		t.Fatalf("compact layout should drop detailed logo, got %q, frame=%d", got, frameWidth)
	}
}

func TestTUIProgramUsesAlternateScreen(t *testing.T) {
	if !defaultTUIProgramConfig().altScreen {
		t.Fatal("TUI program configuration must enable the alternate screen")
	}
	if len(tuiProgramOptions(t.Context())) != 2 {
		t.Fatal("TUI program options must include context and alternate screen")
	}
}

func TestHomeShellFitsTerminalViewportWithMargin(t *testing.T) {
	m := NewModel(&profileStoreStub{profiles: []profile.Profile{testProfile("dev")}})
	m.noColor = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	view := updated.(Model).View()
	if w := lipgloss.Width(view); w > 120 {
		t.Fatalf("shell width = %d, exceeds terminal width 120: %q", w, view)
	}
	if h := lipgloss.Height(view); h > 36 {
		t.Fatalf("shell height = %d, exceeds terminal height 36: %q", h, view)
	}
	frameWidth, frameHeight := updated.(Model).shellFrameDimensions()
	if frameWidth+4 > 120 || frameHeight+4 > 36 {
		t.Fatalf("frame dimensions %dx%d do not leave the intended one-cell margin", frameWidth, frameHeight)
	}
}

func TestHomeMenuRowsKeepIdenticalGeometryWhenSelectionMoves(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.noColor = true
	m.homeSelected = actionCreate
	width := 60
	first := m.renderMenu(width, newHomeTheme(true))
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	second := m.renderMenu(width, newHomeTheme(true))
	firstRows := strings.Split(strings.TrimRight(first, "\n"), "\n")
	secondRows := strings.Split(strings.TrimRight(second, "\n"), "\n")
	if len(firstRows) != len(secondRows) {
		t.Fatalf("menu row count drifted when selection moved: %d vs %d", len(firstRows), len(secondRows))
	}
	for i := range firstRows {
		if lipgloss.Height(firstRows[i]) != lipgloss.Height(secondRows[i]) {
			t.Fatalf("row %d height drifted when selection moved: %q vs %q", i, firstRows[i], secondRows[i])
		}
		if i != len(firstRows)-1 {
			if (firstRows[i] == "") != (secondRows[i] == "") {
				t.Fatalf("row %d spacing drift: %q vs %q", i, firstRows[i], secondRows[i])
			}
		}
	}
}

func TestHomeMenuRowsSpanFullWidthAndUseAlignedMarker(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.noColor = true
	view := m.renderMenu(60, newHomeTheme(true))
	rows := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(rows) == 0 {
		t.Fatalf("renderMenu produced no rows")
	}
	contentRows := 0
	for _, row := range rows {
		if strings.TrimSpace(row) == "" {
			continue
		}
		contentRows++
		if got := lipgloss.Width(row); got != 60 {
			t.Fatalf("menu row width = %d, want 60: %q", got, row)
		}
	}
	if contentRows < 3 {
		t.Fatalf("renderMenu did not produce enough content rows: %q", view)
	}
	markerIdx := strings.Index(view, "▸")
	if markerIdx < 0 {
		t.Fatal("selected marker missing")
	}
	label := "Crear un perfil"
	labelIdx := strings.Index(view, label)
	if labelIdx < 0 {
		t.Fatal("selected label missing")
	}
	if labelIdx-markerIdx < 3 {
		t.Fatalf("marker-to-label spacing = %d, expected wider separation", labelIdx-markerIdx)
	}
}

func TestHomeHeaderIsLeftAlignedWithModestPadding(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	header := updated.(Model).renderStatusHeader(120, newHomeTheme(false))
	if !strings.HasPrefix(header, "  ") {
		t.Fatalf("header must begin with the modest internal padding, got %q", header)
	}
	trimmed := strings.TrimLeft(header, " ")
	if strings.HasPrefix(trimmed, " ") {
		t.Fatalf("header must be left-aligned after the modest padding, got %q", header)
	}
}

func TestHomeReadinessFieldsetTitleIsIntegratedIntoBorder(t *testing.T) {
	m := NewModel(&profileStoreStub{profiles: []profile.Profile{testProfile("dev")}})
	m.noColor = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	model := updated.(Model)
	frameWidth, _ := model.shellFrameDimensions()
	inner := model.shellInnerWidth(frameWidth)
	leftWidth, _, _ := desktopBodyDimensions(model.bodyWidth(inner))
	field := model.renderReadinessFieldset(leftWidth, newHomeTheme(true))
	lines := strings.Split(field, "\n")
	if len(lines) < 11 || len(lines) > 14 {
		t.Fatalf("fieldset height = %d, expected 11-14 rows at target size: %q", len(lines), field)
	}
	if got := lipgloss.Width(field); got != leftWidth {
		t.Fatalf("fieldset width = %d, expected %d", got, leftWidth)
	}
	top := lines[0]
	if !strings.HasPrefix(top, "┌") || !strings.HasSuffix(top, "┐") {
		t.Fatalf("top border missing corners: %q", top)
	}
	if !strings.Contains(top, "Resumen de preparación") {
		t.Fatalf("title must be integrated into the top border: %q", top)
	}
	for _, line := range lines {
		if got := lipgloss.Width(line); got > leftWidth {
			t.Fatalf("inner fieldset line exceeded width %d: %q", leftWidth, line)
		}
	}
}

func TestHomeReadinessFieldsetPreservesTitleInBorderWithoutColor(t *testing.T) {
	m := NewModel(&profileStoreStub{profiles: []profile.Profile{testProfile("dev")}})
	m.noColor = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	model := updated.(Model)
	field := model.renderReadinessFieldset(50, newHomeTheme(true))
	lines := strings.Split(field, "\n")
	if !strings.Contains(lines[0], "Resumen de preparación") {
		t.Fatalf("no-color fieldset lost the title-in-border: %q", field)
	}
	if !strings.Contains(lines[len(lines)-1], "└") {
		t.Fatalf("no-color fieldset missing bottom border: %q", field)
	}
}

func TestHomeFooterUsesApprovedTextAndIsCentered(t *testing.T) {
	m := NewModel(&profileStoreStub{profiles: []profile.Profile{testProfile("dev")}})
	m.noColor = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := updated.(Model).View()
	want := "↑/↓ navegar  •  Enter seleccionar  •  ? ayuda  •  q salir"
	if !strings.Contains(view, want) {
		t.Fatalf("footer text not approved: missing %q in %q", want, view)
	}
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if !strings.Contains(lines[len(lines)-2], want) {
		t.Fatalf("footer is not the second-to-last inner line: %q", lines[len(lines)-2])
	}
}

func TestHomeHelpUpdatesOnlyBoundedFeedback(t *testing.T) {
	store := &profileStoreStub{profiles: []profile.Profile{testProfile("dev")}}
	m := NewModel(store)
	m.noColor = true
	m.err = errors.New("stale Home error")
	m.homeSelected = actionCreate
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	model := updated.(Model)

	if cmd != nil {
		t.Fatal("help scheduled a command")
	}
	if model.screen != screenHome || model.homeSelected != actionCreate {
		t.Fatalf("help changed Home state: screen=%v selection=%q", model.screen, model.homeSelected)
	}
	if len(store.profiles) != 1 || store.profiles[0].Name != "dev" {
		t.Fatalf("help mutated profiles: %#v", store.profiles)
	}
	if model.err != nil {
		t.Fatalf("help did not clear stale error: %v", model.err)
	}
	if got, want := model.status, "Ayuda: ↑/↓ navegar; Enter seleccionar; q salir."; got != want {
		t.Fatalf("help feedback = %q, want %q", got, want)
	}

	const constrainedWidth = 24
	feedback := model.renderFeedback(constrainedWidth, newHomeTheme(true))
	if !strings.Contains(feedback, "Ayuda:") {
		t.Fatalf("help feedback missing from constrained render: %q", feedback)
	}
	for _, line := range strings.Split(feedback, "\n") {
		if got := lipgloss.Width(line); got > constrainedWidth {
			t.Fatalf("bounded help line width = %d, exceeds %d: %q", got, constrainedWidth, line)
		}
	}
}

func TestHomeFooterShowsBuildVersionWithoutMovingCommands(t *testing.T) {
	const width = 114
	build := BuildInfo{Version: "v0.2.0", Revision: "abc123"}
	m := NewModelWithBuildInfo(&profileStoreStub{}, build)
	m.noColor = true
	footer := m.renderFooter(width, newHomeTheme(true))
	version := "BAC NEXUS v0.2.0"
	commands := "↑/↓ navegar  •  Enter seleccionar  •  ? ayuda  •  q salir"
	if !strings.HasPrefix(footer, "  "+version) || strings.Contains(footer, build.Revision) {
		t.Fatalf("footer build label = %q", footer)
	}
	if strings.Contains(footer, "\x1b[") {
		t.Fatalf("no-color footer contains ANSI escape: %q", footer)
	}
	commandIndex := strings.Index(footer, commands)
	if commandIndex < 0 {
		t.Fatalf("footer commands missing: %q", footer)
	}
	if got, want := lipgloss.Width(footer[:commandIndex]), (width-lipgloss.Width(commands))/2; got != want {
		t.Fatalf("command start = %d, want centered start %d", got, want)
	}
	if got := lipgloss.Height(footer); got != 1 {
		t.Fatalf("footer height = %d, want 1", got)
	}
}

func TestHomeFooterDefaultsToDevAndHidesVersionBeforeCommandsOverlap(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.noColor = true
	if footer := m.renderFooter(114, newHomeTheme(true)); !strings.Contains(footer, "BAC NEXUS dev") {
		t.Fatalf("default footer version missing: %q", footer)
	}
	const constrainedWidth = 70
	footer := m.renderFooter(constrainedWidth, newHomeTheme(true))
	commands := "↑/↓ navegar  •  Enter seleccionar  •  ? ayuda  •  q salir"
	if strings.Contains(footer, "BAC NEXUS") || !strings.Contains(footer, commands) || lipgloss.Height(footer) != 1 {
		t.Fatalf("constrained footer overlapped or wrapped: %q", footer)
	}
}

func TestNewModelWithBuildInfoDefaultsEmptyValues(t *testing.T) {
	m := NewModelWithBuildInfo(&profileStoreStub{}, BuildInfo{})
	if m.buildInfo != (BuildInfo{Version: "dev", Revision: "unknown"}) {
		t.Fatalf("build defaults = %#v", m.buildInfo)
	}
}

func TestHomeHeaderUsesDifferentiatedSegmentTones(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	model := updated.(Model)
	t1 := newHomeTheme(false)
	brand, profile, status := model.headerSegments(t1)
	if brand == profile || brand == status || profile == status {
		t.Fatalf("header segments share foreground: %q / %q / %q", brand, profile, status)
	}
	missing, _ := model.Update(profilesMsg{profiles: nil})
	t2 := newHomeTheme(false)
	_, _, urgent := missing.(Model).headerSegments(t2)
	if urgent == "" {
		t.Fatal("zero-profile header status is empty")
	}
	if urgent == status {
		t.Fatal("zero-profile header status did not switch to urgent styling")
	}
}

func TestHomeDesktopBodyUsesStableCenteredCompositionAtWideWidths(t *testing.T) {
	m := NewModel(&profileStoreStub{profiles: []profile.Profile{testProfile("dev")}})
	m.noColor = true
	for _, terminalWidth := range []int{180, 240} {
		t.Run(fmt.Sprintf("%d columns", terminalWidth), func(t *testing.T) {
			updated, _ := m.Update(tea.WindowSizeMsg{Width: terminalWidth, Height: 40})
			model := updated.(Model)
			frameWidth, _ := model.shellFrameDimensions()
			inner := model.shellInnerWidth(frameWidth)
			body := model.renderDesktopBody(inner, newHomeTheme(true))
			bodyLines := strings.Split(body, "\n")
			if got := lipgloss.Width(strings.TrimLeft(bodyLines[0], " ")); got != desktopBodyWidth {
				t.Fatalf("body width = %d, want capped width %d", got, desktopBodyWidth)
			}
			for _, line := range bodyLines {
				left := len(line) - len(strings.TrimLeft(line, " "))
				if left != (inner-desktopBodyWidth)/2 {
					t.Fatalf("line left margin = %d, want %d: %q", left, (inner-desktopBodyWidth)/2, line)
				}
				if right := inner - left - lipgloss.Width(strings.TrimLeft(line, " ")); right < 0 || absoluteDifference(left, right) > 1 {
					t.Fatalf("line margins are not centered: left=%d right=%d line=%q", left, right, line)
				}
			}
			_, gap, _ := desktopBodyDimensions(model.bodyWidth(inner))
			if gap != desktopBodyGap {
				t.Fatalf("desktop gap = %d, want %d", gap, desktopBodyGap)
			}
		})
	}
}

func TestHomeDesktopBodyShrinksWithoutOverflow(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.noColor = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	model := updated.(Model)
	frameWidth, _ := model.shellFrameDimensions()
	inner := model.shellInnerWidth(frameWidth)
	body := model.renderDesktopBody(inner, newHomeTheme(true))
	if got := lipgloss.Width(strings.TrimLeft(strings.Split(body, "\n")[0], " ")); got != inner {
		t.Fatalf("narrow desktop body width = %d, want available width %d", got, inner)
	}
	left, gap, right := desktopBodyDimensions(inner)
	if left >= desktopReadinessWidth || right >= desktopMenuWidth || gap != desktopBodyGap {
		t.Fatalf("narrow desktop dimensions = %d + %d + %d, expected shrunk columns with stable gap", left, gap, right)
	}
}

func TestHomeBrandAndBodyShareCenterAxis(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.noColor = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 240, Height: 40})
	model := updated.(Model)
	frameWidth, _ := model.shellFrameDimensions()
	inner := model.shellInnerWidth(frameWidth)
	brand := model.renderBrand(inner, newHomeTheme(true))
	body := model.renderDesktopBody(inner, newHomeTheme(true))
	identityCenter := -1
	logoCenter := -1
	logoAnchor := strings.TrimSpace(strings.Split(detailedLogo, "\n")[0])
	for _, line := range strings.Split(brand, "\n") {
		if index := strings.Index(line, "BAC NEXUS"); index >= 0 {
			identityCenter = lipgloss.Width(line[:index]) + lipgloss.Width("BAC NEXUS")/2
		}
		if index := strings.Index(line, logoAnchor); index >= 0 {
			logoCenter = lipgloss.Width(line[:index]) + lipgloss.Width(logoAnchor)/2
		}
	}
	if identityCenter < 0 {
		t.Fatal("identity line missing")
	}
	if logoCenter < 0 {
		t.Fatal("logo line missing")
	}
	bodyLine := strings.Split(body, "\n")[0]
	bodyCenter := lipgloss.Width(bodyLine[:len(bodyLine)-len(strings.TrimLeft(bodyLine, " "))]) + desktopBodyWidth/2
	if absoluteDifference(identityCenter, bodyCenter) > 1 {
		t.Fatalf("identity and body centers diverged: identity=%d body=%d", identityCenter, bodyCenter)
	}
	if absoluteDifference(logoCenter, bodyCenter) > 1 {
		t.Fatalf("logo and body centers diverged: logo=%d body=%d", logoCenter, bodyCenter)
	}
}

func absoluteDifference(left, right int) int {
	difference := left - right
	if difference < 0 {
		return -difference
	}
	return difference
}

func TestHomeReadinessFieldsetHeightMatchesTarget(t *testing.T) {
	m := NewModel(&profileStoreStub{profiles: []profile.Profile{testProfile("dev")}})
	m.noColor = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	model := updated.(Model)
	frameWidth, _ := model.shellFrameDimensions()
	body := model.bodyWidth(model.shellInnerWidth(frameWidth))
	leftWidth, _, _ := desktopBodyDimensions(body)
	field := model.renderReadinessFieldset(leftWidth, newHomeTheme(true))
	height := lipgloss.Height(field)
	if height != readinessFieldsetHeight {
		t.Fatalf("readiness fieldset height = %d, want %d", height, readinessFieldsetHeight)
	}
	if !strings.Contains(field, "Resumen de preparación") {
		t.Fatalf("readiness fieldset title missing: %q", field)
	}
	lines := strings.Split(field, "\n")
	if strings.Contains(lines[2], "Perfil IBM i") || !strings.HasPrefix(lines[3], "│    [") {
		t.Fatalf("fieldset must keep two blank top rows and four-cell left padding: %q", field)
	}
}

func TestHomeReadinessFieldsetUsesBorderTokenForEveryBorderSegment(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	theme := newHomeTheme(false)
	field := m.renderReadinessFieldset(desktopReadinessWidth, theme)
	border := theme.fieldsetBorder.Render("┌")
	if !strings.Contains(field, border) {
		t.Fatalf("top-left border does not use fieldset border style: %q", field)
	}
	for _, glyph := range []string{"┐", "│", "└", "┘", "─"} {
		if !strings.Contains(field, theme.fieldsetBorder.Render(glyph)) {
			t.Fatalf("border glyph %q does not use fieldset border style", glyph)
		}
	}
	if got := fmt.Sprint(theme.fieldsetBorder.GetForeground()); got != borderSurface {
		t.Fatalf("fieldset border foreground = %q, want %q", got, borderSurface)
	}
}

func TestHomeMenuUsesOneBlankRowBetweenItemsAtEverySupportedHeight(t *testing.T) {
	for _, height := range []int{36, 42} {
		t.Run(fmt.Sprintf("%d rows", height), func(t *testing.T) {
			m := NewModel(&profileStoreStub{})
			m.noColor = true
			updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: height})
			menu := updated.(Model).renderMenu(72, newHomeTheme(true))
			lines := strings.Split(menu, "\n")
			for i := 0; i < len(lines)-2; i += 2 {
				if strings.TrimSpace(lines[i]) == "" || lines[i+1] != "" || strings.TrimSpace(lines[i+2]) == "" {
					t.Fatalf("menu row rhythm is not one blank line at rows %d-%d: %q", i, i+2, menu)
				}
			}
		})
	}
}

func TestHomeMenuHeadingIsImmediatelyFollowedByFirstItem(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.noColor = true
	block := m.renderMenuBlock(72, newHomeTheme(true))
	lines := strings.Split(block, "\n")
	if len(lines) < 2 || lines[0] != "Perfiles IBM i" || lines[1] == "" || !strings.Contains(lines[1], "Crear un perfil") {
		t.Fatalf("heading must be immediately followed by the first item: %q", block)
	}
}

func TestHomeSelectedHighlightIsCappedAndShrinksSafely(t *testing.T) {
	for _, tt := range []struct {
		menuWidth int
		want      int
	}{
		{menuWidth: 72, want: selectedHighlightMaxWidth},
		{menuWidth: 48, want: 48},
	} {
		t.Run(fmt.Sprintf("%d columns", tt.menuWidth), func(t *testing.T) {
			if got := selectedHighlightWidth(tt.menuWidth); got != tt.want {
				t.Fatalf("highlight width = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestHomeMenuSelectionKeepsLabelsAlignedAndOverflowVisible(t *testing.T) {
	rows := append([]homeMenuRow{{actionReadiness, "Verificar preparación", "readiness", false}}, testHomeRows(7)...)
	m := NewModel(&profileStoreStub{})
	m.noColor = true
	m.homeMenuRows = rows
	m.homeSelected = actionReadiness
	menu := m.renderMenu(72, newHomeTheme(true))
	first := strings.Split(menu, "\n")[0]
	if !strings.Contains(first, "Verificar preparación") || !strings.Contains(menu, "▼") || lipgloss.Width(first) != 72 {
		t.Fatalf("selected capped row lost label, indicator, or full row geometry: %q", menu)
	}
	selectedIndex := strings.Index(first, "Verificar preparación")
	selectedOrigin := lipgloss.Width(first[:selectedIndex])
	m.homeSelected = rows[1].id
	menu = m.renderMenu(72, newHomeTheme(true))
	for _, line := range strings.Split(menu, "\n") {
		if strings.Contains(line, "Verificar preparación") {
			if index := strings.Index(line, "Verificar preparación"); lipgloss.Width(line[:index]) != selectedOrigin {
				origin := lipgloss.Width(line[:index])
				t.Fatalf("selection shifted label origin: selected=%d unselected=%d", selectedOrigin, origin)
			}
			break
		}
	}
}

func TestHomeMenuWindowKeepsSelectionVisibleAndShowsOnlyNeededIndicators(t *testing.T) {
	rows := testHomeRows(8)
	m := NewModel(&profileStoreStub{})
	m.noColor = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	m.homeMenuRows = rows
	m.homeSelected = rows[0].id
	if menu := NewModel(&profileStoreStub{}).renderMenu(60, newHomeTheme(true)); strings.Contains(menu, "▲") || strings.Contains(menu, "▼") {
		t.Fatalf("menu with fitting window unexpectedly rendered indicators: %q", menu)
	}

	for range 5 {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(Model)
	}
	menu := m.renderMenu(60, newHomeTheme(true))
	if !strings.Contains(menu, "Elemento 5") || !strings.Contains(menu, "▼") || strings.Contains(menu, "▲") {
		t.Fatalf("menu did not expose only remaining downward overflow: %q", menu)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(Model)
	menu = m.renderMenu(60, newHomeTheme(true))
	if !strings.Contains(menu, "Elemento 6") || !strings.Contains(menu, "▲") || !strings.Contains(menu, "▼") {
		t.Fatalf("menu did not keep selected j/k item visible with both indicators: %q", menu)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if menu = m.renderMenu(60, newHomeTheme(true)); !strings.Contains(menu, "Elemento 5") {
		t.Fatalf("menu did not keep selected arrow-key item visible: %q", menu)
	}
	block := m.renderMenuBlock(60, newHomeTheme(true))
	if !strings.HasPrefix(block, "Perfiles IBM i\n") {
		t.Fatalf("menu heading moved with the item window: %q", block)
	}
}

func TestHomeReadinessWindowFocusAndIndicators(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.noColor = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	m.homeReadinessRows = testReadinessRows(readinessWindowCapacity + 3)
	if m.homeFocus != homeFocusMenu {
		t.Fatalf("Home focus = %d, want menu", m.homeFocus)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.homeFocus != homeFocusReadiness {
		t.Fatal("tab did not enter scrollable readiness")
	}
	selected := m.homeSelected
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	field := m.renderReadinessFieldset(74, newHomeTheme(true))
	if m.homeSelected != selected || m.readinessOffset != 1 || !strings.Contains(field, "▲") || !strings.Contains(field, "▼") {
		t.Fatalf("readiness scrolling did not stay bounded and focused: selected=%q offset=%d field=%q", m.homeSelected, m.readinessOffset, field)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(Model)
	if m.readinessOffset != 0 {
		t.Fatalf("readiness k scroll offset = %d, want 0", m.readinessOffset)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if updated.(Model).homeFocus != homeFocusMenu {
		t.Fatal("escape did not return Home focus to menu")
	}
}

func TestHomeReadinessWindowHasNoIndicatorWhenRowsFit(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.noColor = true
	m.homeReadinessRows = testReadinessRows(readinessWindowCapacity)
	field := m.renderReadinessFieldset(74, newHomeTheme(true))
	if strings.Contains(field, "▲") || strings.Contains(field, "▼") {
		t.Fatalf("fitting readiness rows unexpectedly rendered indicators: %q", field)
	}
}

func TestHomeStatusThemeUsesSemanticTokensWithoutBACRed(t *testing.T) {
	theme := newHomeTheme(false)
	roles := map[string]lipgloss.Style{
		"[OK]":   theme.statusOK,
		"[INFO]": theme.statusInfo,
		"[WARN]": theme.statusWarning,
		"[ERR]":  theme.statusError,
		"[--]":   theme.statusNeutral,
		"[....]": theme.statusProgress,
	}
	for marker, style := range roles {
		if got := fmt.Sprint(style.GetForeground()); got == "" || got == bacRed {
			t.Fatalf("status marker %s foreground = %q, must be semantic and not BAC Red", marker, got)
		}
	}
	noColor := newHomeTheme(true)
	for marker := range roles {
		if got := NewModel(&profileStoreStub{}).renderStatusRow(marker+" estado", 40, "", noColor); !strings.Contains(got, marker) {
			t.Fatalf("no-color status row lost marker %q: %q", marker, got)
		}
	}
}

func TestHomeFooterSeparatorUsesBorderTokenAndFooterStaysPinned(t *testing.T) {
	m := NewModel(&profileStoreStub{})
	m.noColor = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model := updated.(Model)
	view := model.View()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if !strings.Contains(lines[len(lines)-2], "↑/↓ navegar  •  Enter seleccionar  •  ? ayuda  •  q salir") {
		t.Fatalf("footer text or pinning changed: %q", lines[len(lines)-2])
	}
	separator := lines[len(lines)-3]
	frameWidth, _ := model.shellFrameDimensions()
	separator = strings.TrimSpace(separator)
	separator = strings.TrimPrefix(strings.TrimSuffix(separator, "│"), "│")
	if got := lipgloss.Width(separator); got != frameWidth {
		t.Fatalf("footer separator width = %d, want %d: %q", got, frameWidth, separator)
	}
	if got := fmt.Sprint(newHomeTheme(false).fieldsetBorder.GetForeground()); got != borderSurface {
		t.Fatalf("footer separator foreground = %q, want %q", got, borderSurface)
	}
}

func testHomeRows(count int) []homeMenuRow {
	rows := make([]homeMenuRow, count)
	for i := range rows {
		rows[i] = homeMenuRow{id: homeActionID(fmt.Sprintf("test-%d", i)), label: fmt.Sprintf("Elemento %d", i), routes: false}
	}
	return rows
}

func testReadinessRows(count int) []string {
	rows := make([]string, count)
	for i := range rows {
		rows[i] = fmt.Sprintf("[INFO] Estado %d", i)
	}
	return rows
}

func (m Model) shellFrameDimensionsInner() (int, int) {
	return m.shellFrameDimensions()
}

func TestHomeShellPinsFooterAndKeepsHeroNearTop(t *testing.T) {
	m := NewModel(&profileStoreStub{profiles: []profile.Profile{testProfile("dev")}})
	m.noColor = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := updated.(Model).View()
	if w := lipgloss.Width(view); w > 120 {
		t.Fatalf("shell width = %d, exceeds terminal width 120", w)
	}
	if h := lipgloss.Height(view); h > 40 {
		t.Fatalf("shell height = %d, exceeds terminal height 40", h)
	}
	inner := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(inner) < 3 || !strings.HasPrefix(inner[0], "┌") || !strings.HasSuffix(inner[len(inner)-1], "┘") {
		t.Fatalf("missing frame border: %q", view)
	}
	content := inner[1 : len(inner)-1]
	if len(content) == 0 || !strings.Contains(content[0], "BAC NEXUS") {
		t.Fatalf("first inner line is not the header: %q", content)
	}
	heroIdx := -1
	bodyIdx := -1
	heroAnchor := strings.Split(detailedLogo, "\n")[0]
	for i, line := range content {
		switch {
		case heroIdx < 0 && strings.Contains(line, heroAnchor):
			heroIdx = i
		case bodyIdx < 0 && strings.Contains(line, "Resumen de preparación"):
			bodyIdx = i
		}
	}
	if heroIdx < 0 {
		t.Fatal("hero logo missing from tall viewport")
	}
	if bodyIdx < 0 {
		t.Fatal("body missing from tall viewport")
	}
	footerIdx := len(content) - 1
	if !strings.Contains(content[footerIdx], "↑/↓") {
		t.Fatalf("last inner line is not the footer: %q", content[footerIdx])
	}
	if heroIdx >= footerIdx || bodyIdx >= footerIdx {
		t.Fatalf("hero/body must precede footer; hero=%d body=%d footer=%d", heroIdx, bodyIdx, footerIdx)
	}
	if heroIdx > 25 {
		t.Fatalf("hero drifted from upper region: hero=%d", heroIdx)
	}
	if bodyIdx < heroIdx {
		t.Fatalf("body must follow hero: hero=%d body=%d", heroIdx, bodyIdx)
	}
	trimmed := strings.TrimRight(view, "\n")
	lines := strings.Split(trimmed, "\n")
	if len(lines) < 3 {
		t.Fatalf("shell render has fewer than three lines: %q", view)
	}
	if !strings.Contains(lines[len(lines)-2], "↑/↓") {
		t.Fatalf("line before the bottom border does not contain the footer: %q", lines[len(lines)-2])
	}
	if !strings.Contains(lines[len(lines)-1], "└") {
		t.Fatalf("bottom border line is missing: %q", lines[len(lines)-1])
	}
}

func TestHomeShellCollapsesSafelyWhenConstrained(t *testing.T) {
	m := NewModel(&profileStoreStub{profiles: []profile.Profile{testProfile("dev")}})
	m.noColor = true
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 18})
	view := updated.(Model).View()
	if w := lipgloss.Width(view); w > 80 {
		t.Fatalf("constrained shell width = %d, exceeds terminal width 80", w)
	}
	if h := lipgloss.Height(view); h > 18 {
		t.Fatalf("constrained shell height = %d, exceeds terminal height 18", h)
	}
	if !strings.Contains(view, "↑/↓") {
		t.Fatalf("constrained shell lost the footer: %q", view)
	}
}

func TestCompiledLogosMatchAuthoritativeDesignArtifact(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("assets", "brand", "bac_nexus_braille_logo_variants.txt"))
	if err != nil {
		t.Fatalf("read authoritative logo artifact: %v", err)
	}
	normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
	for name, logo := range map[string]string{"recommended": recommendedLogo, "compact": compactLogo} {
		if !strings.Contains(normalized, logo) {
			t.Fatalf("%s compiled logo drifted from authoritative artifact", name)
		}
	}
}

func TestModelDetailDeleteAndBack(t *testing.T) {
	store := &profileStoreStub{profiles: []profile.Profile{testProfile("dev")}}
	m := NewModel(store)
	m.screen = screenList
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
	m.screen = screenList
	updated, _ := m.Update(profilesMsg{profiles: store.profiles})
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(Model)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if updated.(Model).screen != screenConfirm || store.deleted != "" {
		t.Fatal("single-key confirmation triggered deletion")
	}
	m = NewModel(store)
	m.screen = screenList
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
	m.noColor = true
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
