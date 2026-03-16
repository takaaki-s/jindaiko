package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/takaaki-s/claude-code-valet/internal/config"
)

// --- sshArgs ---

func TestSSHArgs(t *testing.T) {
	t.Run("without SSHOpts", func(t *testing.T) {
		hc := &config.HostConfig{
			ID:   "test-host",
			Type: "ssh",
			Host: "example.com",
		}
		args := sshArgs(hc)

		// Should contain the 3 fixed -o options (6 args)
		if len(args) != 6 {
			t.Fatalf("sshArgs() returned %d args, want 6: %v", len(args), args)
		}

		if args[0] != "-o" || !strings.HasPrefix(args[1], "ControlMaster=") {
			t.Errorf("expected ControlMaster option, got %q %q", args[0], args[1])
		}
		if args[2] != "-o" || !strings.HasPrefix(args[3], "ControlPath=") {
			t.Errorf("expected ControlPath option, got %q %q", args[2], args[3])
		}
		if !strings.Contains(args[3], "test-host") {
			t.Errorf("ControlPath should contain host ID 'test-host', got %q", args[3])
		}
		if args[4] != "-o" || args[5] != "ClearAllForwardings=yes" {
			t.Errorf("expected ClearAllForwardings option, got %q %q", args[4], args[5])
		}
	})

	t.Run("with SSHOpts", func(t *testing.T) {
		hc := &config.HostConfig{
			ID:      "remote-dev",
			Type:    "ssh",
			Host:    "dev.example.com",
			SSHOpts: []string{"-p", "2222", "-i", "/path/to/key"},
		}
		args := sshArgs(hc)

		// 6 fixed args + 4 SSHOpts
		if len(args) != 10 {
			t.Fatalf("sshArgs() returned %d args, want 10: %v", len(args), args)
		}

		// Verify SSHOpts are appended at the end
		if args[6] != "-p" || args[7] != "2222" {
			t.Errorf("expected -p 2222, got %q %q", args[6], args[7])
		}
		if args[8] != "-i" || args[9] != "/path/to/key" {
			t.Errorf("expected -i /path/to/key, got %q %q", args[8], args[9])
		}
	})

	t.Run("with empty SSHOpts", func(t *testing.T) {
		hc := &config.HostConfig{
			ID:      "host-empty",
			Type:    "ssh",
			Host:    "host.local",
			SSHOpts: []string{},
		}
		args := sshArgs(hc)
		if len(args) != 6 {
			t.Fatalf("sshArgs() with empty SSHOpts returned %d args, want 6: %v", len(args), args)
		}
	})
}

// --- DirPickerModel.IsRemote ---

func TestDirPickerModel_IsRemote(t *testing.T) {
	t.Run("default is not remote", func(t *testing.T) {
		m := DirPickerModel{}
		if m.IsRemote() {
			t.Error("IsRemote() should return false by default")
		}
	})

	t.Run("with hostConfig is remote", func(t *testing.T) {
		m := DirPickerModel{
			hostConfig: &config.HostConfig{
				ID:   "remote",
				Type: "ssh",
				Host: "example.com",
			},
		}
		if !m.IsRemote() {
			t.Error("IsRemote() should return true when hostConfig is set")
		}
	})
}

// --- DirPickerModel.Selected ---

func TestDirPickerModel_Selected(t *testing.T) {
	t.Run("default is not selected", func(t *testing.T) {
		m := DirPickerModel{}
		if m.Selected() {
			t.Error("Selected() should return false by default")
		}
	})

	t.Run("returns true when selected", func(t *testing.T) {
		m := DirPickerModel{selected: true}
		if !m.Selected() {
			t.Error("Selected() should return true when selected is set")
		}
	})
}

// --- DirPickerModel.Result ---

func TestDirPickerModel_Result(t *testing.T) {
	t.Run("default is empty", func(t *testing.T) {
		m := DirPickerModel{}
		if m.Result() != "" {
			t.Errorf("Result() should be empty by default, got %q", m.Result())
		}
	})

	t.Run("returns set result", func(t *testing.T) {
		m := DirPickerModel{result: "/home/user/project"}
		if m.Result() != "/home/user/project" {
			t.Errorf("Result() = %q, want %q", m.Result(), "/home/user/project")
		}
	})
}

