package cmd

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/takaaki-s/jind-ai/internal/action"
	"github.com/takaaki-s/jind-ai/internal/config"
	"github.com/takaaki-s/jind-ai/internal/daemon"
	"github.com/takaaki-s/jind-ai/internal/paths"
	"github.com/takaaki-s/jind-ai/internal/plugin"
	"github.com/takaaki-s/jind-ai/internal/tmux"
	"github.com/takaaki-s/jind-ai/internal/tui"
)

var actionPopupCmd = &cobra.Command{
	Use:    "action-popup",
	Short:  "Internal: action palette for tmux popup",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Ensure SSH_AUTH_SOCK is available (tmux popup may not inherit it)
		if os.Getenv("SSH_AUTH_SOCK") == "" {
			if tc, err := tmux.NewMgrClient(); err == nil {
				if sock := tc.GetEnvironment(tmux.SessionName, "SSH_AUTH_SOCK"); sock != "" {
					os.Setenv("SSH_AUTH_SOCK", sock)
				}
			}
		}

		configMgr, _ := config.NewManager(getConfigDir())
		kb := actionKeyBindingsFromConfig(configMgr)
		core := action.CoreActions(kb)

		var plugins []action.Action
		if configMgr != nil {
			reg := plugin.NewRegistry(paths.Plugins(), getStateDir(), configMgr.GetPluginsConfig())
			if entries, err := reg.Runnable(); err == nil {
				plugins = action.PluginActions(entries)
			}
		}

		var cursorID, cursorDesc string
		if tc, err := tmux.NewMgrClient(); err == nil {
			cursorID = tc.GetEnvironment(tmux.SessionName, "JIN_CURSOR_SESSION")
		}
		if cursorID != "" {
			client := daemon.NewClient(getSocketPath())
			if info, err := client.Get(cursorID); err == nil {
				cursorDesc = info.Description
			}
		}

		model := tui.NewPaletteModel(core, plugins, cursorID, cursorDesc)
		p := tea.NewProgram(model, tea.WithAltScreen())
		finalModel, err := p.Run()
		if err != nil {
			return err
		}

		// If user selected an action, set tmux env var for parent TUI to pick up
		if m, ok := finalModel.(tui.PaletteModel); ok && m.Selected() != "" {
			if tc, err := tmux.NewMgrClient(); err == nil {
				_ = tc.SetEnvironment(tmux.SessionName, "JIN_ACTION_ID", m.Selected())
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(actionPopupCmd)
}

// actionKeyBindingsFromConfig builds the narrow action.KeyBindings used to
// resolve palette Shortcut display from the full config.KeybindingsConfig.
// A nil manager (config load failure) yields empty bindings so CoreActions
// still returns rows, just without shortcut hints.
func actionKeyBindingsFromConfig(mgr *config.Manager) action.KeyBindings {
	if mgr == nil {
		return action.KeyBindings{}
	}
	kb := mgr.GetKeybindings()
	return action.KeyBindings{
		New:           kb.New,
		Kill:          kb.Kill,
		Delete:        kb.Delete,
		Refresh:       kb.Refresh,
		Vscode:        kb.Vscode,
		Notifications: kb.Notifications,
		Help:          kb.Help,
		TogglePane:    kb.TogglePane,
	}
}
