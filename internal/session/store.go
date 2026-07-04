package session

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Store handles session persistence
type Store struct {
	dataDir string
}

// NewStore creates a new store
func NewStore(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}
	return &Store{dataDir: dataDir}, nil
}

// Save persists a session
func (s *Store) Save(session *Session) error {
	path := filepath.Join(s.dataDir, session.ID+".json")
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Load loads a session by ID. Legacy schema (top-level "name") is migrated
// in-place to the current schema; the migrated JSON is written back to disk
// so we only pay the cost once per session file.
func (s *Store) Load(id string) (*Session, error) {
	path := filepath.Join(s.dataDir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	migrated, changed, err := migrateSessionJSON(data)
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(migrated, &session); err != nil {
		return nil, err
	}

	if changed {
		if err := s.Save(&session); err != nil {
			return nil, err
		}
	}
	return &session, nil
}

// LoadAll loads all sessions.
//
// Files that fail Load (unparseable JSON, migration write-back failure, missing
// permissions, ...) are skipped instead of aborting so a single corrupt file
// doesn't strand every session. The individual failure is emitted via
// debugLog so it still surfaces under JIN_DEBUG=1.
func (s *Store) LoadAll() ([]*Session, error) {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []*Session
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := entry.Name()[:len(entry.Name())-5] // Remove .json
		session, err := s.Load(id)
		if err != nil {
			debugLog("[LOAD] skip %s: %v", id, err)
			continue
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

// Delete removes a session file
func (s *Store) Delete(id string) error {
	path := filepath.Join(s.dataDir, id+".json")
	err := os.Remove(path)
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}
