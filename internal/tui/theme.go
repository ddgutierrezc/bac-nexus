package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	bacRed             = "#E4002B"
	white              = "#FFFFFF"
	textPrimary        = "#F5F5F5"
	mutedMetadata      = "#A1A1AA"
	insetSurface       = "#151517"
	borderSurface      = "#3A3A40"
	terminalBackground = "#09090B"
	salmonPrimary      = "#FFB3AF"
	salmonMuted        = "#E8BCB9"
	secondaryFixedDim  = "#C6C6C7"
	tertiaryFixedDim   = "#C7C5CD"
	errorColor         = "#FFB4AB"
)

// homeTheme centralizes the approved visual tokens for this TUI and later
// screens. Each role is intentionally distinct so the chrome does not
// collapse into uniform white.
type homeTheme struct {
	frame              lipgloss.Style
	header             lipgloss.Style
	headerBrand        lipgloss.Style
	headerProfile      lipgloss.Style
	headerStatus       lipgloss.Style
	headerStatusUrgent lipgloss.Style
	headerSeparator    lipgloss.Style
	fieldset           lipgloss.Style
	fieldsetBorder     lipgloss.Style
	fieldsetTitle      lipgloss.Style
	fieldsetContent    lipgloss.Style
	panel              lipgloss.Style
	panelTitle         lipgloss.Style
	menuHeading        lipgloss.Style
	logo               lipgloss.Style
	identity           lipgloss.Style
	tagline            lipgloss.Style
	selectedRow        lipgloss.Style
	selectedMarker     lipgloss.Style
	selectedLabel      lipgloss.Style
	menuRow            lipgloss.Style
	statusOK           lipgloss.Style
	statusInfo         lipgloss.Style
	statusWarning      lipgloss.Style
	statusError        lipgloss.Style
	statusNeutral      lipgloss.Style
	statusProgress     lipgloss.Style
	footer             lipgloss.Style
	metadata           lipgloss.Style
}

func newHomeTheme(noColor bool) homeTheme {
	t := homeTheme{
		frame:           lipgloss.NewStyle().Border(lipgloss.NormalBorder()),
		panel:           lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1),
		panelTitle:      lipgloss.NewStyle().Bold(true),
		menuHeading:     lipgloss.NewStyle().Bold(true),
		identity:        lipgloss.NewStyle().Bold(true),
		fieldsetTitle:   lipgloss.NewStyle(),
		fieldsetContent: lipgloss.NewStyle(),
		selectedRow:     lipgloss.NewStyle(),
		selectedMarker:  lipgloss.NewStyle(),
		selectedLabel:   lipgloss.NewStyle(),
		menuRow:         lipgloss.NewStyle(),
		statusOK:        lipgloss.NewStyle(),
		statusInfo:      lipgloss.NewStyle(),
		statusWarning:   lipgloss.NewStyle(),
		statusError:     lipgloss.NewStyle(),
		statusNeutral:   lipgloss.NewStyle(),
		statusProgress:  lipgloss.NewStyle(),
	}
	if noColor {
		return t
	}
	t.frame = t.frame.BorderForeground(lipgloss.Color(borderSurface)).Background(lipgloss.Color(terminalBackground)).Foreground(lipgloss.Color(textPrimary))
	t.header = lipgloss.NewStyle().Bold(true)
	t.headerBrand = lipgloss.NewStyle().Foreground(lipgloss.Color(salmonPrimary)).Bold(true)
	t.headerProfile = lipgloss.NewStyle().Foreground(lipgloss.Color(textPrimary)).Bold(true)
	t.headerStatus = lipgloss.NewStyle().Foreground(lipgloss.Color(salmonMuted)).Bold(true)
	t.headerStatusUrgent = lipgloss.NewStyle().Foreground(lipgloss.Color(bacRed)).Bold(true)
	t.headerSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color(bacRed))
	t.fieldsetBorder = lipgloss.NewStyle().Foreground(lipgloss.Color(borderSurface))
	t.fieldsetTitle = lipgloss.NewStyle().Foreground(lipgloss.Color(salmonMuted)).Bold(true)
	t.fieldsetContent = lipgloss.NewStyle().Foreground(lipgloss.Color(textPrimary))
	t.panel = t.panel.BorderForeground(lipgloss.Color(borderSurface)).Foreground(lipgloss.Color(textPrimary))
	t.panelTitle = t.panelTitle.Foreground(lipgloss.Color(salmonMuted))
	t.menuHeading = t.menuHeading.Foreground(lipgloss.Color(salmonMuted))
	t.logo = lipgloss.NewStyle().Foreground(lipgloss.Color(bacRed))
	t.identity = t.identity.Foreground(lipgloss.Color(salmonPrimary))
	t.tagline = lipgloss.NewStyle().Foreground(lipgloss.Color(textPrimary))
	t.selectedRow = lipgloss.NewStyle().Background(lipgloss.Color(insetSurface)).Bold(true)
	t.selectedMarker = lipgloss.NewStyle().Foreground(lipgloss.Color(bacRed)).Bold(true)
	t.selectedLabel = lipgloss.NewStyle().Foreground(lipgloss.Color(textPrimary)).Bold(true)
	t.statusOK = lipgloss.NewStyle().Foreground(lipgloss.Color(tertiaryFixedDim))
	t.statusInfo = lipgloss.NewStyle().Foreground(lipgloss.Color(secondaryFixedDim))
	t.statusWarning = lipgloss.NewStyle().Foreground(lipgloss.Color(salmonMuted))
	t.statusError = lipgloss.NewStyle().Foreground(lipgloss.Color(errorColor))
	t.statusNeutral = lipgloss.NewStyle().Foreground(lipgloss.Color(mutedMetadata))
	t.statusProgress = lipgloss.NewStyle().Foreground(lipgloss.Color(salmonPrimary))
	t.footer = lipgloss.NewStyle().Foreground(lipgloss.Color(mutedMetadata))
	t.metadata = lipgloss.NewStyle().Foreground(lipgloss.Color(mutedMetadata))
	return t
}