// --- DirPickerModel.totalItems ---

func TestDirPickerModel_TotalItems(t *testing.T) {
	t.Run("empty model", func(t *testing.T) {
		m := DirPickerModel{}
		if m.totalItems() != 0 {
			t.Errorf("totalItems() = %d, want 0", m.totalItems())
		}
	})

	t.Run("with filtered entries only", func(t *testing.T) {
		m := DirPickerModel{
			filtered: []string{"dir1", "dir2", "dir3"},
		}
		if m.totalItems() != 3 {
			t.Errorf("totalItems() = %d, want 3", m.totalItems())
		}
	})

	t.Run("with filtered history only", func(t *testing.T) {
		m := DirPickerModel{
			filteredHistory: []HistoryEntry{
				{Path: "/home/user/a", DisplayPath: "~/a"},
				{Path: "/home/user/b", DisplayPath: "~/b"},
			},
		}
		if m.totalItems() != 2 {
			t.Errorf("totalItems() = %d, want 2", m.totalItems())
		}
	})

	t.Run("with both entries and history", func(t *testing.T) {
		m := DirPickerModel{
			filtered: []string{"dir1", "dir2"},
			filteredHistory: []HistoryEntry{
				{Path: "/home/user/a", DisplayPath: "~/a"},
			},
		}
		if m.totalItems() != 3 {
			t.Errorf("totalItems() = %d, want 3", m.totalItems())
		}
	})
}

// --- DirPickerModel.ClearRemoteHost ---

func TestDirPickerModel_ClearRemoteHost(t *testing.T) {
	m := DirPickerModel{
		hostConfig: &config.HostConfig{
			ID:   "remote",
			Type: "ssh",
			Host: "example.com",
		},
		remoteHome: "/home/remoteuser",
		loading:    true,
		cursor:     5,
		offset:     3,
	}

	m.ClearRemoteHost()

	if m.hostConfig != nil {
		t.Error("ClearRemoteHost() should set hostConfig to nil")
	}
	if m.remoteHome != "" {
		t.Errorf("ClearRemoteHost() should clear remoteHome, got %q", m.remoteHome)
	}
	if m.loading {
		t.Error("ClearRemoteHost() should set loading to false")
	}
	if m.IsRemote() {
		t.Error("IsRemote() should return false after ClearRemoteHost()")
	}
	if m.cursor != 0 {
		t.Errorf("ClearRemoteHost() should reset cursor to 0, got %d", m.cursor)
	}
	if m.offset != 0 {
		t.Errorf("ClearRemoteHost() should reset offset to 0, got %d", m.offset)
	}
	// currentDir should be set to home or "/"
	if m.currentDir == "" {
		t.Error("ClearRemoteHost() should set currentDir to home directory or /")
	}
}

// --- DirPickerModel.SetHistory ---

func TestDirPickerModel_SetHistory(t *testing.T) {
	t.Run("set history entries", func(t *testing.T) {
		m := DirPickerModel{}
		now := time.Now()
		entries := []HistoryEntry{
			{Path: "/home/user/project1", DisplayPath: "~/project1", LastUsedAt: now},
			{Path: "/home/user/project2", DisplayPath: "~/project2", LastUsedAt: now.Add(-time.Hour)},
		}

		m.SetHistory(entries)

		if len(m.historyDirs) != 2 {
			t.Fatalf("SetHistory() stored %d entries, want 2", len(m.historyDirs))
		}
		if m.historyDirs[0].Path != "/home/user/project1" {
			t.Errorf("historyDirs[0].Path = %q, want %q", m.historyDirs[0].Path, "/home/user/project1")
		}
		if m.historyDirs[1].Path != "/home/user/project2" {
			t.Errorf("historyDirs[1].Path = %q, want %q", m.historyDirs[1].Path, "/home/user/project2")
		}
		// filteredHistory should also be populated (no filter active)
		if len(m.filteredHistory) != 2 {
			t.Errorf("filteredHistory should have 2 entries, got %d", len(m.filteredHistory))
		}
	})

	t.Run("set empty history", func(t *testing.T) {
		m := DirPickerModel{
			historyDirs: []HistoryEntry{
				{Path: "/old", DisplayPath: "/old"},
			},
		}

		m.SetHistory(nil)

		if m.historyDirs != nil {
			t.Errorf("SetHistory(nil) should set historyDirs to nil, got %v", m.historyDirs)
		}
		if m.filteredHistory != nil {
			t.Errorf("filteredHistory should be nil after SetHistory(nil), got %v", m.filteredHistory)
		}
	})
}

