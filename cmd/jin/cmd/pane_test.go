package cmd

import (
	"strings"
	"testing"
)

// TestPanePopupCmd_HereFlagRegistered verifies the --here flag exists on the
// popup subcommand.
func TestPanePopupCmd_HereFlagRegistered(t *testing.T) {
	if panePopupCmd.Flags().Lookup("here") == nil {
		t.Error("panePopupCmd is missing the --here flag")
	}
}

// TestRunPopupHere_NoTmuxClient verifies that --here fails with a clear error
// when neither $TMUX nor JIN_CALLER_TMUX_SOCKET can resolve a server socket.
func TestRunPopupHere_NoTmuxClient(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	t.Setenv("JIN_CALLER_TMUX_SOCKET", "")
	t.Setenv("JIN_CALLER_TMUX_PANE", "")

	err := runPopupHere("echo hi", "", "", "")
	if err == nil {
		t.Fatal("expected error when no tmux client is resolvable, got nil")
	}
	if !strings.Contains(err.Error(), "requires a tmux client") {
		t.Errorf("error = %q, want to mention %q", err.Error(), "requires a tmux client")
	}
}

// TestPopupSizeWithEnvFallback_FlagOverridesEnv verifies an explicit flag
// value wins over the JIN_PLUGIN_POPUP_* env vars.
func TestPopupSizeWithEnvFallback_FlagOverridesEnv(t *testing.T) {
	t.Setenv("JIN_PLUGIN_POPUP_WIDTH", "80%")
	t.Setenv("JIN_PLUGIN_POPUP_HEIGHT", "80%")

	width, height := popupSizeWithEnvFallback("30%", "40%")
	if width != "30%" {
		t.Errorf("width = %q, want %q", width, "30%")
	}
	if height != "40%" {
		t.Errorf("height = %q, want %q", height, "40%")
	}
}

// TestPopupSizeWithEnvFallback_UsesEnvWhenFlagEmpty verifies the env var is
// used when the corresponding flag was left unset.
func TestPopupSizeWithEnvFallback_UsesEnvWhenFlagEmpty(t *testing.T) {
	t.Setenv("JIN_PLUGIN_POPUP_WIDTH", "80%")
	t.Setenv("JIN_PLUGIN_POPUP_HEIGHT", "60%")

	width, height := popupSizeWithEnvFallback("", "")
	if width != "80%" {
		t.Errorf("width = %q, want %q", width, "80%")
	}
	if height != "60%" {
		t.Errorf("height = %q, want %q", height, "60%")
	}
}

// TestPopupSizeWithEnvFallback_EmptyWhenBothMissing verifies both values
// stay empty when neither the flag nor the env var is set, so tmux falls
// back to its own default.
func TestPopupSizeWithEnvFallback_EmptyWhenBothMissing(t *testing.T) {
	t.Setenv("JIN_PLUGIN_POPUP_WIDTH", "")
	t.Setenv("JIN_PLUGIN_POPUP_HEIGHT", "")

	width, height := popupSizeWithEnvFallback("", "")
	if width != "" {
		t.Errorf("width = %q, want empty", width)
	}
	if height != "" {
		t.Errorf("height = %q, want empty", height)
	}
}

// TestPopupSizeWithEnvFallback_Table exercises width/height fallback
// precedence independently via a table of flag/env combinations.
func TestPopupSizeWithEnvFallback_Table(t *testing.T) {
	tests := []struct {
		name       string
		flagWidth  string
		flagHeight string
		envWidth   string
		envHeight  string
		wantWidth  string
		wantHeight string
	}{
		{
			name:       "both flags set, env ignored",
			flagWidth:  "50%",
			flagHeight: "50%",
			envWidth:   "90%",
			envHeight:  "90%",
			wantWidth:  "50%",
			wantHeight: "50%",
		},
		{
			name:       "only width flag set",
			flagWidth:  "50%",
			flagHeight: "",
			envWidth:   "90%",
			envHeight:  "90%",
			wantWidth:  "50%",
			wantHeight: "90%",
		},
		{
			name:       "only height flag set",
			flagWidth:  "",
			flagHeight: "50%",
			envWidth:   "90%",
			envHeight:  "90%",
			wantWidth:  "90%",
			wantHeight: "50%",
		},
		{
			name:       "no flags, no env",
			flagWidth:  "",
			flagHeight: "",
			envWidth:   "",
			envHeight:  "",
			wantWidth:  "",
			wantHeight: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("JIN_PLUGIN_POPUP_WIDTH", tt.envWidth)
			t.Setenv("JIN_PLUGIN_POPUP_HEIGHT", tt.envHeight)

			width, height := popupSizeWithEnvFallback(tt.flagWidth, tt.flagHeight)
			if width != tt.wantWidth {
				t.Errorf("width = %q, want %q", width, tt.wantWidth)
			}
			if height != tt.wantHeight {
				t.Errorf("height = %q, want %q", height, tt.wantHeight)
			}
		})
	}
}
