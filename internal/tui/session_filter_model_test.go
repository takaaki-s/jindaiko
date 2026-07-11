package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/takaaki-s/jind-ai/internal/session"
)

// sampleSessions returns a small fixture used by most tests. Ordering
// matches the daemon-provided (session.SortInfos) order that
// NewSessionFilterModel expects.
func sampleSessions() []session.Info {
	return []session.Info{
		{ID: "s1", Description: "feat/oauth-provider", WorkDir: "/repo/api", Fleet: "backend", AgentKind: "claude"},
		{ID: "s2", Description: "authentication-provider refactor", WorkDir: "/repo/web", Fleet: "frontend", AgentKind: "codex"},
		{ID: "s3", Description: "docs cleanup", WorkDir: "/repo/docs", Fleet: "misc", AgentKind: "claude"},
	}
}

func typeQuery(t *testing.T, m SessionFilterModel, s string) SessionFilterModel {
	t.Helper()
	for _, r := range s {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(SessionFilterModel)
	}
	return m
}

func matchIDs(rows []filterRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.sess.ID)
	}
	return out
}

func TestNewSessionFilterModel_EmptyQuery(t *testing.T) {
	m := NewSessionFilterModel(sampleSessions())

	if got, want := len(m.matches), 3; got != want {
		t.Fatalf("matches len = %d, want %d", got, want)
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
	ids := matchIDs(m.matches)
	wantIDs := []string{"s1", "s2", "s3"}
	for i, id := range wantIDs {
		if ids[i] != id {
			t.Errorf("matches[%d].sess.ID = %q, want %q", i, ids[i], id)
		}
	}
}

func TestApplyFilter_FuzzyRanksBySahilm(t *testing.T) {
	m := NewSessionFilterModel(sampleSessions())
	m = typeQuery(t, m, "feat")

	if len(m.matches) == 0 {
		t.Fatalf("expected at least one fuzzy match for 'feat', got zero")
	}
	// The Description "feat/oauth-provider" is an exact contiguous prefix
	// match, which sahilm/fuzzy ranks above "authentication-provider" where
	// f-e-a-t only appears as a spread-out subsequence (or not at all).
	if got := m.matches[0].sess.ID; got != "s1" {
		ids := matchIDs(m.matches)
		t.Errorf("top match = %q (order=%v), want s1 (feat/oauth-provider)", got, ids)
	}
}

func TestApplyFilter_EmptyQueryPreservesOrder(t *testing.T) {
	sessions := sampleSessions()
	m := NewSessionFilterModel(sessions)

	ids := matchIDs(m.matches)
	for i, s := range sessions {
		if ids[i] != s.ID {
			t.Errorf("empty-query order at [%d] = %q, want %q (matches must mirror daemon order)", i, ids[i], s.ID)
		}
	}
}

func TestUpdate_Enter_SetsSelectedID(t *testing.T) {
	m := NewSessionFilterModel(sampleSessions())
	m.cursor = 1

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(SessionFilterModel)
	if got.Selected() != "s2" {
		t.Errorf("Selected() = %q, want %q", got.Selected(), "s2")
	}
}

func TestUpdate_Esc_LeavesSelectedEmpty(t *testing.T) {
	m := NewSessionFilterModel(sampleSessions())
	m.cursor = 1

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := next.(SessionFilterModel)
	if got.Selected() != "" {
		t.Errorf("Selected() after Esc = %q, want empty", got.Selected())
	}
}

func TestUpdate_UpDownNav(t *testing.T) {
	base := NewSessionFilterModel(sampleSessions())

	// Down from top.
	next, _ := base.Update(tea.KeyMsg{Type: tea.KeyDown})
	if got := next.(SessionFilterModel).cursor; got != 1 {
		t.Errorf("after Down: cursor = %d, want 1", got)
	}

	// Ctrl+N (fzf-compat) mirrors Down.
	next2, _ := base.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	if got := next2.(SessionFilterModel).cursor; got != 1 {
		t.Errorf("after Ctrl+N: cursor = %d, want 1", got)
	}

	// Up from top clamps at 0.
	upTop, _ := base.Update(tea.KeyMsg{Type: tea.KeyUp})
	if got := upTop.(SessionFilterModel).cursor; got != 0 {
		t.Errorf("Up from top: cursor = %d, want 0 (clamped)", got)
	}

	// Down from bottom clamps at len-1.
	bottom := base
	bottom.cursor = len(base.matches) - 1
	downBottom, _ := bottom.Update(tea.KeyMsg{Type: tea.KeyDown})
	if got := downBottom.(SessionFilterModel).cursor; got != len(base.matches)-1 {
		t.Errorf("Down from bottom: cursor = %d, want %d (clamped)", got, len(base.matches)-1)
	}

	// Ctrl+P (fzf-compat) mirrors Up.
	mid := base
	mid.cursor = 2
	ctrlP, _ := mid.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	if got := ctrlP.(SessionFilterModel).cursor; got != 1 {
		t.Errorf("after Ctrl+P: cursor = %d, want 1", got)
	}
}

func TestBuildTarget_IncludesAllFields(t *testing.T) {
	s := session.Info{
		Description:    "desc-marker",
		WorkDir:        "/wd-marker",
		CurrentWorkDir: "/cwd-marker",
		CurrentBranch:  "branch-marker",
		Fleet:          "fleet-marker",
		AgentKind:      "agent-marker",
	}
	got := buildTarget(s)
	for _, want := range []string{
		"desc-marker",
		"/wd-marker",
		"/cwd-marker",
		"branch-marker",
		"fleet-marker",
		"agent-marker",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("buildTarget missing %q: %q", want, got)
		}
	}
}

