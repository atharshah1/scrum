package cmd

import (
	"fmt"
	"time"

	"github.com/atharshah1/scrum/scrumX/cli/internal/utils"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Run or inspect sync state",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSyncNow()
	},
}

var syncNowCmd = &cobra.Command{
	Use:   "now",
	Short: "Run push+pull sync now",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSyncNow()
	},
}

var syncStatusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"st"},
	Short:   "Show sync trust status",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSyncStatus()
	},
}

var workspaceStatusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"st"},
	Short:   "Show workspace sync trust status",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSyncStatus()
	},
}

func init() {
	rootCmd.AddCommand(syncCmd, workspaceStatusCmd)
	syncCmd.AddCommand(syncNowCmd, syncStatusCmd)
}

func runSyncNow() error {
	client, err := newClient()
	if err != nil {
		return err
	}
	if err := client.SyncNow(); err != nil {
		return err
	}
	fmt.Println(utils.SuccessText("Sync completed"))
	return runSyncStatus()
}

func runSyncStatus() error {
	client, err := newClient()
	if err != nil {
		return err
	}
	status, err := client.SyncStatus()
	if err != nil {
		return err
	}
	last := "-"
	if !status.LastSyncedAt.IsZero() {
		last = status.LastSyncedAt.Local().Format(time.RFC3339)
	}
	summary := "✔ synced"
	switch {
	case status.PendingConflicts > 0:
		summary = fmt.Sprintf("⚠ %d conflict(s) need review", status.PendingConflicts)
	case status.PendingOps > 0:
		summary = fmt.Sprintf("⟳ %d pending change(s)", status.PendingOps)
	case status.Mode == "offline":
		summary = "⚑ working offline (safe)"
	case status.Mode == "syncing":
		summary = "⟳ syncing now"
	}
	fmt.Println(summary)
	fmt.Printf(
		"mode: %s\npending: %d\nconflicts: %d\ndropped: %d\nlast sync: %s\n",
		status.Mode,
		status.PendingOps,
		status.PendingConflicts,
		status.DroppedOps,
		last,
	)
	return nil
}
