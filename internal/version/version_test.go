package version

import (
	"strings"
	"testing"
)

func TestFull_Default(t *testing.T) {
	got := Full()
	if !strings.Contains(got, "dev") {
		t.Errorf("Full() = %q, want to contain 'dev'", got)
	}
	if !strings.Contains(got, "none") {
		t.Errorf("Full() = %q, want to contain 'none'", got)
	}
}

func TestFull_WithValues(t *testing.T) {
	origVersion, origCommit, origDate := Version, Commit, Date
	defer func() {
		Version, Commit, Date = origVersion, origCommit, origDate
	}()

	Version = "1.2.3"
	Commit = "abc1234"
	Date = "2026-01-01"

	want := "1.2.3 (commit: abc1234, built: 2026-01-01)"
	got := Full()
	if got != want {
		t.Errorf("Full() = %q, want %q", got, want)
	}
}
