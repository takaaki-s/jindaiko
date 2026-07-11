package agent

import (
	"strings"
	"unicode/utf8"
)

// SmartTruncate keeps the first line of s and shortens it to at most maxBytes
// bytes plus a trailing horizontal ellipsis (U+2026). It prefers a whitespace
// boundary within the budget; if that boundary would drop more than half the
// budget it falls back to a hard byte cut. Hard cuts back off by one byte at a
// time to avoid producing invalid UTF-8 when the cut lands mid-rune.
//
// Returns the original string unchanged when it already fits.
func SmartTruncate(s string, maxBytes int) string {
	if nl := strings.IndexByte(s, '\n'); nl >= 0 {
		s = s[:nl]
	}
	s = strings.TrimSpace(s)
	if len(s) <= maxBytes {
		return s
	}

	cut := strings.LastIndexAny(s[:maxBytes], " \t")
	if cut < maxBytes/2 {
		cut = maxBytes
	}
	truncated := s[:cut]
	for len(truncated) > 0 && !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	truncated = strings.TrimRight(truncated, " \t")
	return truncated + "…"
}
