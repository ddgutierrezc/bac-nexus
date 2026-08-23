package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The variants are copied verbatim from the repository-root authoritative
// design artifact. Keeping them compiled makes the binary independent of CWD.
const recommendedLogo = " ⢀⣀⣤⣤⣶⣾\n⣿⣿⣿⣿⡿⠟⣋ ⣦⡀\n⠿⢛⣩⣴⣾⣿⣿ ⣿⣿⣦⣀\n⣾⣿⣿⣿⠟⣋⣴ ⣿⣿⣿⡏\n⣿⠿⣋⣵⣾⣿⡿⢀⣛⣛⠛\n⣵⣾⣿⣿⡿⢋⣴⣿⣿⣿"
const compactLogo = "⢀⣀⣤⣴⣶⣿\n⣿⣿⠿⢟⣫⣵⢸⣦⡀\n⣩⣴⣾⣿⡿⢛⢸⣿⣿⡶\n⣿⡿⢛⣥⣾⣿⠸⠿⠿⠃\n⣥⣾⣿⣿⢟⣵⣿⣿⡇"
const detailedLogo = "   ⣀⣀⣤⣤⣶⣾⡇\n⣶⣾⣿⣿⣿⣿⣿⠿⠟⡃⢠⣄\n⣿⣿⡿⠟⢛⣩⣴⣶⣿⡇⢸⣿⣷⣤⡀\n⣩⣤⣶⣿⣿⣿⣿⡿⠟⡁⢸⣿⣿⣿⣿⡶\n⣿⣿⣿⣿⠿⢛⣡⣶⣿⡇⢸⣿⣿⣿⣿⠁\n⣿⠟⣋⣥⣾⣿⣿⣿⡿⢃⣬⣭⣭⡍⠁\n⣴⣾⣿⣿⣿⣿⡿⢋⣴⣿⣿⣿⣿⡇"

type homeActionID string

type homeFocus uint8

const (
	homeFocusMenu homeFocus = iota
	homeFocusReadiness
)

const (
	actionManage        homeActionID = "manage"
	actionCreate        homeActionID = "create"
	actionReadiness     homeActionID = "readiness"
	actionDiagnostics   homeActionID = "diagnostics"
	actionIntegrations  homeActionID = "integrations"
	actionConfiguration homeActionID = "configuration"
	actionExit          homeActionID = "exit"
)

// headerLeftPadding matches the modest internal horizontal padding the
// target header reserves before its first segment so the left-aligned copy
// does not touch the inner shell edge.
const headerLeftPadding = 2

const (
	menuWindowCapacity        = 6
	menuRowGapLines           = 1
	selectedHighlightMaxWidth = 60
	readinessWindowCapacity   = readinessFieldsetHeight - 2 - readinessFieldsetTopPadding
)

// homeMenuRow describes a menu row for the current Home render.
type homeMenuRow struct {
	id     homeActionID
	label  string
	target string
	routes bool
}

// homeAction is the legacy internal representation consumed by the model
// routing helpers. The Home render builds these from the menu rows above.
type homeAction struct {
	id        homeActionID
	label     string
	available bool
}

func (m Model) emptyHomeMenu() []homeMenuRow {
	return []homeMenuRow{
		{actionCreate, "Crear un perfil", "create", true},
		{actionReadiness, "Verificar preparación", "readiness", false},
		{actionDiagnostics, "Diagnósticos", "diagnostics", false},
		{actionIntegrations, "Integraciones MCP", "integrations", false},
		{actionConfiguration, "Configuración", "configuration", false},
		{actionExit, "Salir", "exit", true},
	}
}

func (m Model) populatedHomeMenu() []homeMenuRow {
	rows := []homeMenuRow{
		{actionManage, "Administrar perfiles", "manage", true},
	}
	rows = append(rows, m.emptyHomeMenu()...)
	return rows
}

