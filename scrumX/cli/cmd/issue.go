package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/cli/internal/api"
	"github.com/atharshah1/scrum/scrumX/cli/internal/utils"
	"github.com/spf13/cobra"
)

var issueCmd = &cobra.Command{
	Use:     "issue",
	Aliases: []string{"issues", "is"},
	Short: "Issue management commands",
}

var issueCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a new issue",
	Args:  cobra.ExactArgs(1),
	Example: strings.TrimSpace(`
  scrumx issue create "Fix login bug" --project-id <project-id> --priority high --type bug
  scrumx issue create "Backend cleanup" --interactive
`),
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
	Example: strings.TrimSpace(`
  scrumx issue list
  scrumx issue list --status in_progress --assignee-id <user-id>
  scrumx issue list --sprint-id <sprint-id> --label backend --project-id <project-id>
`),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		filter := api.IssueListFilter{
			ProjectID:  mustString(cmd, "project-id"),
			Status:     mustString(cmd, "status"),
			AssigneeID: mustString(cmd, "assignee-id"),
			SprintID:   mustString(cmd, "sprint-id"),
			Label:      mustString(cmd, "label"),
			IssueType:  mustString(cmd, "type"),
			Query:      mustString(cmd, "query"),
			SortBy:     mustString(cmd, "sort-by"),
			Order:      mustString(cmd, "order"),
		}
		page, _ := cmd.Flags().GetInt("page")
		limit, _ := cmd.Flags().GetInt("limit")
		filter.Page = page
		filter.Limit = limit
		issues, err := client.ListIssues(filter)
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
	Example: strings.TrimSpace(`
  scrumx issue update <issue-id> --priority high --labels bug,customer
  scrumx issue update <issue-id> --title "New title" --description "Updated details"
`),
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
	Example: "  scrumx issue assign <issue-id> <user-id>",
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
	Example: strings.TrimSpace(`
  scrumx issue move <issue-id> done
  scrumx issue move <issue-id>
`),
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
var issueBulkCmd = &cobra.Command{Use: "bulk", Aliases: []string{"b"}, Short: "Bulk issue operations"}
var issueCommentCmd = &cobra.Command{Use: "comment", Aliases: []string{"comments"}, Short: "Manage issue comments"}

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

var issueBulkAssignCmd = &cobra.Command{
	Use:   "assign <user-id> --issues <id1,id2,...>",
	Short: "Bulk assign issues",
	Args:  cobra.ExactArgs(1),
	Example: `
  scrumx issue bulk assign <user-id> --issues <id1,id2,id3>
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		ids := parseCSV(mustString(cmd, "issues"))
		if len(ids) == 0 {
			return fmt.Errorf("--issues is required")
		}
		failed := 0
		for _, id := range ids {
			if _, err := client.UpdateIssue(id, api.UpdateIssueInput{AssigneeID: &args[0]}); err != nil {
				failed++
				fmt.Printf("✗ %s: %v\n", id, err)
				continue
			}
			fmt.Printf("✓ %s\n", id)
		}
		fmt.Println(utils.SuccessText(fmt.Sprintf("Bulk assign complete: %d succeeded, %d failed", len(ids)-failed, failed)))
		if failed > 0 {
			return fmt.Errorf("bulk assign completed with %d failures", failed)
		}
		return nil
	},
}

var issueBulkMoveCmd = &cobra.Command{
	Use:   "move <status> --issues <id1,id2,...>",
	Short: "Bulk move issues to a status",
	Args:  cobra.ExactArgs(1),
	Example: `
  scrumx issue bulk move done --issues <id1,id2,id3>
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		ids := parseCSV(mustString(cmd, "issues"))
		if len(ids) == 0 {
			return fmt.Errorf("--issues is required")
		}
		status := strings.ToLower(strings.TrimSpace(args[0]))
		failed := 0
		for _, id := range ids {
			if _, err := client.UpdateIssue(id, api.UpdateIssueInput{Status: &status}); err != nil {
				failed++
				fmt.Printf("✗ %s: %v\n", id, err)
				continue
			}
			fmt.Printf("✓ %s\n", id)
		}
		fmt.Println(utils.SuccessText(fmt.Sprintf("Bulk move complete: %d succeeded, %d failed", len(ids)-failed, failed)))
		if failed > 0 {
			return fmt.Errorf("bulk move completed with %d failures", failed)
		}
		return nil
	},
}

