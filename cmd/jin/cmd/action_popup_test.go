package cmd

import (
	"reflect"
	"testing"

	"github.com/takaaki-s/jind-ai/internal/config"
)

func TestActionPopupCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"action-popup"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd == nil || cmd.Name() != "action-popup" {
		t.Fatalf("action-popup not registered: %+v", cmd)
	}
	if !cmd.Hidden {
		t.Errorf("action-popup should be Hidden")
	}
}

// TestActionKeyBindingsFromConfig_TogglePaneDefault regresses the "toggle
// sidebar shortcut is blank" bug: on a fresh install with no config file,
// KeybindingsConfig.TogglePane is nil, and reading it directly from
// GetKeybindings() blanked the palette hint because GetKeybindings has no
// len==0 fallback for that field. The fix routes TogglePane through
// GetTogglePaneKeys(), which nil-checks against DefaultKeybindings.
func TestActionKeyBindingsFromConfig_TogglePaneDefault(t *testing.T) {
	dir := t.TempDir()
	mgr, err := config.NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	akb := actionKeyBindingsFromConfig(mgr)
	want := config.DefaultKeybindings().TogglePane
	if !reflect.DeepEqual(akb.TogglePane, want) {
		t.Errorf("TogglePane on fresh config = %q, want %q (default)", akb.TogglePane, want)
	}
}

func TestActionKeyBindingsFromConfig_NilManager(t *testing.T) {
	akb := actionKeyBindingsFromConfig(nil)
	if akb.TogglePane != nil {
		t.Errorf("TogglePane with nil manager = %q, want nil", akb.TogglePane)
	}
}
