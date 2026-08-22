---
name: Nexus Terminal System
colors:
  surface: "#131315"
  surface-dim: "#131315"
  surface-bright: "#39393b"
  surface-container-lowest: "#0e0e10"
  surface-container-low: "#1c1b1d"
  surface-container: "#201f22"
  surface-container-high: "#2a2a2c"
  surface-container-highest: "#353437"
  on-surface: "#e5e1e4"
  on-surface-variant: "#e8bcb9"
  inverse-surface: "#e5e1e4"
  inverse-on-surface: "#313032"
  outline: "#ae8785"
  outline-variant: "#5e3f3d"
  surface-tint: "#ffb3af"
  primary: "#ffb3af"
  on-primary: "#68000e"
  primary-container: "#e4002b"
  on-primary-container: "#fff6f5"
  inverse-primary: "#bf0022"
  secondary: "#c6c6c7"
  on-secondary: "#2f3131"
  secondary-container: "#454747"
  on-secondary-container: "#b4b5b5"
  tertiary: "#c7c5cd"
  on-tertiary: "#303036"
  tertiary-container: "#727178"
  on-tertiary-container: "#f9f7ff"
  error: "#ffb4ab"
  on-error: "#690005"
  error-container: "#93000a"
  on-error-container: "#ffdad6"
  primary-fixed: "#ffdad7"
  primary-fixed-dim: "#ffb3af"
  on-primary-fixed: "#410005"
  on-primary-fixed-variant: "#930018"
  secondary-fixed: "#e2e2e2"
  secondary-fixed-dim: "#c6c6c7"
  on-secondary-fixed: "#1a1c1c"
  on-secondary-fixed-variant: "#454747"
  tertiary-fixed: "#e4e1e9"
  tertiary-fixed-dim: "#c7c5cd"
  on-tertiary-fixed: "#1b1b21"
  on-tertiary-fixed-variant: "#46464c"
  background: "#131315"
  on-background: "#e5e1e4"
  surface-variant: "#353437"
typography:
  display-ascii:
    fontFamily: JetBrains Mono
    fontSize: 14px
    fontWeight: "700"
    lineHeight: 14px
    letterSpacing: "0"
  headline-lg:
    fontFamily: JetBrains Mono
    fontSize: 20px
    fontWeight: "700"
    lineHeight: 28px
  headline-md:
    fontFamily: JetBrains Mono
    fontSize: 16px
    fontWeight: "700"
    lineHeight: 24px
  body-md:
    fontFamily: JetBrains Mono
    fontSize: 14px
    fontWeight: "400"
    lineHeight: 20px
  body-sm:
    fontFamily: JetBrains Mono
    fontSize: 12px
    fontWeight: "400"
    lineHeight: 16px
  label-caps:
    fontFamily: JetBrains Mono
    fontSize: 12px
    fontWeight: "700"
    lineHeight: 16px
spacing:
  base: 4px
  cell-w: 8px
  cell-h: 20px
  gutter: 16px
  margin: 24px
---

## Brand & Style

The design system is a high-density, technical interface tailored for internal developers at BAC Shared Services. It adopts a **Modern TUI (Terminal User Interface)** aesthetic, prioritizing operational speed, precision, and the reliability of enterprise IBM i environments.

The personality is authoritative and functional. It rejects modern trends like rounded corners, shadows, or transparency in favor of a rigid, grid-based layout that mirrors a professional terminal environment. Visual hierarchy is established through line-drawing characters, ASCII patterns, and strict monochromatic discipline, accented only by the signature brand red.

- **Style:** Technical Brutalism / Modern TUI.
- **Visual Markers:** Double-line borders for primary containers, block cursors, and monospaced alignment.
- **Tone:** Efficient, secure, and developer-centric.

## Colors

The palette is anchored in a deep "Terminal Black" to ensure maximum contrast and reduced eye strain during long engineering sessions.

