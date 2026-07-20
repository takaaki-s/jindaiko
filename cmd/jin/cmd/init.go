package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// configTemplate is the default config.yaml written by `jin init`
const configTemplate = `# jin configuration
# See https://github.com/takaaki-s/jind-ai for details

# Customize keybindings (defaults are used when omitted)
keybindings:
  # Session list view
  up: ["up", "k"]
  down: ["down", "j"]
  attach: ["enter"]
  new: ["n"]
  kill: ["x"]
  delete: ["d"]
  refresh: ["r"]
  # search: keys that open the session filter popup (fuzzy search).
  # Default is M-f (Alt+f); must be modifier-prefixed to avoid stealing
  # input from the display pane. Use ["/"] to restore the old bare-slash
  # binding (breaks agent slash-commands / vim-like search in the display pane).
  search: ["M-f"]
  vscode: ["v"]
  notifications: ["!"]
  quit: ["q", "ctrl+c"]
  help: ["?"]
  # Session creation form
  next_field: ["tab"]
  prev_field: ["shift+tab"]
  submit: ["enter"]
  cancel_form: ["esc"]
  # While attached
  # Supported keys: ctrl+^, ctrl+], ctrl+\, ctrl+g
  detach: ["ctrl+]"]

# Adapter used when 'jin session new' omits --agent.
# Leave commented to fall back to "claude". Uncomment and change to
# override (available kinds: "claude", "codex", "opencode").
# default_agent: claude

# Popup size overrides (percent-based, 1-100).
# Any omitted field falls back to the hardcoded default shown below.
# Out-of-range values log a warn and fall back silently.
# popups:
#   create:         { width: 80, height: 80 }
#   notify:         { width: 70, height: 60 }
#   session_filter: { width: 70, height: 70 }
#   help:           { width: 60, height: 60 }
#   action:         { width: 70, height: 70 }
#   # plugin_default applies to every plugin popup unless overridden per-plugin.
#   plugin_default: { width: 70, height: 70 }
#   # Per-plugin overrides beat the plugin's own manifest declaration.
#   plugins:
#     my-plugin:    { width: 40, height: 20 }
`

var forceInit bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize jin configuration",
	Long: `Create the jin configuration directory and a default config.yaml.

If config.yaml already exists, this command does nothing unless --force is specified.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir := getConfigDir()
		configFile := filepath.Join(configDir, "config.yaml")

		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		if !forceInit {
			if _, err := os.Stat(configFile); err == nil {
				fmt.Printf("Config already exists: %s\n", configFile)
				fmt.Println("Use --force to overwrite.")
				return nil
			}
		}

		if err := os.WriteFile(configFile, []byte(configTemplate), 0644); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}

		fmt.Printf("Created: %s\n", configFile)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&forceInit, "force", false, "Overwrite existing config.yaml")
}
