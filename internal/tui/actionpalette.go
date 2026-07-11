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
// palette: substring-filtered core + plugin actions, with the selected ID
// exported via Selected() for the caller to write to a tmux environment
// variable.
type PaletteModel struct {
	// Source data
	coreActions       []action.Action
	pluginActions     []action.Action
	cursorSessionID   string
	cursorSessionDesc string

	// UI state
	query     string
	input     textinput.Model
	filtered  []paletteRow
	cursor    int
	scrollTop int
	width     int
	height    int

	// Selection outcome
	selected string
}

// paletteRow is a single visible row: either an action or a separator
// between the core and plugin groups.
type paletteRow struct {
	action    action.Action
	separator bool
}

// shortcutColWidth is the fixed width of the right-hand shortcut column.
// 6 comfortably fits keys like "M-\" and any future two-modifier notation
// (e.g. "C-M-p") without wrapping.
const shortcutColWidth = 6

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
		cursorSessionID:   cursorSessionID,
		cursorSessionDesc: cursorSessionDesc,
		input:             ti,
	}
	m.applyFilter()
	return m
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

// applyFilter rebuilds m.filtered from the current query. Empty query =
// show everything. A separator row is inserted only when both groups
// have matches so it does not appear as a dangling line.
func (m *PaletteModel) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(m.query))
	var core, plugins []paletteRow
	for _, a := range m.coreActions {
		if matches(q, a) {
			core = append(core, paletteRow{action: a})
		}
	}
	for _, a := range m.pluginActions {
		if matches(q, a) {
			plugins = append(plugins, paletteRow{action: a})
		}
	}

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

// matches performs a case-insensitive substring test against Label plus
// Description. Shortcut is intentionally excluded (spec).
// q is expected to be already lowercased.
func matches(q string, a action.Action) bool {
	if q == "" {
		return true
	}
	hay := strings.ToLower(a.Label)
	if a.Description != "" {
		hay += "\x00" + strings.ToLower(a.Description)
	}
	return strings.Contains(hay, q)
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
	if rowWidth < shortcutColWidth+1 {
		rowWidth = shortcutColWidth + 1
	}
	labelWidth := rowWidth - shortcutColWidth - 1 // 1 space gutter

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

			label := truncateString(row.action.Label, labelWidth)
			descSuffix := ""
			if row.action.NeedsSession && m.cursorSessionDesc != "" {
				candidate := " (" + m.cursorSessionDesc + ")"
				avail := labelWidth - runewidth.StringWidth(label)
				if avail > 0 {
					if runewidth.StringWidth(candidate) > avail {
						candidate = truncateString(candidate, avail)
					}
					descSuffix = candidate
				}
			}
			labelDisplayWidth := runewidth.StringWidth(label) + runewidth.StringWidth(descSuffix)
			pad := labelWidth - labelDisplayWidth
			if pad < 0 {
				pad = 0
			}

			shortcut := row.action.Shortcut
			if runewidth.StringWidth(shortcut) > shortcutColWidth {
				shortcut = truncateString(shortcut, shortcutColWidth)
			}
			shortcutPad := shortcutColWidth - runewidth.StringWidth(shortcut)
			if shortcutPad < 0 {
				shortcutPad = 0
			}

			if i == m.cursor {
				// Selected: highlight the whole row (plain text, so
				// the cursorStyle background covers uniformly).
				line := "▸ " + label + descSuffix + strings.Repeat(" ", pad) + " " +
					strings.Repeat(" ", shortcutPad) + shortcut
				b.WriteString(cursorStyle.Render(line))
			} else {
				line := "  " + label +
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