// shellLayout builds a vertically aligned layout anchored to the inner shell
// width. Slots are joined top-aligned so headers, body, and footer keep
// predictable positions regardless of content height.
func (t homeTheme) shellLayout(width int) *shellLayout {
	return &shellLayout{theme: t, width: width}
}

type shellSlot struct {
	kind   string
	height int
	text   string
}

const (
	slotBody    = "body"
	slotFooter  = "footer"
	slotGap     = "gap"
	slotStretch = "stretch"
)

type shellLayout struct {
	theme homeTheme
	width int
	slots []shellSlot
}

func (l *shellLayout) Add(slot string) {
	l.slots = append(l.slots, shellSlot{kind: slotBody, text: slot})
}
func (l *shellLayout) AddGap(lines int) {
	l.slots = append(l.slots, shellSlot{kind: slotGap, height: lines})
}
func (l *shellLayout) AddStretch() {
	l.slots = append(l.slots, shellSlot{kind: slotStretch})
}
func (l *shellLayout) AddFooter(footer string) {
	l.slots = append(l.slots, shellSlot{kind: slotFooter, text: footer})
}

// Render assembles the shell with a top-aligned body, fixed gaps around the
// hero, a flexible stretch slot that absorbs remaining vertical space, and
// the footer anchored to the bottom. Only the explicit stretch slot may
// grow or shrink; fixed gaps always retain their declared height so the
// hero and body never drift vertically.
func (l *shellLayout) Render(height int) string {
	if height < 1 {
		height = 1
	}
	var (
		footerText string
		footerH    int
		bodySlots  []shellSlot
	)
	for _, s := range l.slots {
		switch s.kind {
		case slotFooter:
			footerText = s.text
			footerH = lipgloss.Height(s.text)
		default:
			bodySlots = append(bodySlots, s)
		}
	}
	if footerH > height {
		bodySlots = nil
	}
	used := footerH
	for _, s := range bodySlots {
		switch s.kind {
		case slotBody:
			used += lipgloss.Height(s.text)
		case slotGap, slotStretch:
			used += s.height
		}
	}
	extra := 0
	if used < height {
		extra = height - used
	}
	parts := make([]string, 0, len(bodySlots)+2)
	for _, s := range bodySlots {
		switch s.kind {
		case slotBody:
			parts = append(parts, s.text)
		case slotGap:
			parts = append(parts, strings.Repeat("\n", s.height))
		case slotStretch:
			lines := s.height + extra
			if lines < 0 {
				lines = 0
			}
			parts = append(parts, strings.Repeat("\n", lines))
			extra = 0
		}
	}
	if extra > 0 {
		parts = append(parts, strings.Repeat("\n", extra))
	}
	if footerText != "" {
		parts = append(parts, footerText)
	}
	return strings.Join(parts, "\n")
}