// renderHome dispatches to the responsive layout.
func (m Model) renderHome() string {
	switch m.homeLayout() {
	case homeLayoutDesktop:
		return m.renderShell()
	case homeLayoutCompact:
		return m.renderShell()
	default:
		return m.renderMinimalHome()
	}
}

type layoutMode uint8

const (
	homeLayoutMinimal layoutMode = iota
	homeLayoutCompact
	homeLayoutDesktop
)

func (m Model) homeLayout() layoutMode {
	width, height := m.homeDimensions()
	if width >= 96 && height >= 36 {
		return homeLayoutDesktop
	}
	if width >= lipgloss.Width(compactLogo)+10 && height >= 24 {
		return homeLayoutCompact
	}
	return homeLayoutMinimal
}

func (m Model) homeDimensions() (int, int) {
	if m.width == 0 {
		return 120, 36
	}
	return m.width, m.height
}

// shellFrameDimensions returns the outer frame width/height so the fully
// rendered shell, including its single border, fits within the measured
// terminal viewport and leaves a one-cell margin on each side where the
// terminal is large enough. Lip Gloss borders consume one row and one column
// per side, which is reserved here.
func (m Model) shellFrameDimensions() (int, int) {
	width, height := m.homeDimensions()
	const margin = 1
	frameWidth := width - 2*margin - 2
	frameHeight := height - 2*margin - 2
	if frameWidth < 4 {
		frameWidth = max(width-2, 4)
	}
	if frameHeight < 6 {
		frameHeight = max(height-2, 6)
	}
	return frameWidth, frameHeight
}

func (m Model) shellInnerWidth(frameWidth int) int {
	return max(frameWidth-2, 1)
}

// shellInnerHeight returns the height of the inner content area inside the
// frame's borders so the shell layout can fit the rendered content.
func (m Model) shellInnerHeight(frameHeight int) int {
	return max(frameHeight-2, 1)
}

// heroLogo returns the largest logo variant that fits the shell width.
func (m Model) heroLogo(inner int) (label, art string) {
	switch {
	case inner >= 60:
		return "detailed", detailedLogo
	case inner >= lipgloss.Width(recommendedLogo)+6:
		return "recommended", recommendedLogo
	case inner >= lipgloss.Width(compactLogo)+4:
		return "compact", compactLogo
	default:
		return "", ""
	}
}

// renderShell produces a full-height outer frame with a pinned header and
// footer and a centered brand hero above the body. The outer frame is sized
// so that its border stays within the measured terminal viewport.
func (m Model) renderShell() string {
	frameWidth, frameHeight := m.shellFrameDimensions()
	inner := m.shellInnerWidth(frameWidth)
	height := m.shellInnerHeight(frameHeight)
	t := newHomeTheme(m.noColor)
	header := m.renderStatusHeader(inner, t)
	separator := t.headerSeparator.Width(inner).Render(strings.Repeat("─", inner))
	brand := m.renderBrand(inner, t)
	body := m.renderBody(inner, t)
	feedback := m.renderFeedback(inner, t)
	footer := m.renderFooter(inner, t)
	footerSeparator := t.fieldsetBorder.Render(strings.Repeat("─", inner+2))
	layout := t.shellLayout(inner)
	layout.Add(header)
	layout.Add(separator)
	layout.AddGap(2)
	layout.Add(brand)
	layout.AddGap(1)
	layout.Add(body)
	if feedback != "" {
		layout.AddGap(1)
		layout.Add(feedback)
	}
	layout.AddStretch()
	layout.AddFooter(footerSeparator + "\n" + footer)
	content := layout.Render(height)
	return t.frame.Width(frameWidth).Height(frameHeight).Render(content)
}

// shellHeight is retained for compatibility and returns the inner content
// height; callers should normally prefer shellInnerHeight.
func (m Model) shellHeight() int {
	_, frameHeight := m.shellFrameDimensions()
	return m.shellInnerHeight(frameHeight)
}

