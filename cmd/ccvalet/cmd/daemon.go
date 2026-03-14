package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/takaaki-s/claude-code-valet/internal/config"
	"github.com/takaaki-s/claude-code-valet/internal/daemon"
)

type daemonStatusResult struct {
	Running bool `json:"running"`
}

var socketPathFlag string
var hostIDFlag string
var peerSocketFlag string
var peerIDFlag string

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start the daemon server",
	Long:  `Start the ccvalet daemon server. This must be running for session management.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if daemon is already running
		client := daemon.NewClient(getSocketPath())
		if client.IsRunning() {
			return fmt.Errorf("daemon is already running. Use 'ccvalet daemon stop' to stop it first")
		}

		hostID := resolveHostID()
		server, err := daemon.NewServer(getSocketPath(), getDataDir(), getConfigDir(), hostID)
		if err != nil {
			return err
		}

		// Register peer if --peer-socket and --peer-id are specified (slave mode)
		if (peerSocketFlag != "") != (peerIDFlag != "") {
			return fmt.Errorf("--peer-socket and --peer-id must be specified together")
		}
		if peerSocketFlag != "" && peerIDFlag != "" {
			peerClient := daemon.NewRemoteClient(peerSocketFlag, peerIDFlag)
			server.RegisterPeer(peerIDFlag, "ssh", peerClient)
		}

		return server.Start()
	},
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon in the background",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := daemon.NewClient(getSocketPath())
		if client.IsRunning() {
			fmt.Println("Daemon is already running")
			return nil
		}

		// Start daemon in background
		exe, _ := os.Executable()
		daemonArgs := []string{"daemon"}
		if socketPathFlag != "" {
			daemonArgs = append(daemonArgs, "--socket", socketPathFlag)
		}
		if hostIDFlag != "" {
			daemonArgs = append(daemonArgs, "--host-id", hostIDFlag)
		}
		if peerSocketFlag != "" {
			daemonArgs = append(daemonArgs, "--peer-socket", peerSocketFlag)
		}
		if peerIDFlag != "" {
			daemonArgs = append(daemonArgs, "--peer-id", peerIDFlag)
		}
		bgCmd := exec.Command(exe, daemonArgs...)
		bgCmd.Env = os.Environ() // Inherit environment variables
		bgCmd.Stdout = nil
		bgCmd.Stderr = nil
		bgCmd.Stdin = nil

		if err := bgCmd.Start(); err != nil {
			return err
		}

		fmt.Printf("Daemon started (PID: %d)\n", bgCmd.Process.Pid)
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
		client := daemon.NewClient(getSocketPath())
		if !client.IsRunning() {
			fmt.Println("Daemon is not running")
			return nil
		}

		if err := client.Stop(); err != nil {
			return err
		}
		// Wait for daemon to actually exit (handleStop calls os.Exit in a goroutine)
		for range 30 {
			if !client.IsRunning() {
				fmt.Println("Daemon stopped")
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
		fmt.Println("Daemon stopped")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonStopCmd)

	// --socket flag: specify custom socket path for slave mode
	daemonCmd.PersistentFlags().StringVar(&socketPathFlag, "socket", "", "custom socket path (for slave mode)")
	// --host-id flag: set this daemon's host ID for bidirectional routing
	daemonCmd.PersistentFlags().StringVar(&hostIDFlag, "host-id", "", "host ID for this daemon (default: config value or \"local\")")
	// --peer-socket / --peer-id flags: register a peer daemon (used by master to tell slave about reverse tunnel)
	daemonCmd.PersistentFlags().StringVar(&peerSocketFlag, "peer-socket", "", "peer daemon socket path (reverse tunnel)")
	daemonCmd.PersistentFlags().StringVar(&peerIDFlag, "peer-id", "", "peer daemon host ID")
}

func renderDaemonStatusJSON(w io.Writer, result daemonStatusResult) error {
	return writeJSON(w, result)
}

// resolveHostID determines the host ID with priority: flag > config > "local"
func resolveHostID() string {
	if hostIDFlag != "" {
		return hostIDFlag
	}
	// Try loading from config
	configDir := getConfigDir()
	if configMgr, err := config.NewManager(configDir); err == nil {
		if id := configMgr.GetHostID(); id != "" {
			return id
		}
	}
	return "local"
}

func getSocketPath() string {
	if socketPathFlag != "" {
		return socketPathFlag
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ccvalet", "run", "daemon.sock")
}
