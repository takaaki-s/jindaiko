package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/takaaki-s/jind-ai/internal/action"
)

// PaletteModel is the tmux-popup Bubble Tea model that renders the action
// palette: fuzzy-filtered core + plugin actions (via FuzzyFilter over
// label+description haystacks), with the selected ID exported via
// Selected() for the caller to write to a tmux environment variable.
type PaletteModel struct {
	// Source data
	coreActions       []action.Action
	pluginActions     []action.Action
	coreMetas         []actionMeta // parallel to coreActions; precomputed once
	pluginMetas       []actionMeta // parallel to pluginActions; precomputed once
	cursorSessionID   string
	cursorSessionDesc string

	// UI state
	query            string
	input            textinput.Model
	filtered         []paletteRow
	cursor           int
	scrollTop        int
	width            int
	height           int
	shortcutColWidth int

	// Selection outcome
	selected string
}

// actionMeta caches per-action metadata reused across keystrokes. Without
// this the fuzzy path would rebuild the haystack string and re-decode the
// label runes on every applyFilter (i.e. every keypress).
type actionMeta struct {
	haystack          string // BuildActionHaystack(Label, Description)
	labelRunes        []rune // decoded once so View() is not repeat-decoding per frame
	labelDisplayWidth int    // runewidth.StringWidth(Label); used for descSuffix / pad math
}

// paletteRow is a single visible row: either an action or a separator
// between the core and plugin groups.
type paletteRow struct {
	action            action.Action
	separator         bool
	labelRunes        []rune // shared with the source actionMeta
	labelDisplayWidth int    // shared with the source actionMeta
	matchedIndexes    []int  // FuzzyMatch.MatchedIndexes into labelRunes; nil for empty query or separator
}

// Shortcut column width bounds. The lower bound preserves the historical
// visual balance for single-letter shortcuts; the upper bound guards against
// pathological plugin entries eating the label column.
const (
	minShortcutColWidth = 6
	// Upper bound must fit the widest realistic hint FormatKeyHint can
	// emit — currently "Shift+Ctrl+Alt+P" at 16 runes. Bump this in step
	// with any new modifier support so 3-modifier bindings never silently
	// truncate.
	maxShortcutColWidth = 16
)

// NewPaletteModel constructs a PaletteModel with the given actions and
// current-cursor session context.
func NewPaletteModel(core, plugins []action.Action, cursorSessionID, cursorSessionDesc string) PaletteModel {
	ti := textinput.New()
	ti.Placeholder = "type to filter..."
	ti.Prompt = "> "
	ti.CharLimit = 128
	ti.Width = 40
	ti.Focus()

	m := PaletteModel{
		coreActions:       core,
		pluginActions:     plugins,
		coreMetas:         precomputeActionMetas(core),
		pluginMetas:       precomputeActionMetas(plugins),
		cursorSessionID:   cursorSessionID,
		cursorSessionDesc: cursorSessionDesc,
		input:             ti,
		shortcutColWidth:  computeShortcutColWidth(core, plugins),
	}
	m.applyFilter()
	return m
}

// precomputeActionMetas materializes the per-action haystack + label decode
// once at model construction so applyFilter (called on every keystroke)
// does not re-run those computations.
func precomputeActionMetas(actions []action.Action) []actionMeta {
	metas := make([]actionMeta, len(actions))
	for i, a := range actions {
		lr := []rune(a.Label)
		metas[i] = actionMeta{
			haystack:          BuildActionHaystack(a.Label, a.Description),
			labelRunes:        lr,
			labelDisplayWidth: runewidth.StringWidth(a.Label),
		}
	}
	return metas
}

// computeShortcutColWidth returns the max display width of any Shortcut
// across core+plugin actions, clamped to [minShortcutColWidth,
// maxShortcutColWidth].
func computeShortcutColWidth(core, plugins []action.Action) int {
	w := 0
	for _, a := range core {
		if sw := runewidth.StringWidth(a.Shortcut); sw > w {
			w = sw
		}
	}
	for _, a := range plugins {
		if sw := runewidth.StringWidth(a.Shortcut); sw > w {
			w = sw
		}
	}
	if w < minShortcutColWidth {
		w = minShortcutColWidth
	}
	if w > maxShortcutColWidth {
		w = maxShortcutColWidth
	}
	return w
}

