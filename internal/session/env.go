package session

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// validEnvKeyPattern matches valid environment variable key names.
// Must start with a letter or underscore, followed by letters, digits, or underscores.
var validEnvKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// buildEnvString builds a shell-safe environment variable string from the given map.
// Keys are sorted for deterministic output. Invalid keys are skipped with a warning.
// Values are shell-escaped by wrapping in single quotes and escaping embedded single quotes.
func buildEnvString(envMap map[string]string) string {
	if len(envMap) == 0 {
		return ""
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(envMap))
	for k := range envMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		if !validEnvKeyPattern.MatchString(k) {
			debugLog("WARNING: skipping invalid env key %q (must match %s)", k, validEnvKeyPattern.String())
			continue
		}
		v := shellEscape(envMap[k])
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}

	return strings.Join(parts, " ")
}

// shellEscape wraps a value in single quotes and escapes any embedded single quotes.
// The escape sequence for a single quote within single quotes is: '\''
// (end quote, escaped quote, start quote)
func shellEscape(s string) string {
	escaped := strings.ReplaceAll(s, "'", "'\\''")
	return "'" + escaped + "'"
}