// --- DirPickerModel.applyFilter ---

func TestDirPickerModel_ApplyFilter(t *testing.T) {
	t.Run("empty query returns all entries", func(t *testing.T) {
		m := DirPickerModel{
			entries: []string{"alpha", "beta", "gamma"},
		}
		m.applyFilter()

		if len(m.filtered) != 3 {
			t.Fatalf("applyFilter() with empty query returned %d entries, want 3", len(m.filtered))
		}
	})

	t.Run("filter matches case-insensitively", func(t *testing.T) {
		m := DirPickerModel{
			entries: []string{"Documents", "Downloads", "Desktop", "Pictures", "Music"},
		}
		m.filterInput.SetValue("do")
		m.applyFilter()

		if len(m.filtered) != 2 {
			t.Fatalf("applyFilter('do') returned %d entries, want 2: %v", len(m.filtered), m.filtered)
		}
		// Should match "Documents" and "Downloads"
		for _, f := range m.filtered {
			lower := strings.ToLower(f)
			if !strings.Contains(lower, "do") {
				t.Errorf("filtered entry %q does not contain 'do'", f)
			}
		}
	})

	t.Run("filter with no matches", func(t *testing.T) {
		m := DirPickerModel{
			entries: []string{"alpha", "beta", "gamma"},
		}
		m.filterInput.SetValue("xyz")
		m.applyFilter()

		if len(m.filtered) != 0 {
			t.Errorf("applyFilter('xyz') returned %d entries, want 0: %v", len(m.filtered), m.filtered)
		}
	})

	t.Run("filter starting with / returns all entries", func(t *testing.T) {
		m := DirPickerModel{
			entries: []string{"alpha", "beta", "gamma"},
		}
		m.filterInput.SetValue("/home/user")
		m.applyFilter()

		if len(m.filtered) != 3 {
			t.Fatalf("applyFilter with path query should return all entries, got %d", len(m.filtered))
		}
	})

	t.Run("filter starting with ~ returns all entries", func(t *testing.T) {
		m := DirPickerModel{
			entries: []string{"alpha", "beta"},
		}
		m.filterInput.SetValue("~/projects")
		m.applyFilter()

		if len(m.filtered) != 2 {
			t.Fatalf("applyFilter with ~ query should return all entries, got %d", len(m.filtered))
		}
	})

	t.Run("cursor resets when exceeding totalItems", func(t *testing.T) {
		m := DirPickerModel{
			entries: []string{"alpha", "beta", "gamma"},
			cursor:  5,
		}
		m.filterInput.SetValue("alpha")
		m.applyFilter()

		if m.cursor != 0 {
			t.Errorf("cursor should reset to 0 when exceeding totalItems, got %d", m.cursor)
		}
	})
}

// --- DirPickerModel.loadLocalEntries ---

