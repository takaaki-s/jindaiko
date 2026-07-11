package codex

import (
	"fmt"
	"strings"

	"github.com/takaaki-s/jind-ai/internal/agent"
)

// SpawnCommand builds the `codex ...` command line the daemon splices into
// its fixed shell wrapper. Manager handles cwd + JIN_SESSION_ID + env -u
// TMUX; we only own the agent-specific pieces:
//
//   - `codex` on the first spawn — Codex has no `--session-id`
//     equivalent, so we spawn fresh and let SessionStart's hook stdin
//     write the actual UUID back into Session.AgentSessionID (see
//     02_design.md §3.5). The pre-mint UUID Manager set on
//     Session.AgentSessionID is intentionally ignored here.
//   - `codex resume <UUID>` once AgentSessionStarted is true and
//     AgentSessionID has been re-keyed to the real Codex UUID. `codex
//     resume` fails fast on an unknown UUID (~3s in Codex 0.144.1, well
//     within the existing 10s quick-fail auto-recovery window), so a
//     stale UUID does not require a defensive glob check up front.
//   - Hook injection via `--enable hooks` + one `-c 'hooks.X=[...]'`
//     per managedEvent (§3.3, §3.4). See hook_args.go.
//
// UnsetEnv clears three Codex sandbox markers so a jind-ai session
// spawned from inside a Codex-created sandbox does not inherit "we're
// already inside a sandbox" state. The three variables mirror the
// [[cc-child-session-env]] discipline the Claude adapter follows;
// authentication vars (CODEX_API_KEY / CODEX_ACCESS_TOKEN /
// OPENAI_API_KEY) are intentionally left set so the spawned Codex can
// authenticate.
func SpawnCommand(opts agent.SpawnOptions, execPath string) agent.SpawnPlan {
	base := "codex"
	if opts.AgentSessionID != "" && opts.AgentSessionStarted {
		base = fmt.Sprintf("codex resume %s", opts.AgentSessionID)
	}
	cmd := base
	if args := HookArgs(execPath); len(args) > 0 {
		cmd = base + " " + strings.Join(args, " ")
	}
	return agent.SpawnPlan{
		Command: cmd,
		UnsetEnv: []string{
			"CODEX_SANDBOX",
			"CODEX_SANDBOX_NETWORK_DISABLED",
			"CODEX_CI",
		},
	}
}
