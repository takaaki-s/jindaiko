package tui

import (
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/takaaki-s/jind-ai/internal/session"
)

// SessionFilterModel is the tmux-popup Bubble Tea model that renders the
// switch-session picker: fuzzy-filtered session list rendered as two-line
// cards (Description on top, dim-colored dir / branch / fleet / agent-kind
// below), with the selected ID exported via Selected() for the caller to
// write to JIN_FOCUS_SESSION.
//
// The "SessionFilter" name is kept for backward compatibility with the
// `popups.session_filter` config key and the `session-filter-popup` cobra
// subcommand — see the user-facing name "switch session" in README and
// helpview for what this popup is called in the UI.
type SessionFilterModel struct {
	rows      []filterRow // one card per session, precomputed
	haystacks []string    // rows[i].haystack pulled out once so FuzzyFilter can consume it without per-frame []string alloc

	// UI state
	query     string
	input     textinput.Model
	matches   []int // indexes into rows, ordered by fuzzy score / daemon order
	cursor    int   // match-space index (0..len(matches))
	scrollTop int   // match-space index of top visible card
	width     int
	height    int

	// Selection outcome
	selected string
}

// segmentKind labels which column a rowSegment paints. The view layer
// picks the base color and where in the two-line card the segment lands
// (segName → line 1; everything else → line 2, joined by " · ").
type segmentKind int

const (
	segName segmentKind = iota
	segDir
	segBranch
	segFleet
	segKind
)

// rowSegment is one addressable piece of a card. Its text participates in
// both the fuzzy haystack and the visible column for its kind, so haystack
// and display stay rune-for-rune aligned (sahilm/fuzzy's MatchedIndexes
// points into the joined haystack and the view walks the same segments to
// place highlights back in the right column). runes is precomputed once so
// View() never repeats the UTF-8 decode.
type rowSegment struct {
	text  string
	runes []rune
	kind  segmentKind
}

// filterRow is one card in the picker, precomputed once per session so
// each frame pays only for style application. segOffsets describes where
// each segment starts inside the haystack; combined with per-segment
// len(runes), it lets RenderMatchedSegment split haystack-wide
// matchedIndexes back to the right column at render time.
//
// Invariant maintained by buildRowSegments: when a segName segment exists,
// it is always segments[0]. View relies on this to render the name line
// without scanning.
type filterRow struct {
	sess           session.Info
	segments       []rowSegment
	segOffsets     []int
	matchedIndexes []int
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

	home, _ := os.UserHomeDir() // "" is a valid input to shortenHome (no-op).
	rows := make([]filterRow, len(sessions))
	haystacks := make([]string, len(sessions))
	for i, s := range sessions {
		segs := buildRowSegments(s, home)
		hs, offs := joinSegmentsHaystack(segs)
		rows[i] = filterRow{sess: s, segments: segs, segOffsets: offs}
		haystacks[i] = hs
	}

	m := SessionFilterModel{
		rows:      rows,
		haystacks: haystacks,
		input:     ti,
	}
	m.applyFilter()
	return m
}

// buildRowSegments picks the fields worth surfacing per row and tags each
// with a kind so the view layer can style them. Field order is load-bearing
// on two counts: joinSegmentsHaystack turns the same slice into the fuzzy
// haystack (so per-segment rune offsets stay in sync with what sahilm/fuzzy
// scans), and segName — when present — is always at index 0 so View can
// pick the name line without scanning (see filterRow doc).
//
// CurrentWorkDir wins over WorkDir — the picker cares about "where is this
// session now", not "where did it start". Fleet is suppressed when it is
// the default so unremarkable rows stay tidy. AgentKind is always kept so
// multi-adapter setups can filter by "codex" / "claude".
func buildRowSegments(s session.Info, home string) []rowSegment {
	segs := make([]rowSegment, 0, 5)
	add := func(text string, kind segmentKind) {
		segs = append(segs, rowSegment{text: text, runes: []rune(text), kind: kind})
	}
	if s.Description != "" {
		add(s.Description, segName)
	}
	dir := s.CurrentWorkDir
	if dir == "" {
		dir = s.WorkDir
	}
	if dir != "" {
		add(shortenHome(dir, home), segDir)
	}
	if s.CurrentBranch != "" {
		add(s.CurrentBranch, segBranch)
	}
	if s.Fleet != "" && s.Fleet != session.DefaultFleet {
		add(s.Fleet, segFleet)
	}
	if s.AgentKind != "" {
		add(s.AgentKind, segKind)
	}
	return segs
}

// joinSegmentsHaystack concatenates segment texts with a single-space
// separator and returns per-segment rune offsets so haystack-wide
// MatchedIndexes can be split back to segments in View (segment length
// comes from len(segment.runes) at read time — no parallel []int needed).
func joinSegmentsHaystack(segs []rowSegment) (string, []int) {
	if len(segs) == 0 {
		return "", nil
	}
	offsets := make([]int, len(segs))
	var b strings.Builder
	off := 0
	for i, s := range segs {
		if i > 0 {
			b.WriteString(" ")
			off++
		}
		offsets[i] = off
		b.WriteString(s.text)
		off += utf8.RuneCountInString(s.text)
	}
	return b.String(), offsets
}

