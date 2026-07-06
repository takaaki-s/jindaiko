package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// hooksEntry is a single hook command entry — one row inside a matcher's
// "hooks" array in ~/.claude/settings*.json.
type hooksEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

// hooksMatcher is a hook event matcher plus its handler list. Matcher is
// only set for events that support one (Notification uses a "|"-joined
// regex-like string).
type hooksMatcher struct {
	Matcher string       `json:"matcher,omitempty"`
	Hooks   []hooksEntry `json:"hooks"`
}

// hooksSettings is the top-level shape of the file we write.
type hooksSettings struct {
	Hooks map[string][]hooksMatcher `json:"hooks"`
}

// EnsureHooksSettingsFile generates hooks-settings.json inside stateDir so
// Claude Code (started via `claude --settings <path>`) invokes `jin hook`
// on every event the daemon cares about.
//
// The file is written on every call — it's cheap, and always-write means the
// command path stays correct if the jin binary was upgraded / moved. Returns
// the absolute path to the generated file.
func EnsureHooksSettingsFile(stateDir, execPath string) (string, error) {
	entry := hooksEntry{
		Type:    "command",
		Command: execPath + " hook",
		Timeout: 5,
	}
	settings := hooksSettings{
		Hooks: map[string][]hooksMatcher{
			"UserPromptSubmit": {{Hooks: []hooksEntry{entry}}},
			"Stop":             {{Hooks: []hooksEntry{entry}}},
			"StopFailure":      {{Hooks: []hooksEntry{entry}}},
			"PostToolUse":      {{Hooks: []hooksEntry{entry}}},
			"CwdChanged":       {{Hooks: []hooksEntry{entry}}},
			"SessionStart":     {{Hooks: []hooksEntry{entry}}},
			"SessionEnd":       {{Hooks: []hooksEntry{entry}}},
			"Notification": {{
				Matcher: "permission_prompt|elicitation_dialog|idle_prompt",
				Hooks:   []hooksEntry{entry},
			}},
		},
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal hooks settings: %w", err)
	}

	path := filepath.Join(stateDir, "hooks-settings.json")
	// 0600: match trust state and session store; the file is single-user.
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write hooks settings file: %w", err)
	}

	claudeLog("[HOOKS] Wrote hooks settings to %s", path)
	return path, nil
}
