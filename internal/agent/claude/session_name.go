// Package claude — this file adds a reader for the "name" Claude Code 2.x
// assigns to a session in ~/.claude/sessions/<PID>.json. It exists to feed
// the Layer C-name fallback used by CCDescriptionEnhancer when no transcript
// is available yet (e.g. at SessionStart).
package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// CCSessionNameReader looks up the name Claude Code assigns to a session by
// matching ccSessionID against the sessionId field of files under
// ~/.claude/sessions/*.json. Safe for concurrent read-only use.
type CCSessionNameReader struct{}

// NewCCSessionNameReader builds a reader bound to the local ~/.claude/sessions
// store.
func NewCCSessionNameReader() *CCSessionNameReader {
	return &CCSessionNameReader{}
}

// ccSessionFile mirrors the fields of interest in a
// ~/.claude/sessions/<PID>.json file. Unknown fields are ignored.
type ccSessionFile struct {
	SessionID  string `json:"sessionId"`
	Name       string `json:"name"`
	NameSource string `json:"nameSource"`
}

// LookupName returns the CC-assigned name (and its nameSource) for the
// session file whose sessionId matches ccSessionID. It returns ("", "",
// false) on any miss: empty ccSessionID, unresolvable HOME, missing sessions
// directory, unreadable or malformed JSON, or a matching file with an empty
// name (older CC versions that predate the field). Never returns an error —
// all failure modes are silent fallbacks by design (F5).
func (r *CCSessionNameReader) LookupName(ccSessionID string) (name, nameSource string, ok bool) {
	if ccSessionID == "" {
		return "", "", false
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", false
	}

	matches, err := filepath.Glob(filepath.Join(home, ".claude", "sessions", "*.json"))
	if err != nil {
		return "", "", false
	}

	for _, path := range matches {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var f ccSessionFile
		if err := json.Unmarshal(raw, &f); err != nil {
			continue
		}
		if f.SessionID == ccSessionID && f.Name != "" {
			return f.Name, f.NameSource, true
		}
	}

	return "", "", false
}
