// Package plugin discovers, verifies, and runs jin plugins: user-installed
// programs that react to session events (status_changed) or explicit actions.
// Each plugin lives in its own directory under Plugins() and declares a
// jin-plugin.yaml manifest (name, api_version, on, run, build, timeout).
//
// This file owns the manifest: its schema, loading, validation, and the
// matcher grammar used by the dispatcher to decide which plugins an event
// reaches. Compatibility of a manifest's api_version is handled in version.go.
package plugin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ManifestFilename is the fixed name of a plugin's manifest, read from the
// plugin's directory root.
const ManifestFilename = "jin-plugin.yaml"

// DefaultTimeout is applied to a plugin run when the manifest omits timeout.
const DefaultTimeout = 30 * time.Second

// EventStatusChanged is the only event a matcher may target in api v1.
const EventStatusChanged = "status_changed"

// namePattern constrains plugin names so they are safe as directory names and
// as env/log identifiers.
var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Manifest is the parsed jin-plugin.yaml. Timeout is written as a duration
// string (e.g. "30s") in YAML and parsed via UnmarshalYAML; a zero value means
// "unset" and callers should use EffectiveTimeout.
type Manifest struct {
	Name       string        `yaml:"name"`
	APIVersion int           `yaml:"api_version"`
	On         []string      `yaml:"on"`
	Run        string        `yaml:"run"`
	Build      string        `yaml:"build,omitempty"`
	Timeout    time.Duration `yaml:"timeout,omitempty"`
}

// UnmarshalYAML decodes the manifest, translating the human-friendly timeout
// string (e.g. "30s") into a time.Duration. A shadow struct avoids recursing
// into this method and lets Timeout arrive as a string.
func (m *Manifest) UnmarshalYAML(value *yaml.Node) error {
	var raw struct {
		Name       string   `yaml:"name"`
		APIVersion int      `yaml:"api_version"`
		On         []string `yaml:"on"`
		Run        string   `yaml:"run"`
		Build      string   `yaml:"build"`
		Timeout    string   `yaml:"timeout"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}

	m.Name = raw.Name
	m.APIVersion = raw.APIVersion
	m.On = raw.On
	m.Run = raw.Run
	m.Build = raw.Build
	if raw.Timeout != "" {
		d, err := time.ParseDuration(raw.Timeout)
		if err != nil {
			return fmt.Errorf("parse timeout %q: %w", raw.Timeout, err)
		}
		m.Timeout = d
	}
	return nil
}

// EffectiveTimeout returns the run timeout, substituting DefaultTimeout when
// the manifest left it unset (or non-positive).
func (m *Manifest) EffectiveTimeout() time.Duration {
	if m.Timeout <= 0 {
		return DefaultTimeout
	}
	return m.Timeout
}

// Validate checks the manifest's required fields and grammar. It does not
// check api_version compatibility with this jin build — see CheckAPIVersion.
func (m *Manifest) Validate() error {
	if m.Name == "" {
		return errors.New("manifest: name is required")
	}
	if !namePattern.MatchString(m.Name) {
		return fmt.Errorf("manifest: name %q must match %s", m.Name, namePattern.String())
	}
	if m.APIVersion < 1 {
		return fmt.Errorf("manifest: api_version is required and must be >= 1 (got %d)", m.APIVersion)
	}
	if m.Run == "" {
		return errors.New("manifest: run is required")
	}
	for _, matcher := range m.On {
		if err := ValidateMatcher(matcher); err != nil {
			return fmt.Errorf("manifest: on: %w", err)
		}
	}
	return nil
}

// LoadManifest reads and validates <pluginDir>/jin-plugin.yaml.
func LoadManifest(pluginDir string) (*Manifest, error) {
	path := filepath.Join(pluginDir, ManifestFilename)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}

	var m Manifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("decode manifest %s: %w", path, err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// ValidateMatcher reports whether an `on:` entry is well-formed. A matcher is
// either "status_changed" (every status_changed event) or
// "status_changed:<status>" (only that status), where <status> is non-empty.
func ValidateMatcher(matcher string) error {
	name, status, hasStatus := strings.Cut(matcher, ":")
	if name != EventStatusChanged {
		return fmt.Errorf("unknown event %q (only %q is supported)", name, EventStatusChanged)
	}
	if hasStatus && status == "" {
		return fmt.Errorf("matcher %q has an empty status after ':'", matcher)
	}
	return nil
}

// MatcherMatches reports whether a matcher selects the given event and status.
// A bare "status_changed" matches any status; "status_changed:<status>"
// matches only when status equals the suffix. Callers pass matchers that have
// already passed ValidateMatcher.
func MatcherMatches(matcher, event, status string) bool {
	name, want, hasStatus := strings.Cut(matcher, ":")
	if name != event {
		return false
	}
	if !hasStatus {
		return true
	}
	return want == status
}