func TestDirPickerModel_LoadLocalEntries(t *testing.T) {
	t.Run("loads subdirectories from temp dir", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create test directories
		dirs := []string{"alpha", "beta", "gamma"}
		for _, d := range dirs {
			if err := os.Mkdir(filepath.Join(tmpDir, d), 0o755); err != nil {
				t.Fatalf("failed to create dir %q: %v", d, err)
			}
		}
		// Create a file (should be excluded)
		if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("test"), 0o644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		m := DirPickerModel{
			currentDir: tmpDir,
		}
		m.loadLocalEntries()

		if len(m.entries) != 3 {
			t.Fatalf("loadLocalEntries() returned %d entries, want 3: %v", len(m.entries), m.entries)
		}
		// Entries should be sorted
		if m.entries[0] != "alpha" || m.entries[1] != "beta" || m.entries[2] != "gamma" {
			t.Errorf("entries should be sorted, got %v", m.entries)
		}
		// Filtered should also be populated
		if len(m.filtered) != 3 {
			t.Errorf("filtered should have 3 entries, got %d", len(m.filtered))
		}
	})

	t.Run("hidden directories excluded by default", func(t *testing.T) {
		tmpDir := t.TempDir()

		_ = os.Mkdir(filepath.Join(tmpDir, "visible"), 0o755)
		_ = os.Mkdir(filepath.Join(tmpDir, ".hidden"), 0o755)

		m := DirPickerModel{
			currentDir: tmpDir,
			showHidden: false,
		}
		m.loadLocalEntries()

		if len(m.entries) != 1 {
			t.Fatalf("loadLocalEntries() should exclude hidden, got %d entries: %v", len(m.entries), m.entries)
		}
		if m.entries[0] != "visible" {
			t.Errorf("expected 'visible', got %q", m.entries[0])
		}
	})

	t.Run("hidden directories included when showHidden is true", func(t *testing.T) {
		tmpDir := t.TempDir()

		_ = os.Mkdir(filepath.Join(tmpDir, "visible"), 0o755)
		_ = os.Mkdir(filepath.Join(tmpDir, ".hidden"), 0o755)

		m := DirPickerModel{
			currentDir: tmpDir,
			showHidden: true,
		}
		m.loadLocalEntries()

		if len(m.entries) != 2 {
			t.Fatalf("loadLocalEntries() with showHidden should include hidden, got %d entries: %v", len(m.entries), m.entries)
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		m := DirPickerModel{
			currentDir: tmpDir,
		}
		m.loadLocalEntries()

		if len(m.entries) != 0 {
			t.Errorf("loadLocalEntries() on empty dir should return 0 entries, got %d", len(m.entries))
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		m := DirPickerModel{
			currentDir: "/nonexistent/path/that/does/not/exist",
		}
		m.loadLocalEntries()

		if m.entries != nil {
			t.Errorf("loadLocalEntries() on nonexistent dir should return nil, got %v", m.entries)
		}
	})
}

// --- DirPickerModel.adjustScroll ---

func TestDirPickerModel_AdjustScroll(t *testing.T) {
	t.Run("cursor within viewport does not scroll", func(t *testing.T) {
		m := DirPickerModel{
			height: 18, // visibleLines = 18 - 8 = 10
			cursor: 3,
			offset: 0,
		}
		m.adjustScroll()

		if m.offset != 0 {
			t.Errorf("offset should remain 0 when cursor is within viewport, got %d", m.offset)
		}
	})

	t.Run("cursor beyond viewport scrolls down", func(t *testing.T) {
		m := DirPickerModel{
			height: 18, // visibleLines = 10
			cursor: 12,
			offset: 0,
		}
		m.adjustScroll()

		// cursor(12) >= offset(0) + visibleLines(10), so offset = 12 - 10 + 1 = 3
		if m.offset != 3 {
			t.Errorf("offset should be 3 when cursor=12 exceeds viewport of 10, got %d", m.offset)
		}
	})

	t.Run("cursor before offset scrolls up", func(t *testing.T) {
		m := DirPickerModel{
			height: 18, // visibleLines = 10
			cursor: 2,
			offset: 5,
		}
		m.adjustScroll()

		// cursor(2) < offset(5), so offset = cursor = 2
		if m.offset != 2 {
			t.Errorf("offset should be 2 when cursor=2 is before offset=5, got %d", m.offset)
		}
	})

	t.Run("small height uses default visibleLines of 10", func(t *testing.T) {
		m := DirPickerModel{
			height: 5, // visibleLines = 5 - 8 = -3, clamped to 10
			cursor: 15,
			offset: 0,
		}
		m.adjustScroll()

		// visibleLines defaults to 10, cursor(15) >= offset(0)+10, so offset = 15 - 10 + 1 = 6
		if m.offset != 6 {
			t.Errorf("offset should be 6 with small height (default visibleLines=10), got %d", m.offset)
		}
	})

	t.Run("cursor at last visible line does not scroll", func(t *testing.T) {
		m := DirPickerModel{
			height: 18, // visibleLines = 10
			cursor: 9,
			offset: 0,
		}
		m.adjustScroll()

		// cursor(9) < offset(0)+visibleLines(10) → no scroll needed
		if m.offset != 0 {
			t.Errorf("offset should remain 0 when cursor is at last visible line, got %d", m.offset)
		}
	})

	t.Run("cursor exactly at boundary triggers scroll", func(t *testing.T) {
		m := DirPickerModel{
			height: 18, // visibleLines = 10
			cursor: 10,
			offset: 0,
		}
		m.adjustScroll()

		// cursor(10) >= offset(0)+visibleLines(10) → offset = 10 - 10 + 1 = 1
		if m.offset != 1 {
			t.Errorf("offset should be 1 when cursor=10 hits boundary, got %d", m.offset)
		}
	})
}

// --- NewDirPickerModel ---

func TestNewDirPickerModel(t *testing.T) {
	t.Run("empty startDir defaults to home directory", func(t *testing.T) {
		m := NewDirPickerModel("")

		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("cannot get home dir: %v", err)
		}
		if m.currentDir != home {
			t.Errorf("NewDirPickerModel(\"\").currentDir = %q, want %q", m.currentDir, home)
		}
	})

	t.Run("absolute path is used directly", func(t *testing.T) {
		tmpDir := t.TempDir()
		m := NewDirPickerModel(tmpDir)

		if m.currentDir != tmpDir {
			t.Errorf("NewDirPickerModel(%q).currentDir = %q, want %q", tmpDir, m.currentDir, tmpDir)
		}
	})

	t.Run("tilde path is expanded", func(t *testing.T) {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("cannot get home dir: %v", err)
		}
		m := NewDirPickerModel("~/")

		if m.currentDir != home {
			t.Errorf("NewDirPickerModel(\"~/\").currentDir = %q, want %q", m.currentDir, home)
		}
	})

	t.Run("defaults are reasonable", func(t *testing.T) {
		m := NewDirPickerModel("")

		if m.selected {
			t.Error("selected should be false by default")
		}
		if m.result != "" {
			t.Errorf("result should be empty by default, got %q", m.result)
		}
		if m.cursor != 0 {
			t.Errorf("cursor should be 0 by default, got %d", m.cursor)
		}
		if m.offset != 0 {
			t.Errorf("offset should be 0 by default, got %d", m.offset)
		}
		if m.hostConfig != nil {
			t.Error("hostConfig should be nil by default")
		}
		if m.loading {
			t.Error("loading should be false by default")
		}
		if m.showHidden {
			t.Error("showHidden should be false by default")
		}
	})
}

