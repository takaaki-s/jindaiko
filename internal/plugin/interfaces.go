package plugin

// Dispatcher is the seam session.Manager publishes events through. The
// concrete EventDispatcher fans events out to installed plugins; tests
// substitute a mock.
type Dispatcher interface {
	// Publish delivers ev to every runnable plugin whose manifest matches.
	// It never blocks the caller: matching and execution happen on
	// background goroutines and failures are logged, not returned.
	Publish(ev Event)
}