// TestApplyFilter_PopulatesMatchedIndexes regresses the "MatchedIndexes must
// reach the UI layer" contract (02_design §8.2): RenderMatchedLine relies on
// filterRow.matchedIndexes to highlight fuzzy hits, so applyFilter must copy
// sahilm/fuzzy's Match.MatchedIndexes into every row.
func TestApplyFilter_PopulatesMatchedIndexes(t *testing.T) {
	m := NewSessionFilterModel(sampleSessions())
	m = typeQuery(t, m, "feat")

	if len(m.matches) == 0 {
		t.Fatalf("expected at least one match for 'feat', got zero")
	}
	top := m.matches[0]
	if top.matchedIndexes == nil {
		t.Fatalf("top match matchedIndexes = nil, want populated")
	}
	if got, want := len(top.matchedIndexes), len("feat"); got != want {
		t.Errorf("len(matchedIndexes) = %d, want %d (one index per query rune)", got, want)
	}
	targetRunes := []rune(top.target)
	for _, idx := range top.matchedIndexes {
		if idx < 0 || idx >= len(targetRunes) {
			t.Errorf("matchedIndexes contains out-of-range index %d (target rune count %d)", idx, len(targetRunes))
		}
	}
}

// TestApplyFilter_EmptyQuery_NoMatchedIndexes documents the empty-query
// contract: with no query, all sessions pass through with matchedIndexes ==
// nil so RenderMatchedLine takes its fast (no-highlight) path.
func TestApplyFilter_EmptyQuery_NoMatchedIndexes(t *testing.T) {
	m := NewSessionFilterModel(sampleSessions())
	for i, row := range m.matches {
		if row.matchedIndexes != nil {
			t.Errorf("matches[%d].matchedIndexes = %v, want nil for empty query", i, row.matchedIndexes)
		}
	}
}

// TestRenderMatchedLine_IncludesHighlightEscape locks in that populated
// matchedIndexes produce an ANSI-styled string. Combined with the
// tui package's init() setting termenv.TrueColor, this reliably observes
// lipgloss's SGR output. Without it, a regression could silently drop
// fuzzy highlights (target renders as plain text) with no test signal.
func TestRenderMatchedLine_IncludesHighlightEscape(t *testing.T) {
	style := lipgloss.NewStyle().Underline(true).Foreground(lipgloss.Color("42"))
	got := RenderMatchedLine([]rune("feat/oauth"), []int{0, 1, 2, 3}, 20, style, false)

	if !strings.Contains(got, "\x1b[") {
		t.Errorf("RenderMatchedLine output has no ANSI escape (fuzzy highlight missing): %q", got)
	}
}

// TestRenderMatchedLine_SelectedSkipsHighlight regresses the "no fuzzy
// underline on the selected row" invariant (want-4 rationale): the caller
// wraps the selected row in cursorStyle, and the underline foreground would
// clash with cursorStyle's background. RenderMatchedLine must therefore
// return plain text when selected=true.
func TestRenderMatchedLine_SelectedSkipsHighlight(t *testing.T) {
	style := lipgloss.NewStyle().Underline(true).Foreground(lipgloss.Color("42"))
	got := RenderMatchedLine([]rune("feat/oauth"), []int{0, 1, 2, 3}, 20, style, true)

	if strings.Contains(got, "\x1b[") {
		t.Errorf("RenderMatchedLine(selected=true) produced ANSI escape: %q", got)
	}
	if got != "feat/oauth" {
		t.Errorf("RenderMatchedLine(selected=true) = %q, want plain %q", got, "feat/oauth")
	}
}

func TestBuildTarget_MissingFields(t *testing.T) {
	s := session.Info{Description: "only-desc"}

	got := buildTarget(s)
	if !strings.Contains(got, "only-desc") {
		t.Errorf("buildTarget dropped populated field: %q", got)
	}
	// 6 fields joined by " " when 5 are empty → the description surrounded
	// by 5 empty slots means the result starts with "only-desc" and has
	// trailing spaces. Just verify no panic and the populated field survives.
	if got == "" {
		t.Errorf("buildTarget returned empty for a session with a description")
	}
}