const (
	desktopReadinessWidth = 74
	desktopMenuWidth      = 72
	desktopBodyGap        = 8
	desktopBodyWidth      = desktopReadinessWidth + desktopBodyGap + desktopMenuWidth
)

// bodyWidth returns the horizontal body composition width. Desktop compositions
// retain their Stitch proportions at wide sizes, while narrower layouts shrink
// to the shell width without overflowing it.
func (m Model) bodyWidth(inner int) int {
	return min(inner, desktopBodyWidth)
}

func (m Model) renderBody(width int, t homeTheme) string {
	if m.homeLayout() == homeLayoutDesktop {
		return m.renderDesktopBody(width, t)
	}
	return m.renderStackedBody(width, t)
}

// renderDesktopBody renders the readiness group on the left and the menu on
// the right, with no border around the menu section to match the reference.
// The two-column band is centered within the inner shell.
func (m Model) renderDesktopBody(width int, t homeTheme) string {
	body := m.bodyWidth(width)
	leftWidth, gap, rightWidth := desktopBodyDimensions(body)
	if rightWidth < 18 {
		return m.renderStackedBody(width, t)
	}
	left := m.renderReadinessFieldset(leftWidth, t)
	right := m.renderMenuBlock(rightWidth, t)
	joined := lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right)
	return centerHomeBlock(width, joined)
}

func desktopBodyDimensions(width int) (left, gap, right int) {
	if width >= desktopBodyWidth {
		return desktopReadinessWidth, desktopBodyGap, desktopMenuWidth
	}

	gap = desktopBodyGap
	availableColumns := width - gap
	left = (availableColumns * desktopReadinessWidth) / (desktopReadinessWidth + desktopMenuWidth)
	right = availableColumns - left
	return left, gap, right
}

// renderStackedBody renders readiness and menu vertically while preserving
// the unbordered menu block and readiness fieldset. The stack is centered
// within the inner shell width to mirror the desktop rhythm.
func (m Model) renderStackedBody(width int, t homeTheme) string {
	body := m.bodyWidth(width)
	left := m.renderReadinessFieldset(body, t)
	right := m.renderMenuBlock(body, t)
	rows := []string{left, "", right}
	return centerHomeBlock(width, strings.Join(rows, "\n"))
}

// centerHomeBlock pads every rendered line, rather than only the first, so a
// multiline composition remains centered inside the shell at every height.
func centerHomeBlock(width int, block string) string {
	blockWidth := lipgloss.Width(block)
	if width <= blockWidth {
		return block
	}
	padding := strings.Repeat(" ", (width-blockWidth)/2)
	lines := strings.Split(block, "\n")
	for i, line := range lines {
		lines[i] = padding + line
	}
	return strings.Join(lines, "\n")
}

// readinessFieldsetHeight matches the visually taller Stitch target fieldset
// for desktop layouts.
const readinessFieldsetHeight = 13

const (
	readinessFieldsetTopPadding  = 2
	readinessFieldsetSidePadding = 4
)

// renderReadinessFieldset renders the readiness group as a low-contrast
// fieldset whose title is integrated into the top border. The total
// rendered width matches the requested width exactly and no inner content
// line overflows it. No-color mode preserves the same border geometry so
// the title-in-border placement stays readable.
func (m Model) renderReadinessFieldset(width int, t homeTheme) string {
	if width < 12 {
		return m.readinessSummary()
	}
	return m.renderReadinessFieldsetBox(width, t)
}

// renderReadinessFieldsetBox assembles the deterministic low-contrast
// fieldset with the title visually integrated into the top border.
func (m Model) renderReadinessFieldsetBox(width int, t homeTheme) string {
	if width < 12 {
		return m.readinessSummary()
	}
	lines := m.readinessFieldsetLines(width, t)
	return strings.Join(lines, "\n")
}

