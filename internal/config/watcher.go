package config

import (
	"errors"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// DefaultWatchDebounce is the delay applied between the last observed
// filesystem event and the reload/notify step. Editors that save-all can
// emit CREATE+WRITE (or RENAME+CREATE for atomic writes) in a burst; the
// debounce coalesces those into a single reload.
const DefaultWatchDebounce = 250 * time.Millisecond

// Watcher observes a Manager's config file and reloads it in-place on
// change; consumers see a signal on Events() per successful reload. A single
// background goroutine owns the debounce timer, Reload calls, and the
// events publish, so a stopped-but-already-fired timer racing a re-arm from
// a fresh event cannot double-reload. Callers MUST invoke Close (idempotent).
type Watcher struct {
	mgr      *Manager
	fsw      *fsnotify.Watcher
	events   chan struct{}
	done     chan struct{}
	debounce time.Duration
	closeOne sync.Once
	baseName string
}

// NewWatcher wires an fsnotify watcher to mgr's config file. Watches the
// parent directory (not the file directly) so create-after-startup and
// atomic-save-via-rename both surface as events. Returns an error if fsnotify
// is unavailable on this filesystem; callers should log and continue without
// hot-reload rather than fail startup.
//
// A non-positive debounce falls back to DefaultWatchDebounce.
func NewWatcher(mgr *Manager, debounce time.Duration) (*Watcher, error) {
	if mgr == nil {
		return nil, errors.New("config watcher: manager is nil")
	}
	if debounce <= 0 {
		debounce = DefaultWatchDebounce
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	path := mgr.filePath
	dir := filepath.Dir(path)
	if err := fsw.Add(dir); err != nil {
		_ = fsw.Close()
		return nil, err
	}

	w := &Watcher{
		mgr:      mgr,
		fsw:      fsw,
		events:   make(chan struct{}, 1),
		done:     make(chan struct{}),
		debounce: debounce,
		baseName: filepath.Base(path),
	}
	go w.run()
	return w, nil
}

// Events returns the reload-signal channel. Buffered at 1 so bursts coalesce
// into a single event (the latest config is already applied to mgr), and
// closed on Close so TUI Cmd goroutines parked on `<-Events()` unblock at
// shutdown instead of leaking.
func (w *Watcher) Events() <-chan struct{} {
	return w.events
}

func (w *Watcher) Close() error {
	w.closeOne.Do(func() {
		close(w.done)
		_ = w.fsw.Close()
	})
	return nil
}

// run is the single owner of the debounce timer, mgr.Reload calls, and
// event publishes. Keeping all three in one goroutine eliminates the
// double-reload race a background time.AfterFunc worker would introduce
// when a stopped-but-already-fired timer bumps into a re-arm from a fresh
// event. Runs in its own goroutine for the life of the Watcher.
func (w *Watcher) run() {
	// Closing events on the way out unblocks any waitForConfigReload Cmd
	// parked in the TUI, so `jin ui` shuts down cleanly instead of leaking
	// a goroutine per Cmd chain.
	defer close(w.events)

	var timer *time.Timer
	// timerC stays nil (blocks forever in select) when no debounce is
	// pending, so the loop doesn't spin when there's nothing to fire.
	var timerC <-chan time.Time

	for {
		select {
		case <-w.done:
			if timer != nil {
				timer.Stop()
			}
			return

		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if !w.matches(ev) {
				continue
			}
			if timer == nil {
				timer = time.NewTimer(w.debounce)
				timerC = timer.C
			} else {
				// Drain a race between Stop() and the timer firing: if
				// Stop returned false the value is already sitting in
				// timer.C waiting to be consumed by our select.
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(w.debounce)
			}

		case <-timerC:
			timer = nil
			timerC = nil
			if err := w.mgr.Reload(); err != nil {
				log.Printf("config watcher: reload failed, keeping previous config: %v", err)
				continue
			}
			select {
			case w.events <- struct{}{}:
			default:
				// Consumer hasn't drained the last event yet; the
				// coalesced signal already represents "config is newer
				// than what you last saw".
			}

		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			// fsnotify errors are transient (queue overflows, permission
			// hiccups); log and keep watching. Losing an event only means
			// the user resaves.
			log.Printf("config watcher: fsnotify error: %v", err)
		}
	}
}

// matches reports whether ev targets the config file we care about and is a
// mutation likely to change file contents. REMOVE alone doesn't trigger a
// reload — the file will reappear via CREATE/RENAME, and reloading a missing
// file just re-logs the same "not found" from Viper.
func (w *Watcher) matches(ev fsnotify.Event) bool {
	if filepath.Base(ev.Name) != w.baseName {
		return false
	}
	return ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0
}
