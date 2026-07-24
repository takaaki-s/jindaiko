package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

// matchIDs returns the session IDs of every currently-visible match, in
// order. matches is a []int index-array into rows, so this hop chases both
// indirections rather than exposing them to every test site.
func matchIDs(m SessionFilterModel) []string {
	out := make([]string, 0, len(m.matches))
	for _, idx := range m.matches {
		out = append(out, m.rows[idx].sess.ID)
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
	ids := matchIDs(m)
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
	if got := m.rows[m.matches[0]].sess.ID; got != "s1" {
		ids := matchIDs(m)
		t.Errorf("top match = %q (order=%v), want s1 (feat/oauth-provider)", got, ids)
	}
}

func TestApplyFilter_EmptyQueryPreservesOrder(t *testing.T) {
	sessions := sampleSessions()
	m := NewSessionFilterModel(sessions)

	ids := matchIDs(m)
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

// TestBuildRowSegments_IncludesExpectedFields locks in the field → segment
// mapping the view layer relies on. CurrentWorkDir wins over WorkDir when
// both are set (see buildRowSegments doc), Default-named fleets are dropped
// to avoid cluttering the picker, and every other populated field lands in
// its own kind.
func TestBuildRowSegments_IncludesExpectedFields(t *testing.T) {
	s := session.Info{
		Description:    "desc-marker",
		WorkDir:        "/wd-marker",
		CurrentWorkDir: "/cwd-marker",
		CurrentBranch:  "branch-marker",
		Fleet:          "fleet-marker",
		AgentKind:      "agent-marker",
	}
	segs := buildRowSegments(s, "")
	wantByKind := map[segmentKind]string{
		segName:   "desc-marker",
		segDir:    "/cwd-marker",
		segBranch: "branch-marker",
		segFleet:  "fleet-marker",
		segKind:   "agent-marker",
	}
	if got, want := len(segs), len(wantByKind); got != want {
		t.Fatalf("buildRowSegments len = %d, want %d (segs=%+v)", got, want, segs)
	}
	for _, seg := range segs {
		want, ok := wantByKind[seg.kind]
		if !ok {
			t.Errorf("unexpected segment kind %d text=%q", seg.kind, seg.text)
			continue
		}
		if seg.text != want {
			t.Errorf("segment kind %d text = %q, want %q", seg.kind, seg.text, want)
		}
	}
	for _, seg := range segs {
		if seg.kind == segDir && seg.text == "/wd-marker" {
			t.Errorf("WorkDir leaked into segDir even though CurrentWorkDir was set: %+v", seg)
		}
	}
}

// TestBuildRowSegments_DropsDefaultFleet documents that a Fleet equal to
// session.DefaultFleet is treated as "no interesting fleet" and omitted
// from the card so common rows stay tidy.
func TestBuildRowSegments_DropsDefaultFleet(t *testing.T) {
	s := session.Info{Description: "x", Fleet: session.DefaultFleet}
	for _, seg := range buildRowSegments(s, "") {
		if seg.kind == segFleet {
			t.Errorf("segFleet survived for default fleet: %+v", seg)
		}
	}
}

// TestBuildRowSegments_ShortensHome collapses the caller's home prefix in
// segDir so absolute paths render as "~/rest/of/path" instead of a
// hard-to-scan absolute path.
func TestBuildRowSegments_ShortensHome(t *testing.T) {
	s := session.Info{Description: "x", CurrentWorkDir: "/home/u/dev/foo"}
	segs := buildRowSegments(s, "/home/u")
	var dir string
	for _, seg := range segs {
		if seg.kind == segDir {
			dir = seg.text
		}
	}
	if dir != "~/dev/foo" {
		t.Errorf("segDir = %q, want %q", dir, "~/dev/foo")
	}
}

// TestApplyFilter_PopulatesMatchedIndexes regresses the "MatchedIndexes
// must reach the UI layer" contract: RenderMatchedSegment relies on
// filterRow.matchedIndexes (haystack-wide) to redistribute highlights to
// the right column, so applyFilter must copy sahilm/fuzzy's Match.
// MatchedIndexes onto every matched row.
func TestApplyFilter_PopulatesMatchedIndexes(t *testing.T) {
	m := NewSessionFilterModel(sampleSessions())
	m = typeQuery(t, m, "feat")

	if len(m.matches) == 0 {
		t.Fatalf("expected at least one match for 'feat', got zero")
	}
	topIdx := m.matches[0]
	top := m.rows[topIdx]
	if top.matchedIndexes == nil {
		t.Fatalf("top match matchedIndexes = nil, want populated")
	}
	if got, want := len(top.matchedIndexes), len("feat"); got != want {
		t.Errorf("len(matchedIndexes) = %d, want %d (one index per query rune)", got, want)
	}
	haystackRunes := []rune(m.haystacks[topIdx])
	for _, idx := range top.matchedIndexes {
		if idx < 0 || idx >= len(haystackRunes) {
			t.Errorf("matchedIndexes contains out-of-range index %d (haystack rune count %d)", idx, len(haystackRunes))
		}
	}
}

// TestApplyFilter_EmptyQuery_NoMatchedIndexes documents the empty-query
// contract: with no query, every visible row passes through with
// matchedIndexes == nil so RenderMatchedSegment takes its fast
// (no-highlight) path.
func TestApplyFilter_EmptyQuery_NoMatchedIndexes(t *testing.T) {
	m := NewSessionFilterModel(sampleSessions())
	for _, idx := range m.matches {
		row := m.rows[idx]
		if row.matchedIndexes != nil {
			t.Errorf("row %q matchedIndexes = %v, want nil for empty query", row.sess.ID, row.matchedIndexes)
		}
	}
}

// TestBuildRowSegments_MissingFields documents that empty fields are
// simply omitted — no empty slots — so joinSegmentsHaystack produces a
// tight haystack (no run of trailing/interstitial spaces that would
// dilute fuzzy scoring) and the view only draws the segments it has.
func TestBuildRowSegments_MissingFields(t *testing.T) {
	s := session.Info{Description: "only-desc"}
	segs := buildRowSegments(s, "")
	if len(segs) != 1 {
		t.Fatalf("segs len = %d, want 1 (only Description populated): %+v", len(segs), segs)
	}
	if segs[0].kind != segName || segs[0].text != "only-desc" {
		t.Errorf("segs[0] = %+v, want {text: only-desc, kind: segName}", segs[0])
	}
	if want, got := "only-desc", string(segs[0].runes); got != want {
		t.Errorf("segs[0].runes decoded = %q, want %q", got, want)
	}
	hs, offs := joinSegmentsHaystack(segs)
	if hs != "only-desc" {
		t.Errorf("haystack = %q, want %q (no interstitial spaces)", hs, "only-desc")
	}
	if len(offs) != 1 || offs[0] != 0 {
		t.Errorf("offsets = %v, want [0]", offs)
	}
}

// TestJoinSegmentsHaystack_Offsets covers the multi-segment case: offsets
// must point at each segment's first rune inside the joined haystack so
// RenderMatchedSegment can slice haystack-wide MatchedIndexes to the right
// column.
func TestJoinSegmentsHaystack_Offsets(t *testing.T) {
	segs := []rowSegment{
		{text: "ab", kind: segName},
		{text: "cd", kind: segDir},
		{text: "e", kind: segKind},
	}
	hs, offs := joinSegmentsHaystack(segs)
	if hs != "ab cd e" {
		t.Errorf("haystack = %q, want %q", hs, "ab cd e")
	}
	wantOffs := []int{0, 3, 6}
	for i := range segs {
		if offs[i] != wantOffs[i] {
			t.Errorf("offs[%d] = %d, want %d", i, offs[i], wantOffs[i])
		}
	}
}

// TestBuildRowSegments_NameSegmentFirst pins the "name is always
// segments[0] when present" invariant that View relies on to render the
// name line without scanning.
func TestBuildRowSegments_NameSegmentFirst(t *testing.T) {
	s := session.Info{
		Description:   "the-name",
		WorkDir:       "/wd",
		CurrentBranch: "main",
		AgentKind:     "claude",
	}
	segs := buildRowSegments(s, "")
	if len(segs) == 0 || segs[0].kind != segName {
		t.Fatalf("segments[0] = %+v, want kind=segName", segs)
	}
}