// Selected returns the ID of the action the user picked, or "" if the
// palette was dismissed.
func (m PaletteModel) Selected() string {
	return m.selected
}

func (m PaletteModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m PaletteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if r, ok := m.currentRow(); ok && !r.separator {
				m.selected = r.action.ID
			}
			return m, tea.Quit
		case "up", "ctrl+p":
			m.moveCursor(-1)
			return m, nil
		case "down", "ctrl+n":
			m.moveCursor(1)
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}

	var cmd tea.Cmd
	old := m.input.Value()
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != old {
		m.query = m.input.Value()
		m.applyFilter()
	}
	return m, cmd
}

// applyFilter rebuilds m.filtered from the current query via FuzzyFilter
// over Label+Description haystacks. Empty query = show everything in
// original order. A separator row is inserted only when both groups have
// matches so it does not appear as a dangling line.
func (m *PaletteModel) applyFilter() {
	core := fuzzyRows(m.query, m.coreActions, m.coreMetas)
	plugins := fuzzyRows(m.query, m.pluginActions, m.pluginMetas)

	m.filtered = m.filtered[:0]
	m.filtered = append(m.filtered, core...)
	if len(core) > 0 && len(plugins) > 0 {
		m.filtered = append(m.filtered, paletteRow{separator: true})
	}
	m.filtered = append(m.filtered, plugins...)

	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		m.cursor = 0
	}
	// If we happen to land on a separator (only possible when core was
	// filtered to zero mid-typing and cursor lingered), nudge forward.
	if m.cursor < len(m.filtered) && m.filtered[m.cursor].separator {
		m.moveCursor(1)
	}
	m.clampScroll()
}

// fuzzyRows runs FuzzyFilter over a single action group and returns rows in
// score order (fuzzy) or original order (empty query). MatchedIndexes are
// trimmed to the label rune range so description-region hits do not
// misalign highlights in the label column — the description contributes to
// matching but is not rendered as part of the label.
func fuzzyRows(query string, actions []action.Action, metas []actionMeta) []paletteRow {
	if len(actions) == 0 {
		return nil
	}
	targets := make([]string, len(metas))
	for i, meta := range metas {
		targets[i] = meta.haystack
	}
	hits := FuzzyFilter(query, targets)
	rows := make([]paletteRow, 0, len(hits))
	for _, h := range hits {
		meta := metas[h.Index]
		labelLen := len(meta.labelRunes)
		labelHits := make([]int, 0, len(h.MatchedIndexes))
		for _, idx := range h.MatchedIndexes {
			if idx < labelLen {
				labelHits = append(labelHits, idx)
			}
		}
		rows = append(rows, paletteRow{
			action:            actions[h.Index],
			labelRunes:        meta.labelRunes,
			labelDisplayWidth: meta.labelDisplayWidth,
			matchedIndexes:    labelHits,
		})
	}
	return rows
}

// moveCursor advances the cursor in the given direction, skipping
// separator rows. Clamps at both ends of the filtered list.
func (m *PaletteModel) moveCursor(dir int) {
	if len(m.filtered) == 0 {
		m.cursor = 0
		return
	}
	for {
		next := m.cursor + dir
		if next < 0 {
			// Already at top; stay put unless current is a separator.
			if m.filtered[m.cursor].separator {
				m.cursor = 0
			}
			break
		}
		if next >= len(m.filtered) {
			if m.filtered[m.cursor].separator {
				m.cursor = len(m.filtered) - 1
			}
			break
		}
		m.cursor = next
		if !m.filtered[m.cursor].separator {
			break
		}
	}
	m.adjustScroll()
}

func (m *PaletteModel) adjustScroll() {
	lines := m.visibleLines()
	if m.cursor < m.scrollTop {
		m.scrollTop = m.cursor
	} else if m.cursor >= m.scrollTop+lines {
		m.scrollTop = m.cursor - lines + 1
	}
	m.clampScroll()
}

