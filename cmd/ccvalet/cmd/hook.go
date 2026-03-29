package cmd

import (
	"encoding/json"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/takaaki-s/claude-code-valet/internal/daemon"
	"github.com/takaaki-s/claude-code-valet/internal/debug"
)

// hookInput represents the JSON input from Claude Code hooks (stdin)
type hookInput struct {
	SessionID        string `json:"session_id"`
	HookEventName    string `json:"hook_event_name"`
	NotificationType string `json:"notification_type,omitempty"`
	CWD              string `json:"cwd,omitempty"`
	StopReason       string `json:"stop_reason,omitempty"`
}

var hookLog = debug.NewLogger("hook-debug.log")

var hookCmd = &cobra.Command{
	Use:    "hook",
	Short:  "Handle Claude Code hook events (stdin JSON)",
	Long:   "Internal command invoked by Claude Code hooks. Reads JSON from stdin and notifies the daemon.",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Read JSON from stdin
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			hookLog("failed to read stdin: %v", err)
			return nil // Always exit 0
		}

		var input hookInput
		if err := json.Unmarshal(data, &input); err != nil {
			hookLog("failed to parse JSON: %v (data: %s)", err, string(data))
			return nil
		}

		if input.SessionID == "" || input.HookEventName == "" {
			hookLog("missing required fields: session_id=%q hook_event_name=%q", input.SessionID, input.HookEventName)
			return nil
		}

		// Read ccvalet session ID from environment (set by ccvalet when starting Claude)
		ccvaletSessionID := os.Getenv("CCVALET_SESSION_ID")
		if ccvaletSessionID == "" {
			return nil // Not a ccvalet-managed session, skip
		}

		hookLog("event=%s cc_session=%s ccvalet_session=%s notification=%s", input.HookEventName, input.SessionID, ccvaletSessionID, input.NotificationType)

		// Send to daemon
		client := daemon.NewClient(getSocketPath())
		if err := client.SendHook(daemon.HookRequest{
			SessionID:        input.SessionID,
			CcvaletSessionID: ccvaletSessionID,
			HookEventName:    input.HookEventName,
			NotificationType: input.NotificationType,
			CWD:              input.CWD,
			StopReason:       input.StopReason,
		}); err != nil {
			hookLog("SendHook failed: %v", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(hookCmd)
}
