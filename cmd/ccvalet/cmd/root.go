package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/takaaki-s/claude-code-valet/internal/version"
)

var jsonOutput bool

var rootCmd = &cobra.Command{
	Use:     "ccvalet",
	Short:   "LLM session manager for Claude Code",
	Long:    `A CLI tool to manage multiple Claude Code sessions with attach/detach support.`,
	Version: version.Version,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.SetVersionTemplate("ccvalet " + version.Full() + "\n")
}
