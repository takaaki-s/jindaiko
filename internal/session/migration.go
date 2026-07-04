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
func migrateSessionJSON(raw []byte) ([]byte, bool, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, false, err
	}

	changed := false

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

	if !changed {
		return raw, false, nil
	}

	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}
