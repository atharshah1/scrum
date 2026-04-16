package cmd

import (
	"fmt"
	"time"

	"github.com/atharshah1/scrum/scrumX/cli/internal/utils"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Offline sync commands",
}

var syncNowCmd = &cobra.Command{
	Use:   "now",
	Short: "Run push+pull sync now",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		if err := client.SyncNow(); err != nil {
			return err
		}
		fmt.Println(utils.SuccessText("Sync completed"))
		return nil
	},
}

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show offline/online/syncing state and queue status",
	RunE: func(cmd *cobra.Command, args []string) error {
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
		fmt.Printf(
			"Mode: %s\nPending operations: %d\nPending conflicts: %d\nDropped operations: %d\nLast synced at: %s\n",
			status.Mode,
			status.PendingOps,
			status.PendingConflicts,
			status.DroppedOps,
			last,
		)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.AddCommand(syncNowCmd, syncStatusCmd)
}
