package cmd

import (
	"fmt"
	"strings"

	"github.com/atharshah1/scrum/scrumX/cli/internal/api"
	"github.com/atharshah1/scrum/scrumX/cli/internal/utils"
	"github.com/spf13/cobra"
)

var issueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Issue management commands",
}

var issueCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a new issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		cfg, err := cfgStore.Load()
		if err != nil {
			return err
		}
		interactive, _ := cmd.Flags().GetBool("interactive")
		projectID, _ := cmd.Flags().GetString("project-id")
		if strings.TrimSpace(projectID) == "" {
			projectID = cfg.CurrentProjectID
		}
		description, _ := cmd.Flags().GetString("description")
		issueType, _ := cmd.Flags().GetString("type")
		priority, _ := cmd.Flags().GetString("priority")
		parentID, _ := cmd.Flags().GetString("parent-id")
		sprintID, _ := cmd.Flags().GetString("sprint-id")
		assigneeID, _ := cmd.Flags().GetString("assignee-id")
		labelsFlag, _ := cmd.Flags().GetString("labels")

		if interactive {
			if strings.TrimSpace(projectID) == "" {
				projectID, err = utils.Prompt("Project ID")
				if err != nil {
					return err
				}
			}
			description, err = utils.PromptOptional("Description", description)
			if err != nil {
				return err
			}
			issueType, err = utils.PromptOptional("Type (epic|story|task|bug)", issueType)
			if err != nil {
				return err
			}
			priority, err = utils.PromptOptional("Priority", priority)
			if err != nil {
				return err
			}
			parentID, err = utils.PromptOptional("Parent ID", parentID)
			if err != nil {
				return err
			}
			sprintID, err = utils.PromptOptional("Sprint ID", sprintID)
			if err != nil {
				return err
			}
			assigneeID, err = utils.PromptOptional("Assignee ID", assigneeID)
			if err != nil {
				return err
			}
			labelsFlag, err = utils.PromptOptional("Labels (comma-separated)", labelsFlag)
			if err != nil {
				return err
			}
		}

		if strings.TrimSpace(projectID) == "" {
			return fmt.Errorf("--project-id is required (or set context current_project_id)")
		}
		issue, err := client.CreateIssue(api.CreateIssueInput{
			Title:       args[0],
			ProjectID:   projectID,
			Description: description,
			IssueType:   issueType,
			Priority:    priority,
			ParentID:    parentID,
			SprintID:    sprintID,
			AssigneeID:  assigneeID,
			Labels:      parseCSV(labelsFlag),
		})
		if err != nil {
			return err
		}
		fmt.Println(utils.SuccessText(fmt.Sprintf("Created issue %s: %s", issue.ID, issue.Title)))
		return nil
	},
}

var issueListCmd = &cobra.Command{
	Use:   "list",
	Short: "List issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		issues, err := client.ListIssues()
		if err != nil {
			return err
		}
		if len(issues) == 0 {
			fmt.Println("No issues found")
			return nil
		}
		rows := make([][]string, 0, len(issues))
		for _, it := range issues {
			rows = append(rows, []string{it.ID, utils.StatusColor(it.Status), it.Priority, it.IssueType, it.Title})
		}
		utils.PrintTable([]string{"ID", "STATUS", "PRIORITY", "TYPE", "TITLE"}, rows)
		return nil
	},
}

var issueViewCmd = &cobra.Command{
	Use:   "view <issue-id>",
	Short: "View an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		issue, err := client.GetIssue(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("ID: %s\nTitle: %s\nStatus: %s\nPriority: %s\nType: %s\nProject: %s\n",
			issue.ID, issue.Title, issue.Status, issue.Priority, issue.IssueType, issue.ProjectID)
		if issue.ParentID != nil {
			fmt.Printf("Parent: %s\n", *issue.ParentID)
		}
		if issue.SprintID != nil {
			fmt.Printf("Sprint: %s\n", *issue.SprintID)
		}
		if issue.AssigneeID != nil {
			fmt.Printf("Assignee: %s\n", *issue.AssigneeID)
		}
		if len(issue.Labels) > 0 {
			fmt.Printf("Labels: %s\n", strings.Join(issue.Labels, ", "))
		}
		fmt.Printf("Description: %s\n", issue.Description)
		return nil
	},
}

var issueUpdateCmd = &cobra.Command{
	Use:   "update <issue-id>",
	Short: "Update issue fields",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		id := args[0]
		input := api.UpdateIssueInput{}
		setCount := 0

		if cmd.Flags().Changed("title") {
			v, _ := cmd.Flags().GetString("title")
			input.Title = &v
			setCount++
		}
		if cmd.Flags().Changed("description") {
			v, _ := cmd.Flags().GetString("description")
			input.Description = &v
			setCount++
		}
		if cmd.Flags().Changed("priority") {
			v, _ := cmd.Flags().GetString("priority")
			input.Priority = &v
			setCount++
		}
		if cmd.Flags().Changed("type") {
			v, _ := cmd.Flags().GetString("type")
			input.IssueType = &v
			setCount++
		}
		if cmd.Flags().Changed("status") {
			v, _ := cmd.Flags().GetString("status")
			input.Status = &v
			setCount++
		}
		if cmd.Flags().Changed("parent-id") {
			v, _ := cmd.Flags().GetString("parent-id")
			input.ParentID = &v
			setCount++
		}
		if cmd.Flags().Changed("sprint-id") {
			v, _ := cmd.Flags().GetString("sprint-id")
			input.SprintID = &v
			setCount++
		}
		if cmd.Flags().Changed("assignee-id") {
			v, _ := cmd.Flags().GetString("assignee-id")
			input.AssigneeID = &v
			setCount++
		}
		if cmd.Flags().Changed("labels") {
			labels := parseCSV(mustString(cmd, "labels"))
			input.Labels = &labels
			setCount++
		}
		if setCount == 0 {
			return fmt.Errorf("no changes provided")
		}
		issue, err := client.UpdateIssue(id, input)
		if err != nil {
			return err
		}
		fmt.Println(utils.SuccessText(fmt.Sprintf("Updated issue %s (%s)", issue.ID, issue.Status)))
		return nil
	},
}

