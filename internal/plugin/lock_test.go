package plugin

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestLock_LoadEmpty(t *testing.T) {
	dir := t.TempDir()

	l, err := LoadLock(dir)
	if err != nil {
		t.Fatalf("LoadLock on empty dir: %v", err)
	}
	if got := l.All(); len(got) != 0 {
		t.Errorf("expected empty lock, got %v", got)
	}

	if _, err := os.Stat(filepath.Join(dir, lockFilename)); !os.IsNotExist(err) {
		t.Errorf("LoadLock should not create the file, stat err = %v", err)
	}
}

func TestLock_SetAndGet(t *testing.T) {
	dir := t.TempDir()
	l, err := LoadLock(dir)
	if err != nil {
		t.Fatalf("LoadLock: %v", err)
	}

	want := LockEntry{
		Source:      "github.com/owner/repo",
		Ref:         "v1.2.0",
		Commit:      "abc123",
		InstalledAt: time.Now().UTC(),
	}
	if err := l.Set("notifier", want); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, ok := l.Get("notifier")
	if !ok {
		t.Fatalf("Get after Set returned ok=false")
	}
	if got.Source != want.Source {
		t.Errorf("Source = %q, want %q", got.Source, want.Source)
	}
	if got.Ref != want.Ref {
		t.Errorf("Ref = %q, want %q", got.Ref, want.Ref)
	}
	if got.Commit != want.Commit {
		t.Errorf("Commit = %q, want %q", got.Commit, want.Commit)
	}
	if got.Linked {
		t.Error("Linked = true, want false")
	}
	if got.InstalledAt.IsZero() {
		t.Error("InstalledAt should be non-zero")
	}
}

func TestLock_Remove(t *testing.T) {
	dir := t.TempDir()
	l, err := LoadLock(dir)
	if err != nil {
		t.Fatalf("LoadLock: %v", err)
	}

	if err := l.Set("notifier", LockEntry{Source: "s", InstalledAt: time.Now().UTC()}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := l.Remove("notifier"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok := l.Get("notifier"); ok {
		t.Error("Get after Remove should return ok=false")
	}

	// Removing a missing entry should be a no-op, not an error.
	if err := l.Remove("notifier"); err != nil {
		t.Errorf("Remove on missing entry returned err: %v", err)
	}
}

func TestLock_Persistence(t *testing.T) {
	dir := t.TempDir()

	first, err := LoadLock(dir)
	if err != nil {
		t.Fatalf("LoadLock#1: %v", err)
	}
	if err := first.Set("notifier", LockEntry{Source: "github.com/owner/repo", Commit: "persist", InstalledAt: time.Now().UTC()}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	second, err := LoadLock(dir)
	if err != nil {
		t.Fatalf("LoadLock#2: %v", err)
	}
	entry, ok := second.Get("notifier")
	if !ok {
		t.Fatalf("second load did not see persisted entry")
	}
	if entry.Commit != "persist" {
		t.Errorf("Commit = %q, want %q", entry.Commit, "persist")
	}

	// The on-disk file should be YAML nested under the top-level plugins key.
	b, err := os.ReadFile(filepath.Join(dir, lockFilename))
	if err != nil {
		t.Fatalf("read lock file: %v", err)
	}
	var lf lockFile
	if err := yaml.Unmarshal(b, &lf); err != nil {
		t.Fatalf("unmarshal on-disk lock: %v", err)
	}
	if _, ok := lf.Plugins["notifier"]; !ok {
		t.Errorf("on-disk lock missing plugin %q, got %v", "notifier", lf.Plugins)
	}
}

// TestLock_ReadsExternalWrites simulates the daemon+CLI split: the daemon holds
// a long-lived *Lock, while `jin plugin install` writes to the same YAML file
// from a separate process. Get must observe the external write on the next
// call, not the map cached at Load time.
func TestLock_ReadsExternalWrites(t *testing.T) {
	dir := t.TempDir()
	l, err := LoadLock(dir)
	if err != nil {
		t.Fatalf("LoadLock: %v", err)
	}

	if _, ok := l.Get("notifier"); ok {
		t.Fatalf("Get before external write should return ok=false")
	}

	// Simulate a CLI process writing the lock directly.
	external := lockFile{Plugins: map[string]LockEntry{
		"notifier": {Source: "github.com/owner/repo", Commit: "external", InstalledAt: time.Now().UTC()},
	}}
	raw, err := yaml.Marshal(external)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, lockFilename), raw, 0o644); err != nil {
		t.Fatalf("write external lock: %v", err)
	}

	entry, ok := l.Get("notifier")
	if !ok {
		t.Fatalf("Get after external write returned ok=false; want to observe the new entry")
	}
	if entry.Commit != "external" {
		t.Errorf("Commit = %q, want %q", entry.Commit, "external")
	}
}

func TestLock_ConcurrentSet(t *testing.T) {
	dir := t.TempDir()
	l, err := LoadLock(dir)
	if err != nil {
		t.Fatalf("LoadLock: %v", err)
	}

	const workers = 8
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			name := "p" + strconv.Itoa(i)
			if err := l.Set(name, LockEntry{Source: "s" + strconv.Itoa(i), InstalledAt: time.Now().UTC()}); err != nil {
				t.Errorf("Set: %v", err)
			}
		}()
	}
	wg.Wait()

	// After all concurrent writers finish, both the in-memory map and the
	// on-disk file must contain every entry.
	if got := len(l.All()); got != workers {
		t.Errorf("in-memory entries = %d, want %d", got, workers)
	}

	reloaded, err := LoadLock(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := len(reloaded.All()); got != workers {
		t.Errorf("on-disk entries = %d, want %d", got, workers)
	}
}
