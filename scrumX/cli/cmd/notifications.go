package cmd

import (
	"fmt"
	"time"

	"github.com/atharshah1/scrum/scrumX/cli/internal/utils"
	"github.com/spf13/cobra"
)

var notificationsCmd = &cobra.Command{Use: "notifications", Aliases: []string{"notif", "n"}, Short: "Notification commands"}

var notificationsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List notifications",
	Example: `
  scrumx notifications list --limit 20
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		page, _ := cmd.Flags().GetInt("page")
		limit, _ := cmd.Flags().GetInt("limit")
		items, err := client.ListNotifications(page, limit)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			fmt.Println("No notifications")
			return nil
		}
		rows := make([][]string, 0, len(items))
		for _, item := range items {
			read := "unread"
			if item.IsRead {
				read = "read"
			}
			rows = append(rows, []string{item.ID, read, item.Type, item.Title, item.CreatedAt.Local().Format(time.RFC3339)})
		}
		utils.PrintTable([]string{"ID", "STATE", "TYPE", "TITLE", "CREATED"}, rows)
		return nil
	},
}

var notificationsReadCmd = &cobra.Command{
	Use:   "read <notification-id>",
	Short: "Mark a notification as read",
	Args:  cobra.ExactArgs(1),
	Example: "  scrumx notifications read <notification-id>",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		if err := client.MarkNotificationRead(args[0]); err != nil {
			return err
		}
		fmt.Println(utils.SuccessText("Notification marked read"))
		return nil
	},
}

var notificationsReadAllCmd = &cobra.Command{
	Use:   "read-all",
	Short: "Mark all notifications as read",
	Example: "  scrumx notifications read-all",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		if err := client.MarkAllNotificationsRead(); err != nil {
			return err
		}
		fmt.Println(utils.SuccessText("All notifications marked read"))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(notificationsCmd)
	notificationsCmd.AddCommand(notificationsListCmd, notificationsReadCmd, notificationsReadAllCmd)
	notificationsListCmd.Flags().Int("page", 1, "Page number")
	notificationsListCmd.Flags().Int("limit", 20, "Page size")
}
