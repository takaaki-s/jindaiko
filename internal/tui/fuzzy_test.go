package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestFuzzyFilter_EmptyQuery_ReturnsAllInOrder locks in the "empty query =
// pass-through" contract. sahilm/fuzzy.Find returns zero results for an
// empty pattern, so both palette and switch-session picker rely on FuzzyFilter to
// short-circuit and preserve the caller-provided order.
func TestFuzzyFilter_EmptyQuery_ReturnsAllInOrder(t *testing.T) {
	targets := []string{"alpha", "beta", "gamma"}
	got := FuzzyFilter("", targets)
	if len(got) != len(targets) {
		t.Fatalf("len(FuzzyFilter(\"\", ...)) = %d, want %d", len(got), len(targets))
	}
	for i, m := range got {
		if m.Index != i {
			t.Errorf("row %d: Index = %d, want %d", i, m.Index, i)
		}
		if m.MatchedIndexes != nil {
			t.Errorf("row %d: MatchedIndexes = %v, want nil for empty query", i, m.MatchedIndexes)
		}
	}
}

// TestFuzzyFilter_WhitespaceOnly_TreatedAsEmpty documents that a whitespace-
// only query behaves the same as empty (TrimSpace). This matters because
// the textinput.Model can emit " " while the user pauses.
func TestFuzzyFilter_WhitespaceOnly_TreatedAsEmpty(t *testing.T) {
	targets := []string{"alpha", "beta"}
	got := FuzzyFilter("   ", targets)
	if len(got) != len(targets) {
		t.Errorf("whitespace query returned %d rows, want %d (pass-through)", len(got), len(targets))
	}
}

// TestFuzzyFilter_Query_RanksResults regresses that the fuzzy engine is
// engaged for non-empty queries and reports both Index and MatchedIndexes.
func TestFuzzyFilter_Query_RanksResults(t *testing.T) {
	targets := []string{"session_new", "session_kill", "quit"}
	got := FuzzyFilter("kill", targets)
	if len(got) == 0 {
		t.Fatalf("expected at least one match, got zero")
	}
	// "kill" is only substringed inside "session_kill".
	if got[0].Index != 1 {
		t.Errorf("top match Index = %d, want 1 (session_kill)", got[0].Index)
	}
	if got[0].MatchedIndexes == nil {
		t.Errorf("top match MatchedIndexes = nil, want populated")
	}
	if want := len("kill"); len(got[0].MatchedIndexes) != want {
		t.Errorf("len(MatchedIndexes) = %d, want %d (one index per query rune)", len(got[0].MatchedIndexes), want)
	}
}

// TestFuzzyFilter_EmptyTargets_ReturnsEmpty guards the degenerate
// zero-target case (empty registry / all actions filtered upstream).
func TestFuzzyFilter_EmptyTargets_ReturnsEmpty(t *testing.T) {
	if got := FuzzyFilter("query", nil); len(got) != 0 {
		t.Errorf("FuzzyFilter with nil targets = %v, want empty", got)
	}
	if got := FuzzyFilter("", nil); len(got) != 0 {
		t.Errorf("FuzzyFilter with nil targets + empty query = %v, want empty", got)
	}
}

// TestBuildActionHaystack_JoinsLabelDescription documents the exact
// concatenation shape. A space separates the two segments so sahilm/fuzzy
// can still match into the description (it stops matching across NUL).
func TestBuildActionHaystack_JoinsLabelDescription(t *testing.T) {
	if got, want := BuildActionHaystack("label", "desc"), "label desc"; got != want {
		t.Errorf("BuildActionHaystack(label, desc) = %q, want %q", got, want)
	}
	if got, want := BuildActionHaystack("label", ""), "label"; got != want {
		t.Errorf("BuildActionHaystack(label, \"\") = %q, want %q", got, want)
	}
}

// TestRenderMatchedLine_NoMatches confirms the fast path returns the plain
// target verbatim (no ANSI escapes) when there are no highlight indices.
func TestRenderMatchedLine_NoMatches(t *testing.T) {
	style := lipgloss.NewStyle().Underline(true).Foreground(lipgloss.Color("42"))
	got := RenderMatchedLine([]rune("plain"), nil, 20, style, false)
	if got != "plain" {
		t.Errorf("RenderMatchedLine no-highlight = %q, want %q", got, "plain")
	}
	if strings.Contains(got, "\x1b[") {
		t.Errorf("RenderMatchedLine no-highlight produced ANSI escape: %q", got)
	}
}

// TestRenderMatchedLine_TruncatesWithEllipsis documents the "..." tail
// behaviour when the target overflows maxWidth. Highlights past the
// truncation boundary are dropped silently.
func TestRenderMatchedLine_TruncatesWithEllipsis(t *testing.T) {
	style := lipgloss.NewStyle().Underline(true).Foreground(lipgloss.Color("42"))
	got := RenderMatchedLine([]rune("abcdefghij"), []int{0, 9}, 6, style, false)
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected trailing ellipsis, got %q", got)
	}
	// The last-rune highlight ('j', index 9) should be dropped because it
	// falls past the truncation boundary (visible = "abc").
	// Verify by ensuring the output contains "..." literally at the end.
}

