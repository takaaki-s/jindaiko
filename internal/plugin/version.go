package plugin

import "fmt"

// CurrentAPIVersion is the newest plugin API version this jin build speaks.
const CurrentAPIVersion = 1

// MinAPIVersion is the oldest plugin API version this jin build still accepts.
const MinAPIVersion = 1

// CheckAPIVersion reports whether a plugin declaring api_version v is
// compatible with this jin build. When v is outside [MinAPIVersion,
// CurrentAPIVersion] the error names the fix direction: too new means jin is
// behind, too old means the plugin is behind.
func CheckAPIVersion(v int) error {
	if v > CurrentAPIVersion {
		return fmt.Errorf("plugin requires api %d; this jin supports %d-%d (upgrade jin)", v, MinAPIVersion, CurrentAPIVersion)
	}
	if v < MinAPIVersion {
		return fmt.Errorf("plugin declares api %d; this jin supports %d-%d (update the plugin)", v, MinAPIVersion, CurrentAPIVersion)
	}
	return nil
}
