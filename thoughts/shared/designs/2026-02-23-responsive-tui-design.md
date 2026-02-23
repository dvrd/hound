---
date: 2026-02-23
topic: "Responsive TUI Layout"
status: validated
---

# Responsive TUI Layout

## Problem Statement

Every view stores `width` and `height` from `tea.WindowSizeMsg` but never uses them. All layouts are hardcoded to ~80 columns. When the terminal is smaller, content gets truncated by lipgloss at the `App.View()` level, making the UI unusable — buttons, inputs, and data disappear.

## Constraints

- No external layout libraries — use lipgloss and built-in Bubble Tea
- Must work at 60 columns minimum (common split-pane width)
- All 24 test packages must pass
- `App.View()` wraps content in `StyleApp` with `Padding(1,2)` + `RoundedBorder()` — this consumes ~6 columns and ~4 rows from the terminal size. Views get `innerWidth = width - 6` and `innerHeight = height - 4`.

## Approach

### 1. Compute Inner Dimensions in App

`App` already stores `width`/`height`. Add computed `innerWidth`/`innerHeight` that account for the StyleApp border+padding. Pass these to views via the synthetic `WindowSizeMsg` (subtract the chrome).

### 2. Resize Child Components on WindowSizeMsg

Every view's `WindowSizeMsg` handler must update child component widths:
- `textinput.Model` — set `Width` to `min(originalWidth, innerWidth - labelPadding)`
- `textarea.Model` — set `Width` and `Height` to fit available space
- Tables — compute column widths proportionally

### 3. Cap Visible Rows to Available Height

Views with scrollable lists (walletlist, walletstatus, tokenlist, history) should compute `maxVisibleRows = innerHeight - headerRows - footerRows` and only render that many items, showing scroll indicators.

### 4. Responsive Status Bars

Long status bars (walletlist ~85 chars, walletstatus ~80 chars) should truncate or wrap based on available width. Use abbreviated key hints on narrow terminals.

## Changes Per View

### App (`app.go`)
- Subtract border+padding from WindowSizeMsg before forwarding to child views
- `innerWidth = max(20, width - 6)`, `innerHeight = max(5, height - 4)`
- Forward `tea.WindowSizeMsg{Width: innerWidth, Height: innerHeight}` to views

### walletlist
- Table columns: proportional based on `m.width`
- Visible rows: `maxRows = m.height - 8` (title + header + footer + padding)
- Status bar: abbreviated if `m.width < 80`
- Scroll indicator: `↑↓ N more` when list exceeds visible rows

### walletstatus
- Table columns: proportional based on `m.width`
- Visible rows: `maxRows = m.height - 10` (title + SOL balance + header + footer)
- Status bar: abbreviated if `m.width < 80`
- Rename input width: `min(30, m.width - 10)`

### send
- Text input widths: `min(original, m.width - 4)`
- Review step: truncate address display if narrow

### walletimport
- Textarea width: `min(60, m.width - 4)`
- Text input widths: `min(original, m.width - 4)`
- Mnemonic grid: 2 columns if `m.width < 50`, 3 columns otherwise

### swapview
- Text input widths: `min(original, m.width - 4)`

### history
- Table columns: proportional based on `m.width`
- Visible rows: `maxRows = m.height - 6`

### receive
- No changes needed (content is minimal)

### tokenlist
- Table columns: proportional based on `m.width`
- Visible rows: `maxRows = m.height - 6`

### tokenfetch
- No changes needed (content is minimal)

### tokenadd
- Text input widths: `min(original, m.width - 4)`

### walletdelete
- Text input width: `min(50, m.width - 4)`

## Testing Strategy

- Existing tests continue passing (they don't test View() output widths)
- Manual testing at 60, 80, 120 column widths
- Build verification: `go build ./...`

## Open Questions

None.
