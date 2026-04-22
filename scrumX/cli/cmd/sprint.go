package cmd

import (
	"fmt"
	"strings"

	"github.com/atharshah1/scrum/scrumX/cli/internal/utils"
	"github.com/spf13/cobra"
)

var sprintCmd = &cobra.Command{
	Use:     "sprint",
	Aliases: []string{"s"},
	Short:   "Sprint commands",
	Example: strings.TrimSpace(`
  scrumx sprint start <sprint-id>
  sx s s <sprint-id>
  scrumx s -s <sprint-id>
`),
	RunE: runSprintRoot,
}

var sprintStartCmd = &cobra.Command{
	Use:     "start <sprint-id>",
	Aliases: []string{"s"},
	Short:   "Start a sprint",
	Args:    cobra.ExactArgs(1),
	RunE:    runSprintStart,
}

var sprintEndCmd = &cobra.Command{
	Use:     "end <sprint-id>",
	Aliases: []string{"e"},
	Short:   "End a sprint",
	Args:    cobra.ExactArgs(1),
	RunE:    runSprintEnd,
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
	sprintCmd.Flags().StringP("start", "s", "", "Start a sprint from the sprint root")
	sprintCmd.Flags().StringP("end", "e", "", "End a sprint from the sprint root")
}

func runSprintRoot(cmd *cobra.Command, args []string) error {
	startChanged := cmd.Flags().Changed("start")
	endChanged := cmd.Flags().Changed("end")
	if startChanged && endChanged {
		return fmt.Errorf("choose one sprint root shortcut: --start/-s or --end/-e")
	}
	if !startChanged && !endChanged {
		return cmd.Help()
	}
	if startChanged {
		id := strings.TrimSpace(mustString(cmd, "start"))
		if id == "" {
			return fmt.Errorf("sprint id is required for --start/-s")
		}
		if len(args) > 0 {
			return fmt.Errorf("unexpected args for sprint start shortcut: %s", strings.Join(args, " "))
		}
		return runSprintStart(cmd, []string{id})
	}

	id := strings.TrimSpace(mustString(cmd, "end"))
	if id == "" {
		return fmt.Errorf("sprint id is required for --end/-e")
	}
	if len(args) > 0 {
		return fmt.Errorf("unexpected args for sprint end shortcut: %s", strings.Join(args, " "))
	}
	return runSprintEnd(cmd, []string{id})
}

func runSprintStart(cmd *cobra.Command, args []string) error {
	client, err := newClient()
	if err != nil {
		return err
	}
	if err := client.StartSprint(args[0]); err != nil {
		return err
	}
	fmt.Println(utils.SuccessText("Sprint started"))
	return nil
}

func runSprintEnd(cmd *cobra.Command, args []string) error {
	client, err := newClient()
	if err != nil {
		return err
	}
	if err := client.EndSprint(args[0]); err != nil {
		return err
	}
	fmt.Println(utils.SuccessText("Sprint ended"))
	return nil
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
