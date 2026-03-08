package debug

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// enabled is true when CCVALET_DEBUG=1 is set.
var enabled = os.Getenv("CCVALET_DEBUG") == "1"

// NewLogger returns a debug logging function that writes to
// ~/.ccvalet/<filename> when CCVALET_DEBUG=1 is set.
// When debugging is disabled, the returned function is a no-op.
func NewLogger(filename string) func(string, ...any) {
	if !enabled {
		return func(string, ...any) {}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return func(string, ...any) {}
	}
	logPath := filepath.Join(home, ".ccvalet", filename)

	return func(format string, args ...any) {
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		defer f.Close()
		msg := fmt.Sprintf(format, args...)
		fmt.Fprintf(f, "[%s] %s\n", time.Now().Format("15:04:05.000"), msg)
	}
}