var issueBulkUpdateCmd = &cobra.Command{
	Use:   "update --issues <id1,id2,...> [--priority ..] [--type ..] [--status ..] [--assignee-id ..] [--sprint-id ..] [--labels ..]",
	Short: "Bulk update issue fields",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		ids := parseCSV(mustString(cmd, "issues"))
		if len(ids) == 0 {
			return fmt.Errorf("--issues is required")
		}
		input := api.UpdateIssueInput{}
		setCount := 0
		if cmd.Flags().Changed("priority") {
			v := strings.ToLower(mustString(cmd, "priority"))
			input.Priority = &v
			setCount++
		}
		if cmd.Flags().Changed("type") {
			v := strings.ToLower(mustString(cmd, "type"))
			input.IssueType = &v
			setCount++
		}
		if cmd.Flags().Changed("status") {
			v := strings.ToLower(mustString(cmd, "status"))
			input.Status = &v
			setCount++
		}
		if cmd.Flags().Changed("assignee-id") {
			v := mustString(cmd, "assignee-id")
			input.AssigneeID = &v
			setCount++
		}
		if cmd.Flags().Changed("sprint-id") {
			v := mustString(cmd, "sprint-id")
			input.SprintID = &v
			setCount++
		}
		if cmd.Flags().Changed("labels") {
			labels := parseCSV(mustString(cmd, "labels"))
			input.Labels = &labels
			setCount++
		}
		if setCount == 0 {
			return fmt.Errorf("no update fields provided")
		}
		failed := 0
		for _, id := range ids {
			if _, err := client.UpdateIssue(id, input); err != nil {
				failed++
				fmt.Printf("✗ %s: %v\n", id, err)
				continue
			}
			fmt.Printf("✓ %s\n", id)
		}
		fmt.Println(utils.SuccessText(fmt.Sprintf("Bulk update complete: %d succeeded, %d failed", len(ids)-failed, failed)))
		if failed > 0 {
			return fmt.Errorf("bulk update completed with %d failures", failed)
		}
		return nil
	},
}

var issueCommentAddCmd = &cobra.Command{
	Use:   "add <issue-id> --body <text>",
	Short: "Add a comment to an issue",
	Args:  cobra.ExactArgs(1),
	Example: `
  scrumx issue comment add <issue-id> --body "Looking into this now."
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		body := strings.TrimSpace(mustString(cmd, "body"))
		if body == "" {
			body, err = utils.Prompt("Comment body")
			if err != nil {
				return err
			}
		}
		comment, err := client.AddComment(args[0], body)
		if err != nil {
			return err
		}
		fmt.Println(utils.SuccessText(fmt.Sprintf("Added comment %s", comment.ID)))
		return nil
	},
}

var issueCommentListCmd = &cobra.Command{
	Use:   "list <issue-id>",
	Short: "List comments on an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		page, _ := cmd.Flags().GetInt("page")
		limit, _ := cmd.Flags().GetInt("limit")
		items, err := client.ListComments(args[0], page, limit)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			fmt.Println("No comments found")
			return nil
		}
		rows := make([][]string, 0, len(items))
		for _, item := range items {
			rows = append(rows, []string{item.ID, item.AuthorID, item.Body, item.CreatedAt.Local().Format(time.RFC3339)})
		}
		utils.PrintTable([]string{"ID", "AUTHOR", "BODY", "CREATED"}, rows)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(issueCmd)
	issueCmd.AddCommand(issueCreateCmd, issueListCmd, issueViewCmd, issueUpdateCmd, issueAssignCmd, issueMoveCmd, issueLabelCmd, issueBulkCmd, issueCommentCmd)
	issueLabelCmd.AddCommand(issueLabelAddCmd, issueLabelRemoveCmd)
	issueBulkCmd.AddCommand(issueBulkAssignCmd, issueBulkMoveCmd, issueBulkUpdateCmd)
	issueCommentCmd.AddCommand(issueCommentAddCmd, issueCommentListCmd)

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

	issueListCmd.Flags().String("project-id", "", "Filter by project UUID")
	issueListCmd.Flags().String("status", "", "Filter by status")
	issueListCmd.Flags().String("assignee-id", "", "Filter by assignee UUID")
	issueListCmd.Flags().String("sprint-id", "", "Filter by sprint UUID")
	issueListCmd.Flags().String("label", "", "Filter by label")
	issueListCmd.Flags().String("type", "", "Filter by issue type")
	issueListCmd.Flags().String("query", "", "Search in issue titles/descriptions")
	issueListCmd.Flags().String("sort-by", "", "Sort by field (created_at|updated_at|priority|status)")
	issueListCmd.Flags().String("order", "", "Sort order (asc|desc)")
	issueListCmd.Flags().Int("page", 1, "Page number")
	issueListCmd.Flags().Int("limit", 50, "Page size")

	issueBulkAssignCmd.Flags().String("issues", "", "Comma-separated issue IDs")
	_ = issueBulkAssignCmd.MarkFlagRequired("issues")

	issueBulkMoveCmd.Flags().String("issues", "", "Comma-separated issue IDs")
	_ = issueBulkMoveCmd.MarkFlagRequired("issues")

	issueBulkUpdateCmd.Flags().String("issues", "", "Comma-separated issue IDs")
	issueBulkUpdateCmd.Flags().String("priority", "", "New priority")
	issueBulkUpdateCmd.Flags().String("type", "", "New issue type")
	issueBulkUpdateCmd.Flags().String("status", "", "New status")
	issueBulkUpdateCmd.Flags().String("assignee-id", "", "New assignee UUID")
	issueBulkUpdateCmd.Flags().String("sprint-id", "", "New sprint UUID")
	issueBulkUpdateCmd.Flags().String("labels", "", "Replace labels with comma-separated values")
	_ = issueBulkUpdateCmd.MarkFlagRequired("issues")

	issueCommentAddCmd.Flags().String("body", "", "Comment text")
	issueCommentListCmd.Flags().Int("page", 1, "Page number")
	issueCommentListCmd.Flags().Int("limit", 20, "Page size")
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