func (m Model) readinessFieldsetLines(width int, t homeTheme) []string {
	innerWidth := width
	title := readinessFieldsetTitle
	prefixWidth := 1 + lipgloss.Width(title) + 1
	fillTotal := innerWidth - 2 - prefixWidth
	if fillTotal < 0 {
		fillTotal = 0
	}
	fillBefore := fillTotal / 2
	fillAfter := fillTotal - fillBefore
	top := t.fieldsetBorder.Render("┌"+strings.Repeat("─", fillBefore)+" ") +
		t.fieldsetTitle.Render(title) +
		t.fieldsetBorder.Render(" "+strings.Repeat("─", fillAfter)+"┐")
	content := m.readinessRows()
	window := m.visibleReadinessWindow()
	lines := make([]string, 0, readinessFieldsetHeight+2)
	lines = append(lines, top)
	padInner := innerWidth - 2 - 2*readinessFieldsetSidePadding
	if padInner < 1 {
		padInner = 1
	}
	blank := t.fieldsetBorder.Render("│") +
		t.fieldsetContent.Render(strings.Repeat(" ", innerWidth-2)) +
		t.fieldsetBorder.Render("│")
	for range readinessFieldsetTopPadding {
		lines = append(lines, blank)
	}
	for i, line := range content[window.Start:window.End] {
		indicator := ""
		if i == 0 && window.Above {
			indicator = "▲"
		}
		if i == window.End-window.Start-1 && window.Below {
			indicator = "▼"
		}
		body := m.renderStatusRow(line, padInner, indicator, t)
		visible := lipgloss.Width(body)
		space := padInner - visible
		if space < 0 {
			space = 0
		}
		lines = append(lines,
			t.fieldsetBorder.Render("│")+
				t.fieldsetContent.Render(strings.Repeat(" ", readinessFieldsetSidePadding)+body+strings.Repeat(" ", space+readinessFieldsetSidePadding))+
				t.fieldsetBorder.Render("│"),
		)
	}
	targetContentLines := readinessFieldsetHeight - 2
	for len(lines) < targetContentLines+1 {
		lines = append(lines, blank)
	}
	bottom := t.fieldsetBorder.Render("└" + strings.Repeat("─", innerWidth-2) + "┘")
	lines = append(lines, bottom)
	for i, line := range lines {
		if got := lipgloss.Width(line); got != innerWidth {
			lines[i] = truncateToDisplayWidth(line, innerWidth)
		}
	}
	return lines
}

// boundedWindow describes a fixed-height slice of a longer sequence. The same
// deterministic algorithm keeps both custom Home regions bounded without
// introducing a second Bubble Tea child model.
type boundedWindow struct {
	Start int
	End   int
	Above bool
	Below bool
}

func windowLines(total, selected, offset, capacity int) boundedWindow {
	if total <= 0 || capacity <= 0 {
		return boundedWindow{}
	}
	capacity = min(capacity, total)
	maximumOffset := total - capacity
	offset = clamp(offset, maximumOffset+1)
	if selected >= 0 {
		selected = clamp(selected, total)
		if selected < offset {
			offset = selected
		}
		if selected >= offset+capacity {
			offset = selected - capacity + 1
		}
	}
	end := min(offset+capacity, total)
	return boundedWindow{Start: offset, End: end, Above: offset > 0, Below: end < total}
}

func (m Model) readinessRows() []string {
	if m.homeReadinessRows != nil {
		return m.homeReadinessRows
	}
	return strings.Split(m.readinessSummary(), "\n")
}

func (m Model) visibleReadinessWindow() boundedWindow {
	return windowLines(len(m.readinessRows()), -1, m.readinessOffset, readinessWindowCapacity)
}

func (m Model) readinessCanScroll() bool {
	return len(m.readinessRows()) > readinessWindowCapacity
}

func (m Model) moveReadinessOffset(delta int) int {
	rows := m.readinessRows()
	maximumOffset := max(len(rows)-readinessWindowCapacity, 0)
	return clamp(m.readinessOffset+delta, maximumOffset+1)
}

