// Package cmd provides the CLI entry point using cobra.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go-file-sync/pkg/core"
)

var (
	cfgFile string
	verbose bool
	dryRun  bool
)

// rootCmd represents the base command.
var rootCmd = &cobra.Command{
	Use:   "go-file-sync",
	Short: "A lightweight file/directory auto-sync tool",
	Long: `go-file-sync watches specified files and directories for changes
and automatically syncs them to target locations. It supports:
  - Concurrent file watching with goroutines
  - Nested conf file configuration with inheritance
  - Debouncing and filtering of events
  - Parallel sync with worker pool
  - Web dashboard, metrics, triggers, and more`,
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := core.New(cfgFile)
		if err != nil {
			return fmt.Errorf("initialize: %w", err)
		}
		return app.Run()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "sync.yaml", "Path to config file (sync.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "Show what would be synced without doing it")

	// Subcommands
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(versionCmd)
}

// runCmd starts the sync engine.
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the file sync engine (continuous watching)",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := core.New(cfgFile)
		if err != nil {
			return fmt.Errorf("initialize: %w", err)
		}
		if dryRun {
			app.SetDryRun(true)
		}
		return app.Run()
	},
}

// syncCmd performs a one-shot sync and exits.
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync all files once and exit (no continuous watching)",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := core.New(cfgFile)
		if err != nil {
			return fmt.Errorf("initialize: %w", err)
		}
		return app.RunOnce()
	},
}

// checkCmd validates the configuration.
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate the configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate config by loading it
		_, err := core.New(cfgFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Configuration check FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Configuration check PASSED")
		return nil
	},
}

// statusCmd displays sync status and metrics.
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display sync status and metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := core.New(cfgFile)
		if err != nil {
			return fmt.Errorf("initialize: %w", err)
		}
		defer app.Close()
		fmt.Print(app.MetricsReport())
		return nil
	},
}

// versionCmd prints the version.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("go-file-sync v0.3.0")
	},
}
