package tmux

// Runner defines the interface for tmux operations used by session.Manager.
// The concrete *Client satisfies this interface.
type Runner interface {
	HasSession(name string) bool
	KillSession(name string) error
	NewSessionWithCmdInDir(name string, width, height int, dir, cmd string) error
	RespawnPane(target, cmd string) error
	GetPaneID(sessionName string) (string, error)
	IsPaneDead(target string) bool
	TagManagedPane(paneID string) error
	SetupAutoCleanDeadPanes() error
	KillPane(paneID string) error
	GetPaneCurrentPath(target string) (string, error)
	SendKeys(target, keys string) error
	SendKeysLiteral(target, text string) error
	DisplayPopup(opts DisplayPopupOptions) error
	SplitWindow(target string, horizontal bool, percent int, shellCmd string) error
	CapturePane(target string, ansi bool) (string, error)
}