var issueAssignCmd = &cobra.Command{
	Use:   "assign <issue-id> <user-id>",
	Short: "Assign an issue",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		issue, err := client.UpdateIssue(args[0], api.UpdateIssueInput{AssigneeID: &args[1]})
		if err != nil {
			return err
		}
		fmt.Println(utils.SuccessText(fmt.Sprintf("Assigned issue %s", issue.ID)))
		return nil
	},
}

var issueMoveCmd = &cobra.Command{
	Use:   "move <issue-id> [target-status]",
	Short: "Move issue through workflow transitions",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		issue, err := client.GetIssue(args[0])
		if err != nil {
			return err
		}
		allowed, err := client.ListAllowedTransitions(issue)
		if err != nil {
			return err
		}
		if len(allowed) == 0 {
			return fmt.Errorf("no workflow transitions available from %s", issue.Status)
		}
		target := ""
		if len(args) > 1 {
			target = strings.ToLower(strings.TrimSpace(args[1]))
		} else {
			target, err = utils.PromptSelect("Move issue to", allowed)
			if err != nil {
				return err
			}
			target = strings.ToLower(strings.TrimSpace(target))
		}
		if !contains(allowed, target) {
			return fmt.Errorf("invalid transition to %q (allowed: %s)", target, strings.Join(allowed, ", "))
		}
		updated, err := client.UpdateIssue(issue.ID, api.UpdateIssueInput{Status: &target})
		if err != nil {
			return err
		}
		fmt.Println(utils.SuccessText(fmt.Sprintf("Moved issue %s: %s -> %s", updated.ID, issue.Status, updated.Status)))
		return nil
	},
}

var issueLabelCmd = &cobra.Command{Use: "label", Short: "Manage issue labels"}

var issueLabelAddCmd = &cobra.Command{
	Use:   "add <issue-id> <label>",
	Short: "Add a label to an issue",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		if err := client.AddLabel(args[0], args[1]); err != nil {
			return err
		}
		fmt.Println(utils.SuccessText("Label added"))
		return nil
	},
}

var issueLabelRemoveCmd = &cobra.Command{
	Use:   "remove <issue-id> <label>",
	Short: "Remove a label from an issue",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		if err := client.RemoveLabel(args[0], args[1]); err != nil {
			return err
		}
		fmt.Println(utils.SuccessText("Label removed"))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(issueCmd)
	issueCmd.AddCommand(issueCreateCmd, issueListCmd, issueViewCmd, issueUpdateCmd, issueAssignCmd, issueMoveCmd, issueLabelCmd)
	issueLabelCmd.AddCommand(issueLabelAddCmd, issueLabelRemoveCmd)

	issueCreateCmd.Flags().String("project-id", "", "Project UUID (defaults to active context)")
	issueCreateCmd.Flags().String("description", "", "Issue description")
	issueCreateCmd.Flags().String("type", "task", "Issue type")
	issueCreateCmd.Flags().String("priority", "medium", "Issue priority")
	issueCreateCmd.Flags().String("parent-id", "", "Parent issue UUID")
	issueCreateCmd.Flags().String("sprint-id", "", "Sprint UUID")
	issueCreateCmd.Flags().String("assignee-id", "", "Assignee user UUID")
	issueCreateCmd.Flags().String("labels", "", "Comma-separated labels")
	issueCreateCmd.Flags().Bool("interactive", false, "Launch interactive creation wizard")

	issueUpdateCmd.Flags().String("title", "", "Issue title")
	issueUpdateCmd.Flags().String("description", "", "Issue description")
	issueUpdateCmd.Flags().String("priority", "", "Issue priority")
	issueUpdateCmd.Flags().String("type", "", "Issue type")
	issueUpdateCmd.Flags().String("status", "", "Issue status")
	issueUpdateCmd.Flags().String("parent-id", "", "Parent issue UUID")
	issueUpdateCmd.Flags().String("sprint-id", "", "Sprint UUID")
	issueUpdateCmd.Flags().String("assignee-id", "", "Assignee user UUID")
	issueUpdateCmd.Flags().String("labels", "", "Replace labels with comma-separated values")
}

func parseCSV(in string) []string {
	parts := strings.Split(in, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		v := strings.TrimSpace(part)
		if v == "" {
			continue
		}
		v = strings.ToLower(v)
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if strings.EqualFold(value, needle) {
			return true
		}
	}
	return false
}

func mustString(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}