// --- DirPickerModel.SetRemoteHost ---

func TestDirPickerModel_SetRemoteHost(t *testing.T) {
	t.Run("sets SSH host config", func(t *testing.T) {
		m := NewDirPickerModel("")
		hc := &config.HostConfig{
			ID:   "remote-dev",
			Type: "ssh",
			Host: "dev.example.com",
		}

		cmd := m.SetRemoteHost(hc)

		if m.hostConfig == nil {
			t.Fatal("hostConfig should be set after SetRemoteHost")
		}
		if m.hostConfig.ID != "remote-dev" {
			t.Errorf("hostConfig.ID = %q, want %q", m.hostConfig.ID, "remote-dev")
		}
		if !m.IsRemote() {
			t.Error("IsRemote() should return true after SetRemoteHost")
		}
		if !m.loading {
			t.Error("loading should be true after SetRemoteHost")
		}
		if m.currentDir != "/home" {
			t.Errorf("currentDir should be reset to /home, got %q", m.currentDir)
		}
		if m.cursor != 0 {
			t.Errorf("cursor should be reset to 0, got %d", m.cursor)
		}
		if m.offset != 0 {
			t.Errorf("offset should be reset to 0, got %d", m.offset)
		}
		if m.entries != nil {
			t.Errorf("entries should be nil, got %v", m.entries)
		}
		if m.filtered != nil {
			t.Errorf("filtered should be nil, got %v", m.filtered)
		}
		if cmd == nil {
			t.Error("SetRemoteHost should return a non-nil tea.Cmd for SSH host")
		}
	})

	t.Run("nil hostConfig is no-op", func(t *testing.T) {
		m := NewDirPickerModel("")
		cmd := m.SetRemoteHost(nil)

		if m.hostConfig != nil {
			t.Error("hostConfig should remain nil for nil input")
		}
		if cmd != nil {
			t.Error("SetRemoteHost(nil) should return nil cmd")
		}
	})

	t.Run("non-SSH type is no-op", func(t *testing.T) {
		m := NewDirPickerModel("")
		hc := &config.HostConfig{
			ID:        "docker-dev",
			Type:      "docker",
			Container: "my-container",
		}
		cmd := m.SetRemoteHost(hc)

		if m.hostConfig != nil {
			t.Error("hostConfig should remain nil for non-SSH type")
		}
		if cmd != nil {
			t.Error("SetRemoteHost with non-SSH type should return nil cmd")
		}
	})

	t.Run("clears previous state", func(t *testing.T) {
		m := DirPickerModel{
			currentDir: "/some/old/dir",
			cursor:     10,
			offset:     5,
			entries:    []string{"old1", "old2"},
			filtered:   []string{"old1"},
		}
		hc := &config.HostConfig{
			ID:   "new-host",
			Type: "ssh",
			Host: "new.example.com",
		}

		m.SetRemoteHost(hc)

		if m.currentDir != "/home" {
			t.Errorf("currentDir should be reset to /home, got %q", m.currentDir)
		}
		if m.cursor != 0 {
			t.Errorf("cursor should be reset to 0, got %d", m.cursor)
		}
		if m.offset != 0 {
			t.Errorf("offset should be reset to 0, got %d", m.offset)
		}
		if m.entries != nil {
			t.Errorf("entries should be nil, got %v", m.entries)
		}
		if m.filtered != nil {
			t.Errorf("filtered should be nil, got %v", m.filtered)
		}
	})
}

