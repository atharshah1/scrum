package cmd

import (
	"fmt"
	"strings"

	"github.com/atharshah1/scrum/scrumX/cli/internal/utils"
	"github.com/spf13/cobra"
)

var sprintCmd = &cobra.Command{Use: "sprint", Short: "Sprint commands"}

var sprintStartCmd = &cobra.Command{
	Use:   "start <sprint-id>",
	Short: "Start a sprint",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		if err := client.StartSprint(args[0]); err != nil {
			return err
		}
		fmt.Println(utils.SuccessText("Sprint started"))
		return nil
	},
}

var sprintEndCmd = &cobra.Command{
	Use:   "end <sprint-id>",
	Short: "End a sprint",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		if err := client.EndSprint(args[0]); err != nil {
			return err
		}
		fmt.Println(utils.SuccessText("Sprint ended"))
		return nil
	},
}

var sprintAddIssuesCmd = &cobra.Command{
	Use:   "add-issues <sprint-id> <issue-id[,issue-id...]>",
	Short: "Add issues to a sprint",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		issueIDs := parseSprintIssueIDs(args[1])
		if len(issueIDs) == 0 {
			return fmt.Errorf("provide at least one issue id")
		}
		if err := client.AddIssuesToSprint(args[0], issueIDs); err != nil {
			return err
		}
		fmt.Println(utils.SuccessText(fmt.Sprintf("Added %d issue(s) to sprint", len(issueIDs))))
		return nil
	},
}

var sprintRemoveIssuesCmd = &cobra.Command{
	Use:   "remove-issues <sprint-id> <issue-id[,issue-id...]>",
	Short: "Remove issues from a sprint",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		issueIDs := parseSprintIssueIDs(args[1])
		if len(issueIDs) == 0 {
			return fmt.Errorf("provide at least one issue id")
		}
		if err := client.RemoveIssuesFromSprint(args[0], issueIDs); err != nil {
			return err
		}
		fmt.Println(utils.SuccessText(fmt.Sprintf("Removed %d issue(s) from sprint", len(issueIDs))))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(sprintCmd)
	sprintCmd.AddCommand(sprintStartCmd, sprintEndCmd, sprintAddIssuesCmd, sprintRemoveIssuesCmd)
}

func parseSprintIssueIDs(in string) []string {
	parts := strings.Split(in, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		v := strings.TrimSpace(part)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
