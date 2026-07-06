package session

import "encoding/json"

// migrateSessionJSON reads raw session JSON, applies schema migrations in-place,
// and returns the (possibly rewritten) JSON along with a flag indicating whether
// any change was applied. It is idempotent: running it twice on the same input
// produces the same output on the second call with changed=false.
//
// Currently handled migrations:
//   - v1 → v2: rename "name" to "description" and set "description_locked" = true
//     (the historical "name" value was manually chosen by the user, so lock it).
//   - v2 → v3: rename "claude_session_id" / "claude_session_started" to their
//     agent-agnostic equivalents ("agent_session_id" / "agent_session_started")
//     and backfill "agent_kind" with "claude". Legacy records predate the
//     agent-abstraction split, so every unmarked session is by definition a
//     Claude Code session.
func migrateSessionJSON(raw []byte) ([]byte, bool, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, false, err
	}

	changed := false

	// v1 → v2 -----------------------------------------------------------
	if rawName, ok := m["name"]; ok {
		if name, isString := rawName.(string); isString && name != "" {
			if desc, _ := m["description"].(string); desc == "" {
				m["description"] = name
				m["description_locked"] = true
			}
		}
		delete(m, "name")
		changed = true
	}

	// v2 → v3 -----------------------------------------------------------
	// AgentKind is the invariant "always present" identifier; use "claude"
	// as the backfill because every pre-migration record was a Claude Code
	// session. A record that already carries a non-empty agent_kind is
	// left untouched (idempotency + future-agent compatibility).
	if k, _ := m["agent_kind"].(string); k == "" {
		m["agent_kind"] = "claude"
		changed = true
	}
	// Rename claude_session_id → agent_session_id. Do not clobber a value
	// the new field already carries — that only happens when someone
	// hand-edited a record mid-migration, but preserving the newer field
	// is the safer resolution.
	if rawCC, ok := m["claude_session_id"]; ok {
		if id, isString := rawCC.(string); isString && id != "" {
			if existing, _ := m["agent_session_id"].(string); existing == "" {
				m["agent_session_id"] = id
			}
		}
		delete(m, "claude_session_id")
		changed = true
	}
	if rawStarted, ok := m["claude_session_started"]; ok {
		if started, isBool := rawStarted.(bool); isBool {
			if _, existing := m["agent_session_started"].(bool); !existing {
				m["agent_session_started"] = started
			}
		}
		delete(m, "claude_session_started")
		changed = true
	}

	if !changed {
		return raw, false, nil
	}

	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}