// --- Left/Right section navigation ---

func sendKey(m DirPickerModel, key string) DirPickerModel {
	var msg tea.KeyMsg
	switch key {
	case "left":
		msg = tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		msg = tea.KeyMsg{Type: tea.KeyRight}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	updated, _ := m.Update(msg)
	return updated
}

func TestDirPickerModel_LeftRightNavigation(t *testing.T) {
	history := []HistoryEntry{
		{Path: "/home/user/a", DisplayPath: "~/a"},
		{Path: "/home/user/b", DisplayPath: "~/b"},
	}
	dirs := []string{"dir1", "dir2", "dir3"}

	t.Run("left jumps to top of history from directory section", func(t *testing.T) {
		m := DirPickerModel{
			filteredHistory: history,
			filtered:        dirs,
			cursor:          len(history) + 1, // inside directory section
		}
		m = sendKey(m, "left")
		if m.cursor != 0 {
			t.Errorf("cursor should be 0 after left key, got %d", m.cursor)
		}
	})

	t.Run("left jumps to top of history when already in history", func(t *testing.T) {
		m := DirPickerModel{
			filteredHistory: history,
			filtered:        dirs,
			cursor:          1, // second history entry
		}
		m = sendKey(m, "left")
		if m.cursor != 0 {
			t.Errorf("cursor should be 0 after left key, got %d", m.cursor)
		}
	})

	t.Run("right jumps to top of directory section from history", func(t *testing.T) {
		m := DirPickerModel{
			filteredHistory: history,
			filtered:        dirs,
			cursor:          0, // first history entry
		}
		m = sendKey(m, "right")
		if m.cursor != len(history) {
			t.Errorf("cursor should be %d after right key, got %d", len(history), m.cursor)
		}
	})

	t.Run("right jumps to top of directory section when already in directories", func(t *testing.T) {
		m := DirPickerModel{
			filteredHistory: history,
			filtered:        dirs,
			cursor:          len(history) + 2, // third directory entry
		}
		m = sendKey(m, "right")
		if m.cursor != len(history) {
			t.Errorf("cursor should be %d after right key, got %d", len(history), m.cursor)
		}
	})

	t.Run("left is no-op when history is empty", func(t *testing.T) {
		m := DirPickerModel{
			filteredHistory: nil,
			filtered:        dirs,
			cursor:          1,
		}
		m = sendKey(m, "left")
		if m.cursor != 1 {
			t.Errorf("cursor should remain 1 when no history, got %d", m.cursor)
		}
	})

	t.Run("right is no-op when directories are empty", func(t *testing.T) {
		m := DirPickerModel{
			filteredHistory: history,
			filtered:        nil,
			cursor:          0,
		}
		m = sendKey(m, "right")
		if m.cursor != 0 {
			t.Errorf("cursor should remain 0 when no directories, got %d", m.cursor)
		}
	})
}