// truncateToDisplayWidth returns the input truncated by display width,
// preserving runes exactly and trimming trailing partial-rune data.
func truncateToDisplayWidth(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		candidate := b.String() + string(r)
		if lipgloss.Width(candidate) > width {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}

const readinessFieldsetTitle = "Resumen de preparación"

func (m Model) renderMenuBlock(width int, t homeTheme) string {
	if width < 12 {
		return m.renderMenu(width, t)
	}
	heading := t.menuHeading.Render("Perfiles IBM i")
	body := m.renderMenu(width, t)
	return heading + "\n" + body
}

func (m Model) renderMinimalHome() string {
	frameWidth, _ := m.shellFrameDimensions()
	contentWidth := m.shellInnerWidth(frameWidth)
	var b strings.Builder
	b.WriteString("BAC NEXUS\n")
	t := newHomeTheme(true)
	b.WriteString(t.menuHeading.Render("Perfiles IBM i") + "\n")
	for _, row := range m.minimalMenu() {
		line := "  " + row.label
		if row.id == m.homeSelected {
			line = "▸ " + row.label
		}
		line = fitHomeLine(line, contentWidth)
		if row.id == m.homeSelected {
			line = t.selectedRow.Render(t.selectedMarker.Render("▸") + " " + t.selectedLabel.Render(strings.TrimPrefix(line, "▸ ")))
		}
		b.WriteString(line + "\n")
	}
	if m.status != "" {
		fmt.Fprintln(&b, fitHomeLine("[--] "+m.status, contentWidth))
	}
	if m.err != nil {
		fmt.Fprintln(&b, fitHomeLine("[ERR] "+sanitizeError(m.err), contentWidth))
	}
	b.WriteString(m.renderFooter(contentWidth, t))
	return b.String()
}

func (m Model) renderStatusHeader(width int, t homeTheme) string {
	brand, profile, status := m.headerSegments(t)
	joined := strings.Join([]string{brand, "│", profile, "│", status}, "  ")
	padded := strings.Repeat(" ", headerLeftPadding) + joined
	return t.header.Width(width).Align(lipgloss.Left).Render(padded)
}

func (m Model) headerSegments(t homeTheme) (brand, profile, status string) {
	statusStyle := t.headerStatus
	switch m.headerStatusState() {
	case headerUrgent:
		statusStyle = t.headerStatusUrgent
	}
	brand = t.headerBrand.Render("BAC NEXUS")
	profile = t.headerProfile.Render(m.headerProfileSegment())
	status = statusStyle.Render(m.headerStatusSegment())
	return brand, profile, status
}

type headerStatusMode int

const (
	headerUnknown headerStatusMode = iota
	headerReady
	headerUrgent
)

func (m Model) headerStatusState() headerStatusMode {
	if !m.profilesLoaded || len(m.profiles) == 0 {
		return headerUrgent
	}
	return headerReady
}

func (m Model) headerProfileSegment() string {
	if !m.profilesLoaded {
		return "PERFIL: NO EVALUADO"
	}
	if len(m.profiles) == 0 {
		return "PERFIL: NINGUNO"
	}
	return "PERFIL: SIN SELECCIONAR"
}

func (m Model) headerStatusSegment() string {
	if !m.profilesLoaded {
		return "ESTADO: NO EVALUADO"
	}
	if len(m.profiles) == 0 {
		return "ESTADO: REQUIERE CONFIGURACIÓN"
	}
	return "ESTADO: PENDIENTE DE VERIFICACIÓN"
}

func (m Model) renderBrand(width int, t homeTheme) string {
	_, art := m.heroLogo(width)
	var brandRows []string
	if art != "" {
		brandRows = append(brandRows, t.logo.Render(art))
		brandRows = append(brandRows, "")
	}
	brandRows = append(brandRows, t.identity.Render("BAC NEXUS"))
	brandRows = append(brandRows, "")
	brandRows = append(brandRows, t.tagline.Render("Contexto IBM i seguro para desarrolladores y agentes de IA"))
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, lipgloss.JoinVertical(lipgloss.Center, brandRows...))
}