func (m *PaletteModel) clampScroll() {
	if m.scrollTop < 0 {
		m.scrollTop = 0
	}
	max := len(m.filtered) - m.visibleLines()
	if m.scrollTop > max {
		if max < 0 {
			m.scrollTop = 0
		} else {
			m.scrollTop = max
		}
	}
}

// visibleLines reserves 5 lines: title (1) + blank (1) + input (1) +
// help (1) + tail blank (1). Matches the notifyview pattern.
func (m PaletteModel) visibleLines() int {
	lines := m.height - 5
	if lines < 1 {
		lines = 1
	}
	return lines
}

func (m PaletteModel) currentRow() (paletteRow, bool) {
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return paletteRow{}, false
	}
	return m.filtered[m.cursor], true
}

func (m PaletteModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	cursorStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Background(primaryColor)
	dimStyle := lipgloss.NewStyle().Foreground(dimColor)
	matchStyle := lipgloss.NewStyle().Underline(true).Foreground(primaryColor)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Action Palette"))
	b.WriteString("\n\n")

	// Row width: full width minus the "  " / "▸ " cursor prefix.
	// Fall back to a sane default when width has not been set yet
	// (e.g. before the first WindowSizeMsg or in tests).
	totalWidth := m.width
	if totalWidth <= 0 {
		totalWidth = 60
	}
	rowWidth := totalWidth - 2
	if rowWidth < m.shortcutColWidth+1 {
		rowWidth = m.shortcutColWidth + 1
	}
	labelWidth := rowWidth - m.shortcutColWidth - 1 // 1 space gutter

	if len(m.filtered) == 0 {
		b.WriteString(helpStyle.Render("  No matching actions"))
		b.WriteString("\n")
	} else {
		lines := m.visibleLines()
		end := m.scrollTop + lines
		if end > len(m.filtered) {
			end = len(m.filtered)
		}
		for i := m.scrollTop; i < end; i++ {
			row := m.filtered[i]
			if row.separator {
				sep := strings.Repeat("─", rowWidth)
				b.WriteString("  ")
				b.WriteString(dimStyle.Render(sep))
				b.WriteString("\n")
				continue
			}

			// Layout math uses the cached labelDisplayWidth (runewidth-
			// aware, computed once). The styled version with fuzzy
			// highlights is rendered separately below. Both truncate to
			// labelWidth so their display widths agree for ASCII labels.
			labelWidthOnRow := row.labelDisplayWidth
			if labelWidthOnRow > labelWidth {
				labelWidthOnRow = labelWidth
			}
			descSuffix := ""
			if row.action.NeedsSession && m.cursorSessionDesc != "" {
				candidate := " (" + m.cursorSessionDesc + ")"
				avail := labelWidth - labelWidthOnRow
				if avail > 0 {
					if runewidth.StringWidth(candidate) > avail {
						candidate = truncateString(candidate, avail)
					}
					descSuffix = candidate
				}
			}
			pad := labelWidth - (labelWidthOnRow + runewidth.StringWidth(descSuffix))
			if pad < 0 {
				pad = 0
			}

			shortcut := row.action.Shortcut
			if runewidth.StringWidth(shortcut) > m.shortcutColWidth {
				shortcut = truncateString(shortcut, m.shortcutColWidth)
			}
			shortcutPad := m.shortcutColWidth - runewidth.StringWidth(shortcut)
			if shortcutPad < 0 {
				shortcutPad = 0
			}

			styledLabel := RenderMatchedLine(row.labelRunes, row.matchedIndexes, labelWidth, matchStyle, i == m.cursor)

			if i == m.cursor {
				// Selected: highlight the whole row (plain text, so
				// the cursorStyle background covers uniformly).
				line := "▸ " + styledLabel + descSuffix + strings.Repeat(" ", pad) + " " +
					strings.Repeat(" ", shortcutPad) + shortcut
				b.WriteString(cursorStyle.Render(line))
			} else {
				line := "  " + styledLabel +
					dimStyle.Render(descSuffix) +
					strings.Repeat(" ", pad) + " " +
					strings.Repeat(" ", shortcutPad) +
					dimStyle.Render(shortcut)
				b.WriteString(line)
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(m.input.View())
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Enter select  ↑↓ nav  Esc close"))

	return b.String()
}
