package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/takaaki-s/jind-ai/internal/session"
)

// SessionFilterModel is the tmux-popup Bubble Tea model that renders the
// switch-session picker: fuzzy-filtered session list, with the selected ID
// exported via Selected() for the caller to write to JIN_FOCUS_SESSION.
//
// The "SessionFilter" name is kept for backward compatibility with the
// `popups.session_filter` config key and the `session-filter-popup` cobra
// subcommand — see the user-facing name "switch session" in README and
// helpview for what this popup is called in the UI.
type SessionFilterModel struct {
	// Source data
	sessions []session.Info
	targets  []string // len == len(sessions), same index

	// UI state
	query     string
	input     textinput.Model
	matches   []filterRow
	cursor    int
	scrollTop int
	width     int
	height    int

	// Selection outcome
	selected string
}

// filterRow is one visible row in the picker.
type filterRow struct {
	sess           session.Info
	target         string // pre-built haystack (buildTarget result), rendered as the row label
	targetRunes    []rune // target decoded once at build time so View() is not repeat-decoding per frame
	matchedIndexes []int  // sahilm/fuzzy Match.MatchedIndexes; nil for empty query. Sorted ascending.
}

// NewSessionFilterModel constructs a SessionFilterModel with the given
// sessions. The caller is expected to pass sessions in the daemon-sorted
// order (session.SortInfos); we preserve that order for empty queries.
func NewSessionFilterModel(sessions []session.Info) SessionFilterModel {
	ti := textinput.New()
	ti.Placeholder = "type to filter..."
	ti.Prompt = "> "
	ti.CharLimit = 128
	ti.Width = 40
	ti.Focus()

	targets := make([]string, len(sessions))
	for i, s := range sessions {
		targets[i] = buildTarget(s)
	}

	m := SessionFilterModel{
		sessions: sessions,
		targets:  targets,
		input:    ti,
	}
	m.applyFilter()
	return m
}

// buildTarget concatenates the fields used for fuzzy matching into a single
// haystack per session. Field order is load-bearing: highlight indexes point
// into this exact string, and the View() draws the same string, so the two
// must agree byte-for-byte. AgentKind is included so multi-adapter setups
// can filter by "codex" / "claude".
func buildTarget(s session.Info) string {
	parts := []string{
		s.Description,
		s.WorkDir,
		s.CurrentWorkDir,
		s.CurrentBranch,
		s.Fleet,
		s.AgentKind,
	}
	return strings.Join(parts, " ")
}

// Selected returns the ID of the session the user picked, or "" if the
// picker was dismissed.
func (m SessionFilterModel) Selected() string {
	return m.selected
}

func (m SessionFilterModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m SessionFilterModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if m.cursor >= 0 && m.cursor < len(m.matches) {
				m.selected = m.matches[m.cursor].sess.ID
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

// applyFilter rebuilds m.matches from the current query via FuzzyFilter,
// which short-circuits empty queries to the caller-provided (daemon) order
// and delegates non-empty queries to sahilm/fuzzy.
func (m *SessionFilterModel) applyFilter() {
	m.matches = m.matches[:0]
	for _, mt := range FuzzyFilter(m.query, m.targets) {
		t := m.targets[mt.Index]
		m.matches = append(m.matches, filterRow{
			sess:           m.sessions[mt.Index],
			target:         t,
			targetRunes:    []rune(t),
			matchedIndexes: mt.MatchedIndexes,
		})
	}
	if m.cursor < 0 || m.cursor >= len(m.matches) {
		m.cursor = 0
	}
	m.clampScroll()
}

func (m *SessionFilterModel) moveCursor(dir int) {
	if len(m.matches) == 0 {
		m.cursor = 0
		return
	}
	next := m.cursor + dir
	if next < 0 {
		next = 0
	}
	if next >= len(m.matches) {
		next = len(m.matches) - 1
	}
	m.cursor = next
	m.adjustScroll()
}

func (m *SessionFilterModel) adjustScroll() {
	lines := m.visibleLines()
	if m.cursor < m.scrollTop {
		m.scrollTop = m.cursor
	} else if m.cursor >= m.scrollTop+lines {
		m.scrollTop = m.cursor - lines + 1
	}
	m.clampScroll()
}

func (m *SessionFilterModel) clampScroll() {
	max := len(m.matches) - m.visibleLines()
	if max < 0 {
		max = 0
	}
	if m.scrollTop < 0 {
		m.scrollTop = 0
	}
	if m.scrollTop > max {
		m.scrollTop = max
	}
}

// visibleLines mirrors PaletteModel: title (1) + blank (1) + input (1) +
// help (1) + tail blank (1) = 5 reserved rows.
func (m SessionFilterModel) visibleLines() int {
	lines := m.height - 5
	if lines < 1 {
		lines = 1
	}
	return lines
}

func (m SessionFilterModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	cursorStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Background(primaryColor)
	matchStyle := lipgloss.NewStyle().Underline(true).Foreground(primaryColor)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Switch Session"))
	b.WriteString("\n\n")

	if len(m.sessions) == 0 {
		b.WriteString(helpStyle.Render("  No sessions"))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("Esc close"))
		return b.String()
	}

	totalWidth := m.width
	if totalWidth <= 0 {
		totalWidth = 60
	}
	rowWidth := totalWidth - 2
	if rowWidth < 1 {
		rowWidth = 1
	}

	if len(m.matches) == 0 {
		b.WriteString(helpStyle.Render("  No matching sessions"))
		b.WriteString("\n")
	} else {
		lines := m.visibleLines()
		end := m.scrollTop + lines
		if end > len(m.matches) {
			end = len(m.matches)
		}
		for i := m.scrollTop; i < end; i++ {
			row := m.matches[i]
			// sahilm/fuzzy is rune-aware; MatchedIndexes points into runes.
			// Selected rows pass selected=true so the fuzzy underline is
			// skipped (underline foreground would clash with cursorStyle's
			// background); the whole row is then wrapped in cursorStyle so
			// the highlight covers uniformly (mirrors PaletteModel.View).
			line := RenderMatchedLine(row.targetRunes, row.matchedIndexes, rowWidth, matchStyle, i == m.cursor)
			if i == m.cursor {
				pad := rowWidth - runewidth.StringWidth(line)
				if pad < 0 {
					pad = 0
				}
				full := "▸ " + line + strings.Repeat(" ", pad)
				b.WriteString(cursorStyle.Render(full))
			} else {
				b.WriteString("  ")
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
