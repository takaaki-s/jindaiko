package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

// FuzzyMatch is one result row from FuzzyFilter. Index is the position in
// the caller-provided targets slice; MatchedIndexes are rune positions in
// the corresponding target that participated in the fuzzy match. For empty
// queries MatchedIndexes is nil so callers can take a fast render path.
type FuzzyMatch struct {
	Index          int
	MatchedIndexes []int
}

// FuzzyFilter runs sahilm/fuzzy.Find over targets and returns index +
// matched-rune positions per hit. An empty query short-circuits to all
// targets in their original order (MatchedIndexes nil) — this preserves
// the "no filter typed" ordering that palette and session filter both
// expect, which sahilm/fuzzy would otherwise report as zero results.
func FuzzyFilter(query string, targets []string) []FuzzyMatch {
	q := strings.TrimSpace(query)
	if q == "" {
		out := make([]FuzzyMatch, len(targets))
		for i := range targets {
			out[i] = FuzzyMatch{Index: i}
		}
		return out
	}
	hits := fuzzy.Find(q, targets)
	out := make([]FuzzyMatch, len(hits))
	for i, h := range hits {
		out[i] = FuzzyMatch{Index: h.Index, MatchedIndexes: h.MatchedIndexes}
	}
	return out
}

// BuildActionHaystack joins a palette action's Label and Description into
// the single haystack that FuzzyFilter matches against. A space separates
// the two so both segments remain searchable — sahilm/fuzzy stops matching
// across NUL bytes, which would otherwise drop description hits entirely
// under the fuzzy engine (a substring engine wouldn't care). Empty
// Description keeps just the label.
func BuildActionHaystack(label, description string) string {
	if description == "" {
		return label
	}
	return label + " " + description
}

// RenderMatchedLine renders target runes with matchedIndexes highlighted,
// truncating with an ellipsis if the target exceeds maxWidth. Highlights
// past the truncation boundary are dropped silently; we do not attempt to
// scroll the target horizontally.
//
// matched is expected to be sorted ascending (sahilm/fuzzy guarantees this)
// so we walk it with a two-pointer cursor instead of building a lookup map,
// and coalesce runs of adjacent hits into a single style.Render call so a
// contiguous match of N runes emits one SGR pair instead of N.
func RenderMatchedLine(target []rune, matched []int, maxWidth int, style lipgloss.Style, selected bool) string {
	if maxWidth <= 0 {
		return ""
	}
	// Truncate rune-wise. maxWidth here is a display width but callers
	// (palette label column, session filter row) pass mostly-ASCII targets;
	// for the popup a rune-count fallback is close enough and keeps the
	// highlight index math simple. If East-Asian descriptions become
	// common we can revisit.
	visible := target
	truncated := false
	if len(visible) > maxWidth {
		if maxWidth > 3 {
			visible = visible[:maxWidth-3]
			truncated = true
		} else {
			visible = visible[:maxWidth]
		}
	}

	// Fast path: no highlights, or selected rows (which suppress the fuzzy
	// underline to avoid clashing with the cursorStyle background).
	if len(matched) == 0 || selected {
		if truncated {
			return string(visible) + "..."
		}
		return string(visible)
	}

	var b strings.Builder
	j := 0 // cursor into matched
	i := 0
	for i < len(visible) {
		// Advance j past any hits that fell outside visible.
		for j < len(matched) && matched[j] < i {
			j++
		}
		if j < len(matched) && matched[j] == i {
			// Collect one contiguous run of hits.
			runStart := i
			for j < len(matched) && matched[j] == i && i < len(visible) {
				i++
				j++
			}
			b.WriteString(style.Render(string(visible[runStart:i])))
			continue
		}
		// Collect one contiguous plain run up to the next hit (or end).
		plainStart := i
		next := len(visible)
		if j < len(matched) && matched[j] < next {
			next = matched[j]
		}
		if next > len(visible) {
			next = len(visible)
		}
		i = next
		b.WriteString(string(visible[plainStart:i]))
	}
	if truncated {
		b.WriteString("...")
	}
	return b.String()
}
