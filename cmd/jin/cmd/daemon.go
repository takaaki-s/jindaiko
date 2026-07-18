package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/takaaki-s/jind-ai/internal/daemon"
	"github.com/takaaki-s/jind-ai/internal/paths"
)

type daemonStatusResult struct {
	Running bool `json:"running"`
}

type daemonRestartResult struct {
	Stopped bool `json:"stopped"`
	Started bool `json:"started"`
	PID     int  `json:"pid,omitempty"`
}

var socketPathFlag string

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start the daemon server",
	Long:  `Start the jin daemon server. This must be running for session management.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if daemon is already running
		client := daemon.NewClient(getSocketPath())
		if client.IsRunning() {
			return fmt.Errorf("daemon is already running. Use 'jin daemon stop' to stop it first")
		}

		server, err := daemon.NewServer(getSocketPath(), getDataDir(), getConfigDir(), getStateDir())
		if err != nil {
			return err
		}

		return server.Start()
	},
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon in the background",
	RunE: func(cmd *cobra.Command, args []string) error {
		started, pid, err := startDaemonInBackground()
		if err != nil {
			return err
		}
		if !started {
			fmt.Println("Daemon is already running")
			return nil
		}
		fmt.Printf("Daemon started (PID: %d)\n", pid)
		return nil
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check daemon status",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := daemon.NewClient(getSocketPath())
		running := client.IsRunning()
		if jsonOutput {
			return renderDaemonStatusJSON(os.Stdout, daemonStatusResult{Running: running})
		}
		if running {
			fmt.Println("Daemon is running")
		} else {
			fmt.Println("Daemon is not running")
		}
		return nil
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		stopped, err := stopDaemonIfRunning()
		if err != nil {
			return err
		}
		if !stopped {
			fmt.Println("Daemon is not running")
			return nil
		}
		fmt.Println("Daemon stopped")
		return nil
	},
}

var daemonRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the daemon",
	Long: `Restart the jin daemon: stop the running instance (if any), then start a
fresh one in the background. If no daemon is running, this is equivalent to
'jin daemon start'.

Useful when swapping in an upgraded jin binary (e.g., after a protocol version
mismatch) or when configuration changes need to be re-read.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		stopped, err := stopDaemonIfRunning()
		if err != nil {
			return err
		}
		started, pid, err := startDaemonInBackground()
		if err != nil {
			return err
		}
		if jsonOutput {
			return renderDaemonRestartJSON(os.Stdout, daemonRestartResult{
				Stopped: stopped,
				Started: started,
				PID:     pid,
			})
		}
		if stopped {
			fmt.Println("Daemon stopped")
		}
		if started {
			fmt.Printf("Daemon started (PID: %d)\n", pid)
		} else {
			// Race: something else started a daemon between our stop and start.
			// Rare, but report it plainly instead of pretending we started one.
			fmt.Println("Daemon is already running")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonRestartCmd)

	daemonCmd.PersistentFlags().StringVar(&socketPathFlag, "socket", "", "custom socket path")
}

func renderDaemonStatusJSON(w io.Writer, result daemonStatusResult) error {
	return writeJSON(w, result)
}

func renderDaemonRestartJSON(w io.Writer, result daemonRestartResult) error {
	return writeJSON(w, result)
}

// stopDaemonIfRunning stops the daemon if it is currently running. The bool
// indicates whether a running daemon was actually stopped (false when nothing
// was running).
func stopDaemonIfRunning() (bool, error) {
	client := daemon.NewClient(getSocketPath())
	if !client.IsRunning() {
		return false, nil
	}
	if err := client.Stop(); err != nil {
		return false, err
	}
	return true, nil
}

// startDaemonInBackground launches `jin daemon` as a detached background
// process. The bool indicates whether a new process was started (false when a
// daemon was already running).
func startDaemonInBackground() (bool, int, error) {
	client := daemon.NewClient(getSocketPath())
	if client.IsRunning() {
		return false, 0, nil
	}

	exe, _ := os.Executable()
	daemonArgs := []string{"daemon"}
	if socketPathFlag != "" {
		daemonArgs = append(daemonArgs, "--socket", socketPathFlag)
	}
	bgCmd := exec.Command(exe, daemonArgs...)
	bgCmd.Env = os.Environ() // Inherit environment variables
	bgCmd.Stdout = nil
	bgCmd.Stderr = nil
	bgCmd.Stdin = nil

	if err := bgCmd.Start(); err != nil {
		return false, 0, err
	}
	return true, bgCmd.Process.Pid, nil
}

func getSocketPath() string {
	if socketPathFlag != "" {
		return socketPathFlag
	}
	if s := os.Getenv("JIN_SOCKET"); s != "" {
		return s
	}
	return paths.Socket()
}
