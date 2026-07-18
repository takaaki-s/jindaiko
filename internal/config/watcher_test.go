package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// writeConfig writes YAML to a manager's config file, forcing a mod time
// difference so fsnotify sees a WRITE even if the process wrote the same
// bytes recently.
func writeConfig(t *testing.T, path, yaml string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// waitEvent drains one signal from ev with a bounded wait. Fails the test
// on timeout so a flaky watcher doesn't hang the suite.
func waitEvent(t *testing.T, ev <-chan struct{}, d time.Duration) {
	t.Helper()
	select {
	case <-ev:
	case <-time.After(d):
		t.Fatalf("timed out waiting for reload event after %v", d)
	}
}

// expectNoEvent asserts no reload signal arrives within d — used to verify
// debounce coalescing and parse-error suppression.
func expectNoEvent(t *testing.T, ev <-chan struct{}, d time.Duration) {
	t.Helper()
	select {
	case <-ev:
		t.Fatalf("unexpected reload event within %v", d)
	case <-time.After(d):
	}
}

func TestWatcher_ReloadsOnWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// Seed with an initial file so viper's first ReadInConfig succeeds.
	writeConfig(t, path, "keybindings:\n  up: [\"k\"]\n")

	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if got := mgr.GetKeybindings().Up; !reflect.DeepEqual(got, []string{"k"}) {
		t.Fatalf("initial Up = %v, want [k]", got)
	}

	w, err := NewWatcher(mgr, 40*time.Millisecond)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })

	writeConfig(t, path, "keybindings:\n  up: [\"K\", \"up\"]\n")
	waitEvent(t, w.Events(), 2*time.Second)

	if got := mgr.GetKeybindings().Up; !reflect.DeepEqual(got, []string{"K", "up"}) {
		t.Fatalf("after reload Up = %v, want [K up]", got)
	}
}

func TestWatcher_Debounces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	writeConfig(t, path, "keybindings:\n  up: [\"k\"]\n")

	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	w, err := NewWatcher(mgr, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })

	// Burst three writes well under the debounce window.
	for i, val := range []string{"a", "b", "c"} {
		writeConfig(t, path, "keybindings:\n  up: [\""+val+"\"]\n")
		if i < 2 {
			time.Sleep(30 * time.Millisecond)
		}
	}

	// Exactly one event should arrive after the debounce.
	waitEvent(t, w.Events(), 2*time.Second)
	expectNoEvent(t, w.Events(), 250*time.Millisecond)

	if got := mgr.GetKeybindings().Up; !reflect.DeepEqual(got, []string{"c"}) {
		t.Fatalf("after burst Up = %v, want [c] (last write wins)", got)
	}
}

func TestWatcher_IgnoresParseErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	writeConfig(t, path, "keybindings:\n  up: [\"k\"]\n")

	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	w, err := NewWatcher(mgr, 40*time.Millisecond)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })

	// Malformed YAML — reload should fail silently and keep the old value.
	writeConfig(t, path, "keybindings: [not-a-map\n")
	expectNoEvent(t, w.Events(), 500*time.Millisecond)

	if got := mgr.GetKeybindings().Up; !reflect.DeepEqual(got, []string{"k"}) {
		t.Fatalf("after parse error Up = %v, want [k] (unchanged)", got)
	}

	// A subsequent good write should still reload — the watcher must not
	// wedge itself after a parse failure.
	writeConfig(t, path, "keybindings:\n  up: [\"K\"]\n")
	waitEvent(t, w.Events(), 2*time.Second)
	if got := mgr.GetKeybindings().Up; !reflect.DeepEqual(got, []string{"K"}) {
		t.Fatalf("after recovery Up = %v, want [K]", got)
	}
}

func TestWatcher_CloseUnblocksEventsChannel(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, filepath.Join(dir, "config.yaml"), "")

	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	w, err := NewWatcher(mgr, 40*time.Millisecond)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}

	// A consumer parked on Events() must observe Close as a clean channel
	// shutdown — otherwise the TUI's Cmd goroutine leaks after `jin ui` exit.
	done := make(chan struct{})
	go func() {
		_, ok := <-w.Events()
		if ok {
			t.Errorf("Events() returned a value; want closed-channel zero value")
		}
		close(done)
	}()

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("consumer parked on Events() did not unblock within 2s of Close")
	}
}

func TestWatcher_CloseIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, filepath.Join(dir, "config.yaml"), "")

	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	w, err := NewWatcher(mgr, 0) // 0 → default debounce
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestWatcher_HandlesCreateAfterStartup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	// Manager loaded defaults since the file was absent.
	if got := mgr.GetKeybindings().Up; !reflect.DeepEqual(got, DefaultKeybindings().Up) {
		t.Fatalf("initial Up = %v, want default %v", got, DefaultKeybindings().Up)
	}

	w, err := NewWatcher(mgr, 40*time.Millisecond)
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })

	// Create the file for the first time — CREATE + WRITE both fire, the
	// debounce coalesces them.
	writeConfig(t, path, "keybindings:\n  up: [\"K\"]\n")
	waitEvent(t, w.Events(), 2*time.Second)
	if got := mgr.GetKeybindings().Up; !reflect.DeepEqual(got, []string{"K"}) {
		t.Fatalf("after create Up = %v, want [K]", got)
	}
}

func TestWatcher_NilManagerErrors(t *testing.T) {
	if _, err := NewWatcher(nil, 0); err == nil {
		t.Fatal("NewWatcher(nil) = nil error, want non-nil")
	}
}
