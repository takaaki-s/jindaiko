package notify

import (
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

// Notifier sends desktop notifications
type Notifier struct {
	enabled bool
	// Debounce to avoid notification spam
	lastNotify map[string]time.Time
	mu         sync.Mutex
	// Minimum interval between notifications for the same session
	debounceInterval time.Duration
	// Notification history
	history *History
}

// NewNotifier creates a new notifier
func NewNotifier() *Notifier {
	return &Notifier{
		enabled:          true,
		lastNotify:       make(map[string]time.Time),
		debounceInterval: 3 * time.Second,
		history:          NewHistory(100),
	}
}

// SetEnabled enables or disables notifications
func (n *Notifier) SetEnabled(enabled bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.enabled = enabled
}

// NotifyPermission sends a notification when permission is required
func (n *Notifier) NotifyPermission(sessionID, sessionName string) {
	msg := fmt.Sprintf("[%s] Claude is waiting for permission", sessionName)
	n.history.Add(Entry{
		SessionID:   sessionID,
		SessionName: sessionName,
		Type:        "permission",
		Message:     msg,
		Timestamp:   time.Now(),
	})
	n.notify(sessionID, "Permission Required", msg)
}

// NotifyTaskComplete sends a notification when a task is complete
func (n *Notifier) NotifyTaskComplete(sessionID, sessionName string) {
	msg := fmt.Sprintf("[%s] Claude has finished the task", sessionName)
	n.history.Add(Entry{
		SessionID:   sessionID,
		SessionName: sessionName,
		Type:        "task_complete",
		Message:     msg,
		Timestamp:   time.Now(),
	})
	n.notify(sessionID, "Task Complete", msg)
}

// NotificationHistory returns a copy of the notification history (newest first)
func (n *Notifier) NotificationHistory() []Entry {
	return n.history.List()
}

func (n *Notifier) notify(sessionID, title, message string) {
	n.mu.Lock()
	if !n.enabled {
		n.mu.Unlock()
		return
	}

	// Debounce: skip if we recently notified for this session
	key := sessionID + ":" + title
	if lastTime, ok := n.lastNotify[key]; ok {
		if time.Since(lastTime) < n.debounceInterval {
			n.mu.Unlock()
			return
		}
	}
	n.lastNotify[key] = time.Now()
	n.mu.Unlock()

	// Send local desktop notification asynchronously
	go func() { _ = sendDesktopNotification(title, message) }()
}

// SendDesktop sends a desktop notification with the given title and message.
// Used by the local daemon to relay notifications from remote slaves.
func (n *Notifier) SendDesktop(title, message string) {
	go func() { _ = sendDesktopNotification(title, message) }()
}

// sendDesktopNotification sends a desktop notification using OS-specific methods
func sendDesktopNotification(title, message string) error {
	switch runtime.GOOS {
	case "darwin":
		return sendMacOSNotification(title, message)
	case "linux":
		return sendLinuxNotification(title, message)
	default:
		// Unsupported platform - silently ignore
		return nil
	}
}

// sendMacOSNotification sends a notification using osascript
func sendMacOSNotification(title, message string) error {
	script := fmt.Sprintf(`display notification %q with title %q sound name "default"`, message, title)
	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}

// sendLinuxNotification sends a notification using notify-send
func sendLinuxNotification(title, message string) error {
	cmd := exec.Command("notify-send", title, message)
	return cmd.Run()
}