func (m Model) renderFooter(width int, t homeTheme) string {
	text := "↑/↓ navegar  •  Enter seleccionar  •  ? ayuda  •  q salir"
	return renderFooterText(width, t, text, m.buildInfo)
}

// renderFooterText keeps every shell screen on the same centered command-bar
// contract while hiding version metadata before it could collide with commands.
func renderFooterText(width int, t homeTheme, text string, buildInfo BuildInfo) string {
	if lipgloss.Width(text) > width {
		return t.footer.Width(width).Align(lipgloss.Left).Render(fitHomeLine(text, width))
	}
	commandWidth := lipgloss.Width(text)
	commandStart := max((width-commandWidth)/2, 0)
	version := "BAC NEXUS " + buildInfo.Version
	const footerVersionMargin = 2
	if footerVersionMargin+lipgloss.Width(version) > commandStart {
		return t.footer.Width(width).Align(lipgloss.Center).Render(text)
	}
	return strings.Repeat(" ", footerVersionMargin) +
		t.footer.Render(version) +
		strings.Repeat(" ", commandStart-footerVersionMargin-lipgloss.Width(version)) +
		t.footer.Render(text) +
		strings.Repeat(" ", max(width-commandStart-commandWidth, 0))
}

// renderFeedback keeps contextual feedback semantically styled wherever the
// shared shell is rendered. NO_COLOR retains its textual markers unchanged.
func (m Model) renderFeedback(width int, t homeTheme) string {
	var lines []string
	if m.status != "" {
		lines = append(lines, t.statusNeutral.Render(fitHomeLine("[--] "+m.status, width)))
	}
	if m.err != nil {
		lines = append(lines, t.statusError.Render(fitHomeLine("[ERR] "+sanitizeError(m.err), width)))
	}
	return strings.Join(lines, "\n")
}

func (m Model) readinessSummary() string {
	profile := "[--] Perfil IBM i: no evaluado"
	if m.profilesLoaded {
		switch {
		case len(m.profiles) == 0:
			profile = "[WARN] Perfil IBM i: ninguno configurado"
		default:
			profile = fmt.Sprintf("[OK] Perfil IBM i: %d disponibles", len(m.profiles))
		}
	}
	return profile + "\n[--] Preparación local: no evaluada"
}

func (m Model) renderStatusRow(row string, width int, indicator string, t homeTheme) string {
	marker, content, found := strings.Cut(row, " ")
	if !found {
		marker, content = row, ""
	}
	style := t.statusNeutral
	switch marker {
	case "[OK]":
		style = t.statusOK
	case "[INFO]":
		style = t.statusInfo
	case "[WARN]":
		style = t.statusWarning
	case "[ERR]":
		style = t.statusError
	case "[....]":
		style = t.statusProgress
	case "[--]":
		style = t.statusNeutral
	}
	plain := marker
	if content != "" {
		plain += " " + content
	}
	if indicator != "" && lipgloss.Width(plain) < width {
		plain += strings.Repeat(" ", width-lipgloss.Width(plain)-1) + indicator
	}
	if content == "" {
		return style.Render(plain)
	}
	return style.Render(marker) + " " + t.fieldsetContent.Render(strings.TrimPrefix(plain, marker+" "))
}

func (m Model) currentMenu() []homeMenuRow {
	if m.homeMenuRows != nil {
		return m.homeMenuRows
	}
	if m.profilesLoaded && len(m.profiles) > 0 {
		return m.populatedHomeMenu()
	}
	return m.emptyHomeMenu()
}