- **Primary Brand:** BAC Red (#E4002B) is reserved for the BAC lion logo, active navigation focus, and critical "Execute" actions.
- **Surfaces:** Use `#09090B` for the main background and `#151517` for raised panels or inset code blocks.
- **Borders:** Use `#3A3A40` for structural lines to maintain a low-profile hierarchy.
- **Typography:** Primary text uses High-Contrast White (`#F5F5F5`). Secondary metadata or inactive labels use Muted Gray (`#A1A1AA`).
- **Status:** Semantic colors ([OK], [WARN], [ERR]) are used sparingly within square brackets to provide immediate diagnostic feedback without overwhelming the monochromatic theme.

## Typography

The design system exclusively utilizes **JetBrains Mono** to maintain a consistent character grid. Typography does not rely on massive scale shifts; instead, importance is conveyed through **weight, case, and surrounding symbols.**

- **Headers:** Use uppercase or bold weights with a prefix symbol (e.g., `> HEADER`).
- **Mono-Grid:** All text must align to a strict vertical and horizontal rhythm.
- **ASCII Art:** The `display-ascii` role is specifically for the BAC Lion and logo marks, requiring a non-variable character width to prevent distortion.
- **Mobile:** Typography remains the same size on mobile to preserve legibility in dense data views, but line lengths are restricted.

## Layout & Spacing

This design system uses a **Fixed Character Grid** philosophy. Every element's position should be a multiple of the base character width and height.

- **Grid:** A 12-column layout is used for desktop, but internal panels behave like terminal "windows."
- **Borders as Spacing:** Containers are defined by line-drawing characters (┌ ─ ┐).
- **Density:** Information density is high. Use a 4px (base) / 8px (small) / 16px (medium) spacing scale.
- **Responsive Adaption:** On mobile devices, the interface stacks vertically. The ASCII Hero (Lion) is hidden on screens smaller than 768px to prioritize technical data. Panels transition from side-by-side to full-width blocks.

## Elevation & Depth

Depth is achieved through **Tonal Layering** and **Line Art**, never shadows or blurs.

- **Level 0 (Background):** `#09090B` — The base "screen" layer.
- **Level 1 (Surface):** `#151517` — Inset areas for input forms or secondary data panels.
- **Stroke-based Hierarchy:**
  - **Primary Containers:** Double-line borders (`║`, `═`).
  - **Secondary/Groups:** Single-line borders (`│`, `─`).
- **Focus State:** Elements do not "lift." Focus is indicated by a background color fill of the Primary Red or a `▸` (cursor) character placed to the left of the active row.

## Shapes

The design system is strictly **squared-off**.

- **Radius:** All elements (buttons, inputs, panels) have a `0px` border-radius.
- **Symbols:** Use square brackets `[ ]` for buttons and checkboxes, and `( )` for radio buttons to reinforce the TUI aesthetic.
- **Dividers:** Use horizontal repeats of dash characters `- - - -` or solid lines for section breaks.

## Components

### Buttons

Buttons are rendered as text within brackets or solid blocks.

- **Default:** `[ ACTION ]`
- **Active/Focused:** Solid BAC Red background with white text.
- **Ghost:** `< BACK >`

### Selection & Inputs

- **Row Focus:** Indicated by a `▸` character at the start of the line and a subtle background highlight of `#151517`.
- **Input Fields:** Represented as an underscored area or a boxed region: `Username: [__________]`.
- **Checkboxes:** `[X]` for selected, `[ ]` for unselected.

### Progress Indicators

Progress is displayed via block characters:

- `Loading: [████████░░░░] 80%`

### Tables

Tables use simple pipe characters for column separation.

- **Header:** Uppercase text with a single-line separator below.
- **Hover:** The entire row changes to a `#151517` background.

### Navigation Footer

A persistent command bar at the bottom of the viewport:

- `F1 Help  |  F5 Refresh  |  F10 Commit  |  ESC Exit`
- This replaces traditional heavy navigation menus.

### Terminal Hero

The top of the main dashboard features the product name in large block-style text (ASCII) alongside the BAC Lion art and current system metadata (IP, Environment, Version).