// TestRenderMatchedLine_ContiguousRunPreservesText verifies that a run of
// N contiguous highlighted runes emits every character in order (i.e.
// coalescing runs into a single style.Render call — the code's optimization
// — must not drop any character from the visible output).
func TestRenderMatchedLine_ContiguousRunPreservesText(t *testing.T) {
	style := lipgloss.NewStyle().Underline(true).Foreground(lipgloss.Color("42"))
	got := RenderMatchedLine([]rune("abcd"), []int{0, 1, 2, 3}, 20, style, false)
	// Every character must still appear (styled) in the output.
	for _, r := range "abcd" {
		if !strings.ContainsRune(got, r) {
			t.Errorf("styled output missing rune %q: %q", r, got)
		}
	}
}

// TestRenderMatchedLine_FragmentedHits verifies that non-contiguous hits
// still produce highlighted output for each hit rune with plain runes in
// between (a mixed styled/plain sequence).
func TestRenderMatchedLine_FragmentedHits(t *testing.T) {
	style := lipgloss.NewStyle().Underline(true).Foreground(lipgloss.Color("42"))
	got := RenderMatchedLine([]rune("a-b-c"), []int{0, 2, 4}, 20, style, false)
	// Plain runes between hits ("-") must survive unstyled but present.
	if !strings.Contains(got, "-") {
		t.Errorf("plain rune '-' missing between hits: %q", got)
	}
	// At least one ANSI escape sequence must be present.
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("no ANSI escape emitted for fragmented hits: %q", got)
	}
}

// TestFuzzyMatch_ShapeIsStable pins down the returned struct fields so
// downstream callers (palette / switch-session picker) can rely on the shape.
func TestFuzzyMatch_ShapeIsStable(t *testing.T) {
	m := FuzzyMatch{Index: 3, MatchedIndexes: []int{0, 2, 4}}
	if m.Index != 3 || !reflect.DeepEqual(m.MatchedIndexes, []int{0, 2, 4}) {
		t.Fatalf("FuzzyMatch shape drifted: %+v", m)
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
// underline on the selected row" invariant: the palette caller wraps the
// selected row in a cursor background, so an underline foreground on top
// would clash. RenderMatchedLine must therefore return plain text when
// selected=true.
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

// TestRenderMatchedSegment_NoMatches_AppliesBaseStyle covers the fast path
// (no hits in the window): the whole segment must render under `base`, so
// callers can rely on per-piece color even without highlights.
func TestRenderMatchedSegment_NoMatches_AppliesBaseStyle(t *testing.T) {
	base := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	got := RenderMatchedSegment([]rune("plain"), nil, 0, 20, base)
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("no ANSI escape emitted for base-styled plain segment: %q", got)
	}
	if !strings.Contains(got, "plain") {
		t.Errorf("segment text missing from output: %q", got)
	}
}

// TestRenderMatchedSegment_HighlightsWithinWindow verifies the matchOffset
// filter: only haystack indexes falling inside [matchOffset, matchOffset+
// len(target)) participate, rebased to segment-local positions.
func TestRenderMatchedSegment_HighlightsWithinWindow(t *testing.T) {
	base := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	// Say the haystack is "ab cd ef" and matched indexes are {0,3,6}. The
	// "cd" segment starts at haystack offset 3, length 2 → local hit at
	// position 0 only.
	got := RenderMatchedSegment([]rune("cd"), []int{0, 3, 6}, 3, 20, base)
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("expected an ANSI escape for the in-window hit: %q", got)
	}
	if !strings.ContainsRune(got, 'c') || !strings.ContainsRune(got, 'd') {
		t.Errorf("segment characters missing: %q", got)
	}
}

// TestRenderMatchedSegment_OutOfWindowHitsIgnored ensures that hits with a
// non-empty matched slice but none inside the segment window fall through
// to the fast (no-highlight) path — same visual as the no-match case.
func TestRenderMatchedSegment_OutOfWindowHitsIgnored(t *testing.T) {
	base := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	fast := RenderMatchedSegment([]rune("cd"), nil, 3, 20, base)
	skipped := RenderMatchedSegment([]rune("cd"), []int{0, 6}, 3, 20, base)
	if fast != skipped {
		t.Errorf("out-of-window hits altered output:\n plain path = %q\n hits path  = %q", fast, skipped)
	}
}

// TestRenderMatchedSegment_TruncatesWithEllipsis mirrors RenderMatchedLine's
// truncation contract: overflow past maxWidth gets a "..." tail, still under
// the base style.
func TestRenderMatchedSegment_TruncatesWithEllipsis(t *testing.T) {
	base := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	got := RenderMatchedSegment([]rune("abcdefghij"), nil, 0, 6, base)
	if !strings.HasSuffix(stripANSI(got), "...") {
		t.Errorf("expected trailing ellipsis, got %q", got)
	}
}

// stripANSI removes CSI escape sequences so string assertions can inspect
// only the visible characters. Kept local because it is only useful in
// tests that mix content and styling.
func stripANSI(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			// Skip until the terminator byte (final byte in CSI is @..~).
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			if j < len(s) {
				j++
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
