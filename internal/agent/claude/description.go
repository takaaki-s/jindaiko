// Package claude houses Claude Code-specific glue that must live outside
// internal/session (which is agent-agnostic). The initial inhabitant is the
// Layer C DescriptionEnhancer that mines the first user prompt from a session
// transcript to upgrade the auto-generated Layer A baseline.
package claude

import (
	"strings"
	"unicode/utf8"

	"github.com/takaaki-s/honjin/internal/session"
	"github.com/takaaki-s/honjin/internal/transcript"
)

// descriptionMaxBytes caps the derived description length. 60 bytes keeps the
// value from wrapping in the tabwriter-based `jin session list` output and
// leaves headroom for the locked marker.
const descriptionMaxBytes = 60

// minPromptLen is the shortest prompt we treat as a real hint. Anything under
// three characters is almost certainly a stray keystroke or a slash-command
// abbreviation.
const minPromptLen = 3

// minSlashArgLen is the shortest args string we treat as meaningful when the
// user starts with a slash command. Below this we assume the user is still
// typing the command name or has not fed a description yet.
const minSlashArgLen = 10

// CCDescriptionEnhancer implements session.DescriptionEnhancer using the
// Claude Code transcript on disk. It reads the transcript associated with the
// session's AgentSessionID, extracts the first user turn's text, and applies
// the slash-command aware interpretation documented in the F4 spec.
type CCDescriptionEnhancer struct {
	reader *transcript.Reader
}

// NewCCDescriptionEnhancer builds an enhancer bound to the local ~/.claude
// transcript store. Safe to share across goroutines: the underlying Reader
// only performs read-only file I/O.
func NewCCDescriptionEnhancer() *CCDescriptionEnhancer {
	return &CCDescriptionEnhancer{reader: transcript.NewReader()}
}

// TryGenerate returns a candidate description built from the first user prompt
// of the session's transcript. Returns ("", false) when the session has no
// Claude session id yet, when no transcript exists, when no user turn has been
// recorded, or when the first user turn is still too short to be meaningful
// (see interpretUserPrompt). Never mutates sess.
func (e *CCDescriptionEnhancer) TryGenerate(sess *session.Session) (string, bool) {
	if sess == nil || sess.AgentSessionID == "" {
		return "", false
	}

	// Prefer the last known working directory so we still find the transcript
	// after the session moves into a worktree (which shifts the encoded path
	// under ~/.claude/projects/). ReadEntries falls back to a globbed lookup
	// by sessionID when the path guess misses.
	workDir := sess.CurrentWorkDir
	if workDir == "" {
		workDir = sess.WorkDir
	}

	entries, err := e.reader.ReadEntries(workDir, sess.AgentSessionID, "")
	if err != nil || len(entries) == 0 {
		return "", false
	}

	// Skip past pending user turns (e.g. a bare "/init" at position 0). The
	// transcript is append-only, so if we returned pending on the first user
	// text we'd stay stuck forever — every subsequent hook would re-read the
	// same head entry. Instead, keep scanning for the first user turn whose
	// text produces a real candidate.
	for _, ent := range entries {
		if ent.Type != "user" {
			continue
		}
		text := extractFirstText(ent.Blocks)
		if text == "" {
			continue
		}
		if cand, ok := interpretUserPrompt(text); ok {
			return cand, true
		}
	}
	return "", false
}

// extractFirstText returns the first non-empty "text" block within a user
// turn's blocks. Tool result blocks and other kinds are skipped so we do not
// mistake reply payloads for the user's own words.
func extractFirstText(blocks []transcript.Block) string {
	for _, b := range blocks {
		if b.Kind != "text" {
			continue
		}
		if s := strings.TrimSpace(b.Text); s != "" {
			return s
		}
	}
	return ""
}

// interpretUserPrompt turns the raw first-user-turn text into a description
// candidate following the F4 rules:
//
//   - Empty or trivially short input yields no candidate.
//   - Slash commands with no args (or with args shorter than minSlashArgLen)
//     are considered pending — the user is still composing the request.
//   - Slash commands with meaningful args use the args as the description.
//   - Plain text uses the message body directly.
//
// The returned string is smart-truncated so it fits inside the list view.
func interpretUserPrompt(text string) (string, bool) {
	text = strings.TrimSpace(text)
	// Length thresholds are measured in runes so multi-byte scripts (Japanese,
	// CJK, emoji) aren't unfairly rejected as "too short". A three-character
	// Japanese prompt weighs nine bytes and would fail a byte-length gate.
	if utf8.RuneCountInString(text) < minPromptLen {
		return "", false
	}
	if strings.HasPrefix(text, "/") {
		parts := strings.SplitN(text, " ", 2)
		if len(parts) < 2 {
			return "", false
		}
		args := strings.TrimSpace(parts[1])
		if utf8.RuneCountInString(args) < minSlashArgLen {
			return "", false
		}
		return smartTruncate(args, descriptionMaxBytes), true
	}
	return smartTruncate(text, descriptionMaxBytes), true
}

// smartTruncate keeps the first line of s and shortens it to at most maxBytes
// bytes plus a trailing horizontal ellipsis (U+2026). It prefers a whitespace
// boundary within the budget; if that boundary would drop more than half the
// budget it falls back to a hard byte cut. Hard cuts back off by one byte at a
// time to avoid producing invalid UTF-8 when the cut lands mid-rune.
//
// Returns the original string unchanged when it already fits.
func smartTruncate(s string, maxBytes int) string {
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