// shortenHome collapses the user's home dir prefix to "~". Returns p
// unchanged when home is empty (couldn't resolve at model init) or when
// p doesn't sit under home.
func shortenHome(p, home string) string {
	if home == "" || p == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
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
				m.selected = m.rows[m.matches[m.cursor]].sess.ID
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
// and delegates non-empty queries to sahilm/fuzzy. Only rows that were
// matched last frame get their matchedIndexes cleared, so large session
// lists don't pay an O(rows) walk per keystroke.
func (m *SessionFilterModel) applyFilter() {
	for _, idx := range m.matches {
		m.rows[idx].matchedIndexes = nil
	}
	m.matches = m.matches[:0]
	for _, mt := range FuzzyFilter(m.query, m.haystacks) {
		m.rows[mt.Index].matchedIndexes = mt.MatchedIndexes
		m.matches = append(m.matches, mt.Index)
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
	cards := m.visibleCards()
	if m.cursor < m.scrollTop {
		m.scrollTop = m.cursor
	} else if m.cursor >= m.scrollTop+cards {
		m.scrollTop = m.cursor - cards + 1
	}
	m.clampScroll()
}

func (m *SessionFilterModel) clampScroll() {
	max := len(m.matches) - m.visibleCards()
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

// visibleCards mirrors PaletteModel accounting: title (1) + blank (1) +
// input (1) + help (1) + tail blank (1) = 5 reserved rows. Each card is
// two lines (name + metadata) so the remaining rows are floor-divided.
func (m SessionFilterModel) visibleCards() int {
	lines := m.height - 5
	if lines < 2 {
		return 1
	}
	return lines / 2
}

// Styles used by View. Hoisted to package scope because Bubble Tea calls
// View() on every message/tick — reconstructing them per frame allocates
// eleven Style values for immutable data.
var (
	pickerTitleStyle        = lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	pickerNameStyle         = lipgloss.NewStyle().Bold(true)
	pickerNameSelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	pickerCursorBarStyle    = lipgloss.NewStyle().Foreground(primaryColor)
	pickerDirStyle          = lipgloss.NewStyle().Foreground(secondaryColor)
	pickerBranchStyle       = lipgloss.NewStyle().Foreground(cyanColor)
	pickerKindStyle         = lipgloss.NewStyle().Foreground(purpleColor)
	pickerFleetStyle        = lipgloss.NewStyle().Foreground(dimColor)
	pickerMetaSepStyle      = lipgloss.NewStyle().Foreground(dimColor)

	// metaStyleByKind indexed by segmentKind so View can pick a color
	// without a per-frame switch. segName occupies index 0 but is never
	// consulted through this table (name line is styled separately).
	metaStyleByKind = [...]lipgloss.Style{
		segName:   pickerNameStyle,
		segDir:    pickerDirStyle,
		segBranch: pickerBranchStyle,
		segFleet:  pickerFleetStyle,
		segKind:   pickerKindStyle,
	}
)

func (m SessionFilterModel) View() string {
	var b strings.Builder
	b.WriteString(pickerTitleStyle.Render("Switch Session"))
	b.WriteString("\n\n")

	if len(m.rows) == 0 {
		b.WriteString(helpStyle.Render("  No sessions"))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("Esc close"))
		return b.String()
	}

	totalWidth := m.width
	if totalWidth <= 0 {
		totalWidth = 60
	}
	// Name line indents by 2 (either "▎ " when selected or two spaces);
	// meta line indents by 2 more so metadata nests visually under its name.
	nameRowWidth := totalWidth - 2
	if nameRowWidth < 1 {
		nameRowWidth = 1
	}
	metaRowWidth := totalWidth - 4
	if metaRowWidth < 1 {
		metaRowWidth = 1
	}

	if len(m.matches) == 0 {
		b.WriteString(helpStyle.Render("  No matching sessions"))
		b.WriteString("\n")
	} else {
		// Pre-render the constants used per card so the loop only pays for
		// per-row work.
		sep := pickerMetaSepStyle.Render(" · ")
		bar := pickerCursorBarStyle.Render("▎")
		namePrefixSel := bar + " "
		metaPrefixSel := bar + "   "
		const namePrefixNo = "  "
		const metaPrefixNo = "    "

		cards := m.visibleCards()
		end := m.scrollTop + cards
		if end > len(m.matches) {
			end = len(m.matches)
		}
		for i := m.scrollTop; i < end; i++ {
			row := m.rows[m.matches[i]]
			selected := i == m.cursor

			namePrefix := namePrefixNo
			metaPrefix := metaPrefixNo
			if selected {
				namePrefix = namePrefixSel
				metaPrefix = metaPrefixSel
			}

			// Line 1: segName is always segments[0] when present (invariant
			// maintained by buildRowSegments), so no scan is needed.
			b.WriteString(namePrefix)
			metaStart := 0
			if len(row.segments) > 0 && row.segments[0].kind == segName {
				seg := row.segments[0]
				nameBase := pickerNameStyle
				if selected {
					nameBase = pickerNameSelectedStyle
				}
				b.WriteString(RenderMatchedSegment(seg.runes, row.matchedIndexes, row.segOffsets[0], nameRowWidth, nameBase))
				metaStart = 1
			}
			b.WriteString("\n")

			// Line 2: everything after the name, joined by pre-rendered sep.
			b.WriteString(metaPrefix)
			for si := metaStart; si < len(row.segments); si++ {
				if si > metaStart {
					b.WriteString(sep)
				}
				seg := row.segments[si]
				b.WriteString(RenderMatchedSegment(seg.runes, row.matchedIndexes, row.segOffsets[si], metaRowWidth, metaStyleByKind[seg.kind]))
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