func (m Model) visibleHomeActions() []homeAction {
	if m.homeLayout() == homeLayoutMinimal {
		return m.menuActionsFor(m.minimalMenu())
	}
	return m.menuActionsFor(m.currentMenu())
}

func (m Model) minimalMenu() []homeMenuRow {
	if m.profilesLoaded && len(m.profiles) > 0 {
		return []homeMenuRow{
			{actionManage, "Administrar perfiles", "manage", true},
			{actionCreate, "Crear un perfil", "create", true},
			{actionExit, "Salir", "exit", true},
		}
	}
	return []homeMenuRow{
		{actionCreate, "Crear un perfil", "create", true},
		{actionExit, "Salir", "exit", true},
	}
}

func (m Model) menuActionsFor(rows []homeMenuRow) []homeAction {
	out := make([]homeAction, len(rows))
	for i, row := range rows {
		out[i] = homeAction{id: row.id, label: row.label, available: row.routes}
	}
	return out
}

func (m Model) renderMenu(width int, t homeTheme) string {
	rows := m.visibleMenuRows()
	window := m.visibleMenuWindow()
	var b strings.Builder
	for i, row := range rows[window.Start:window.End] {
		indicator := ""
		if i == 0 && window.Above {
			indicator = "▲"
		}
		if i == window.End-window.Start-1 && window.Below {
			indicator = "▼"
		}
		selected := row.id == m.homeSelected
		var line string
		if selected {
			highlightWidth := selectedHighlightWidth(width)
			content := "  " + t.selectedMarker.Render("▸") + "    " + t.selectedLabel.Render(row.label)
			content = padHomeRow(content, highlightWidth, indicator, t)
			line = t.selectedRow.Width(highlightWidth).Render(content) + strings.Repeat(" ", width-highlightWidth)
		} else {
			content := padHomeRow("       "+row.label, width, indicator, t)
			padded := lipgloss.NewStyle().Width(width).Render(content)
			line = t.menuRow.Render(padded)
		}
		b.WriteString(line)
		if i != window.End-window.Start-1 {
			b.WriteString(strings.Repeat("\n", menuRowGapLines+1))
			continue
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func selectedHighlightWidth(menuWidth int) int {
	return min(menuWidth, selectedHighlightMaxWidth)
}

func padHomeRow(content string, width int, indicator string, t homeTheme) string {
	if indicator == "" || lipgloss.Width(content) >= width {
		return content
	}
	return content + strings.Repeat(" ", width-lipgloss.Width(content)-1) + t.metadata.Render(indicator)
}

func (m Model) visibleMenuWindow() boundedWindow {
	rows := m.visibleMenuRows()
	selected := 0
	for i, row := range rows {
		if row.id == m.homeSelected {
			selected = i
			break
		}
	}
	return windowLines(len(rows), selected, m.menuOffset, menuWindowCapacity)
}

func (m Model) visibleMenuRows() []homeMenuRow {
	if m.homeLayout() == homeLayoutMinimal {
		return m.minimalMenu()
	}
	return m.currentMenu()
}

func (m Model) renderMinimalMenu(width int) string {
	rows := m.currentMenu()
	var b strings.Builder
	for _, row := range rows {
		line := "  " + row.label
		if row.id == m.homeSelected {
			line = "▸ " + row.label
		}
		line = fitHomeLine(line, width)
		if row.id == m.homeSelected {
			t := newHomeTheme(m.noColor)
			line = t.selectedRow.Render(t.selectedMarker.Render("▸") + " " + t.selectedLabel.Render(strings.TrimPrefix(line, "▸ ")))
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func fitHomeLine(text string, width int) string {
	if width <= 0 || lipgloss.Width(text) <= width {
		return text
	}
	const ellipsis = "…"
	var b strings.Builder
	for _, r := range text {
		candidate := b.String() + string(r) + ellipsis
		if lipgloss.Width(candidate) > width {
			break
		}
		b.WriteRune(r)
	}
	return b.String() + ellipsis
}
