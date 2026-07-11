package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/takaaki-s/jind-ai/internal/tmux"
	"github.com/takaaki-s/jind-ai/internal/tui"
)

var createPopupCmd = &cobra.Command{
	Use:    "create-popup",
	Short:  "Internal: session creation form for tmux popup",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Restore SSH_AUTH_SOCK if the popup inherited a stripped env, and
		// read the transient default adapter kind set by `jin ui --agent`.
		// One tmux client shared for both to keep the popup fast.
		initialAgentKind := ""
		if tc, err := tmux.NewMgrClient(); err == nil {
			ensureSSHAuthSockFromTmux(tc)
			initialAgentKind = tc.GetEnvironment(tmux.SessionName, "JIN_UI_AGENT")
		}

		model := tui.NewCreateFormModel(getSocketPath(), initialAgentKind)
		p := tea.NewProgram(model, tea.WithAltScreen())
		_, err := p.Run()
		return err
	},
}

func init() {
	rootCmd.AddCommand(createPopupCmd)
}
