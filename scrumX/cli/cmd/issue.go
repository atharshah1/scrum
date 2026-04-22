package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/cli/internal/api"
	"github.com/atharshah1/scrum/scrumX/cli/internal/utils"
	"github.com/spf13/cobra"
)

var issueCmd = &cobra.Command{
	Use:     "issues",
	Aliases: []string{"issue", "is", "i"},
	Short:   "Issue management commands",
	Example: strings.TrimSpace(`
  scrumx issues create "fix login bug p1 assign me #auth"
  sx i c "fix login bug p1 assign me #auth"
  scrumx i -c "fix login bug p1 assign me #auth"

  scrumx issues list --status open
  sx i l --status open
  scrumx i -l --status open
`),
	RunE: runIssueRoot,
}

var issueCreateCmd = &cobra.Command{
	Use:     "create <title>",
	Aliases: []string{"c"},
	Short:   "Create a new issue",
	Args:    cobra.ExactArgs(1),
	Example: strings.TrimSpace(`
  sx i c "fix login bug p1 assign me #auth"
  sx i c "Backend cleanup" --interactive
`),
	RunE: runIssueCreate,
}

var issueListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"l"},
	Short:   "List issues",
	Example: strings.TrimSpace(`
  sx i l
  sx i l --status in_progress --assignee-id <user-id>
  sx i l --sprint-id <sprint-id> --label backend --project-id <project-id>
`),
	RunE: runIssueList,
}

var issueSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search issues with filter syntax (AND/OR, field=value)",
	Args:  cobra.ExactArgs(1),
	Example: strings.TrimSpace(`
  sx i search "status=done AND assignee=me AND priority=high"
  sx i search "status=in_progress AND label=payments"
`),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		page, _ := cmd.Flags().GetInt("page")
		limit, _ := cmd.Flags().GetInt("limit")
		issues, err := client.SearchIssuesSmart(args[0], page, limit)
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
	Use:     "view <issue-id>",
	Aliases: []string{"v"},
	Short:   "View an issue",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		issue, err := client.GetIssueSmart(args[0])
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
	Use:     "update <issue-id>",
	Aliases: []string{"u"},
	Short:   "Update issue fields",
	Args:    cobra.ExactArgs(1),
	Example: strings.TrimSpace(`
  sx i u <issue-id> --priority high --labels bug,customer
  sx i u <issue-id> --title "New title" --description "Updated details"
`),
	RunE: runIssueUpdate,
}

var issueAssignCmd = &cobra.Command{
	Use:     "assign <issue-id> <user-id>",
	Short:   "Assign an issue",
	Args:    cobra.ExactArgs(2),
	Example: "  scrumx issues assign <issue-id> <user-id>",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		issue, err := client.UpdateIssueSmart(args[0], api.UpdateIssueInput{AssigneeID: &args[1]})
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
  scrumx issues move <issue-id> done
  scrumx issues move <issue-id>
`),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		issue, err := client.GetIssueSmart(args[0])
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
		updated, err := client.UpdateIssueSmart(issue.ID, api.UpdateIssueInput{Status: &target})
		if err != nil {
			return err
		}
		fmt.Println(utils.SuccessText(fmt.Sprintf("Moved issue %s: %s -> %s", updated.ID, issue.Status, updated.Status)))
		return nil
	},
}

var issueDeleteCmd = &cobra.Command{
	Use:     "delete <issue-id>",
	Aliases: []string{"d", "rm"},
	Short:   "Delete an issue (queues offline when disconnected)",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		if err := client.DeleteIssueSmart(args[0]); err != nil {
			return err
		}
		fmt.Println(utils.SuccessText(fmt.Sprintf("Deleted issue %s (or queued for sync)", args[0])))
		return nil
	},
}

var issueLabelCmd = &cobra.Command{Use: "label", Short: "Manage issue labels"}
var issueBulkCmd = &cobra.Command{Use: "bulk", Aliases: []string{"b"}, Short: "Bulk issue operations"}
var issueCommentCmd = &cobra.Command{Use: "comment", Aliases: []string{"comments"}, Short: "Manage issue comments"}
var issueFilterCmd = &cobra.Command{Use: "filter", Aliases: []string{"filters"}, Short: "Manage saved and recent issue search filters"}

var issueFilterSaveCmd = &cobra.Command{
	Use:   "save <name> <query>",
	Short: "Save an issue search query",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		saved, err := client.SaveIssueQuery(args[0], args[1])
		if err != nil {
			return err
		}
		fmt.Println(utils.SuccessText(fmt.Sprintf("Saved query %q", saved.Name)))
		return nil
	},
}

var issueFilterListCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved issue search queries",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		items, err := client.ListSavedIssueQueries()
		if err != nil {
			return err
		}
		if len(items) == 0 {
			fmt.Println("No saved filters")
			return nil
		}
		rows := make([][]string, 0, len(items))
		for _, item := range items {
			rows = append(rows, []string{item.ID, item.Name, item.Query, item.UpdatedAt.Local().Format(time.RFC3339)})
		}
		utils.PrintTable([]string{"ID", "NAME", "QUERY", "UPDATED"}, rows)
		return nil
	},
}

var issueFilterRunCmd = &cobra.Command{
	Use:   "run <name-or-id>",
	Short: "Run a saved issue search query by name or id",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		saved, err := client.ListSavedIssueQueries()
		if err != nil {
			return err
		}
		target := strings.TrimSpace(args[0])
		var selected *api.SavedIssueQuery
		for i := range saved {
			if saved[i].ID == target || strings.EqualFold(saved[i].Name, target) {
				selected = &saved[i]
				break
			}
		}
		if selected == nil {
			return fmt.Errorf("saved filter not found: %s", target)
		}
		page, _ := cmd.Flags().GetInt("page")
		limit, _ := cmd.Flags().GetInt("limit")
		issues, err := client.SearchIssuesSmart(selected.Query, page, limit)
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

var issueFilterDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a saved issue search query by id",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		if err := client.DeleteSavedIssueQuery(args[0]); err != nil {
			return err
		}
		fmt.Println(utils.SuccessText("Saved filter deleted"))
		return nil
	},
}

var issueFilterRecentCmd = &cobra.Command{
	Use:   "recent",
	Short: "List recent issue search queries",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		limit, _ := cmd.Flags().GetInt("limit")
		items, err := client.ListRecentIssueQueries(limit)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			fmt.Println("No recent filters")
			return nil
		}
		rows := make([][]string, 0, len(items))
		for _, item := range items {
			rows = append(rows, []string{item.Query, item.LastUsedAt.Local().Format(time.RFC3339)})
		}
		utils.PrintTable([]string{"QUERY", "LAST_USED"}, rows)
		return nil
	},
}

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
  scrumx issues bulk assign <user-id> --issues <id1,id2,id3>
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
			if _, err := client.UpdateIssueSmart(id, api.UpdateIssueInput{AssigneeID: &args[0]}); err != nil {
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
  scrumx issues bulk move done --issues <id1,id2,id3>
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
			if _, err := client.UpdateIssueSmart(id, api.UpdateIssueInput{Status: &status}); err != nil {
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
			if _, err := client.UpdateIssueSmart(id, input); err != nil {
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
  scrumx issues comment add <issue-id> --body "Looking into this now."
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
	issueCmd.AddCommand(issueCreateCmd, issueListCmd, issueSearchCmd, issueViewCmd, issueUpdateCmd, issueAssignCmd, issueMoveCmd, issueDeleteCmd, issueLabelCmd, issueBulkCmd, issueCommentCmd, issueFilterCmd)
	issueLabelCmd.AddCommand(issueLabelAddCmd, issueLabelRemoveCmd)
	issueBulkCmd.AddCommand(issueBulkAssignCmd, issueBulkMoveCmd, issueBulkUpdateCmd)
	issueCommentCmd.AddCommand(issueCommentAddCmd, issueCommentListCmd)
	issueFilterCmd.AddCommand(issueFilterSaveCmd, issueFilterListCmd, issueFilterRunCmd, issueFilterDeleteCmd, issueFilterRecentCmd)

	registerIssueRootShortcutFlags(issueCmd)
	registerIssueRootOptionFlags(issueCmd)
	registerIssueCreateFlags(issueCreateCmd)
	registerIssueUpdateFlags(issueUpdateCmd)
	registerIssueListFlags(issueListCmd)
	issueSearchCmd.Flags().Int("page", 1, "Page number")
	issueSearchCmd.Flags().Int("limit", 50, "Page size")
	issueFilterRunCmd.Flags().Int("page", 1, "Page number")
	issueFilterRunCmd.Flags().Int("limit", 50, "Page size")
	issueFilterRecentCmd.Flags().Int("limit", 20, "Max recent query count")

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

func runIssueRoot(cmd *cobra.Command, args []string) error {
	createChanged := cmd.Flags().Changed("create")
	listChanged := cmd.Flags().Changed("list")
	updateChanged := cmd.Flags().Changed("update")
	actionCount := 0
	for _, changed := range []bool{createChanged, listChanged, updateChanged} {
		if changed {
			actionCount++
		}
	}
	if actionCount == 0 {
		return cmd.Help()
	}
	if actionCount > 1 {
		return fmt.Errorf("choose one root shortcut: --create/-c, --list/-l, or --update/-u")
	}

	if createChanged {
		titleParts := []string{strings.TrimSpace(mustString(cmd, "create"))}
		titleParts = append(titleParts, args...)
		title := strings.TrimSpace(strings.Join(titleParts, " "))
		if title == "" {
			return fmt.Errorf("issue title is required")
		}
		return runIssueCreate(cmd, []string{title})
	}
	if listChanged {
		if len(args) > 0 {
			return fmt.Errorf("unexpected args for issue list shortcut: %s", strings.Join(args, " "))
		}
		return runIssueList(cmd, nil)
	}

	id := strings.TrimSpace(mustString(cmd, "update"))
	if id == "" {
		return fmt.Errorf("issue id is required for --update/-u")
	}
	if len(args) > 0 {
		return fmt.Errorf("unexpected args for issue update shortcut: %s", strings.Join(args, " "))
	}
	return runIssueUpdate(cmd, []string{id})
}

func runIssueCreate(cmd *cobra.Command, args []string) error {
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
	smart := parseSmartIssueInput(args[0], cfg.UserID)
	title := smart.Title
	if title == "" {
		title = strings.TrimSpace(args[0])
	}
	if strings.TrimSpace(issueType) == "" {
		issueType = "task"
	}
	if strings.TrimSpace(priority) == "" {
		priority = "medium"
	}
	if !cmd.Flags().Changed("priority") && smart.Priority != "" {
		priority = smart.Priority
	}
	if !cmd.Flags().Changed("assignee-id") && smart.AssigneeID != "" {
		assigneeID = smart.AssigneeID
	}
	if strings.TrimSpace(assigneeID) == "" {
		assigneeID = strings.TrimSpace(cfg.UserID)
	}
	if strings.TrimSpace(description) == "" && strings.TrimSpace(smart.Description) != "" {
		description = smart.Description
	}
	labels := parseCSV(labelsFlag)
	if !cmd.Flags().Changed("labels") {
		labels = append(labels, smart.Labels...)
	}
	labels = parseCSV(strings.Join(labels, ","))
	if len(labels) == 0 {
		if repoLabel := currentRepoLabel(); repoLabel != "" {
			labels = []string{repoLabel}
		}
	}

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
	issue, err := client.CreateIssueSmart(api.CreateIssueInput{
		Title:       title,
		ProjectID:   projectID,
		Description: description,
		IssueType:   issueType,
		Priority:    priority,
		ParentID:    parentID,
		SprintID:    sprintID,
		AssigneeID:  assigneeID,
		Labels:      labels,
	})
	if err != nil {
		return err
	}
	fmt.Println(utils.SuccessText(fmt.Sprintf("Created issue %s: %s", issue.ID, issue.Title)))
	return nil
}

func runIssueList(cmd *cobra.Command, args []string) error {
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
	issues, err := client.ListIssuesSmart(filter)
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
}

func runIssueUpdate(cmd *cobra.Command, args []string) error {
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
	issue, err := client.UpdateIssueSmart(id, input)
	if err != nil {
		return err
	}
	fmt.Println(utils.SuccessText(fmt.Sprintf("Updated issue %s (%s)", issue.ID, issue.Status)))
	return nil
}

func registerIssueRootShortcutFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("create", "c", "", "Create an issue from the issues root")
	cmd.Flags().BoolP("list", "l", false, "List issues from the issues root")
	cmd.Flags().StringP("update", "u", "", "Update an issue by ID from the issues root")
}

func registerIssueRootOptionFlags(cmd *cobra.Command) {
	cmd.Flags().String("project-id", "", "Project UUID (create) or project filter (list)")
	cmd.Flags().String("description", "", "Issue description")
	cmd.Flags().String("type", "", "Issue type or type filter")
	cmd.Flags().String("priority", "", "Issue priority")
	cmd.Flags().String("parent-id", "", "Parent issue UUID")
	cmd.Flags().String("sprint-id", "", "Sprint UUID or sprint filter")
	cmd.Flags().String("assignee-id", "", "Assignee user UUID or assignee filter")
	cmd.Flags().String("labels", "", "Comma-separated labels for create/update")
	cmd.Flags().String("label", "", "Filter by label")
	cmd.Flags().Bool("interactive", false, "Launch interactive creation wizard")
	cmd.Flags().String("title", "", "Issue title")
	cmd.Flags().String("status", "", "Issue status or status filter")
	cmd.Flags().String("query", "", "Filter text in issue titles/descriptions")
	cmd.Flags().String("sort-by", "", "Sort by field (created_at|updated_at|priority|status)")
	cmd.Flags().String("order", "", "Sort order (asc|desc)")
	cmd.Flags().Int("page", 1, "Page number")
	cmd.Flags().Int("limit", 50, "Page size")
}

func registerIssueCreateFlags(cmd *cobra.Command) {
	cmd.Flags().String("project-id", "", "Project UUID (defaults to active context)")
	cmd.Flags().String("description", "", "Issue description")
	cmd.Flags().String("type", "task", "Issue type")
	cmd.Flags().String("priority", "medium", "Issue priority")
	cmd.Flags().String("parent-id", "", "Parent issue UUID")
	cmd.Flags().String("sprint-id", "", "Sprint UUID")
	cmd.Flags().String("assignee-id", "", "Assignee user UUID")
	cmd.Flags().String("labels", "", "Comma-separated labels")
	cmd.Flags().Bool("interactive", false, "Launch interactive creation wizard")
}

func registerIssueUpdateFlags(cmd *cobra.Command) {
	cmd.Flags().String("title", "", "Issue title")
	cmd.Flags().String("description", "", "Issue description")
	cmd.Flags().String("priority", "", "Issue priority")
	cmd.Flags().String("type", "", "Issue type")
	cmd.Flags().String("status", "", "Issue status")
	cmd.Flags().String("parent-id", "", "Parent issue UUID")
	cmd.Flags().String("sprint-id", "", "Sprint UUID")
	cmd.Flags().String("assignee-id", "", "Assignee user UUID")
	cmd.Flags().String("labels", "", "Replace labels with comma-separated values")
}

func registerIssueListFlags(cmd *cobra.Command) {
	cmd.Flags().String("project-id", "", "Filter by project UUID")
	cmd.Flags().String("status", "", "Filter by status")
	cmd.Flags().String("assignee-id", "", "Filter by assignee UUID")
	cmd.Flags().String("sprint-id", "", "Filter by sprint UUID")
	cmd.Flags().String("label", "", "Filter by label")
	cmd.Flags().String("type", "", "Filter by issue type")
	cmd.Flags().String("query", "", "Filter text in issue titles/descriptions")
	cmd.Flags().String("sort-by", "", "Sort by field (created_at|updated_at|priority|status)")
	cmd.Flags().String("order", "", "Sort order (asc|desc)")
	cmd.Flags().Int("page", 1, "Page number")
	cmd.Flags().Int("limit", 50, "Page size")
}

type smartIssueInput struct {
	Title       string
	Description string
	Priority    string
	AssigneeID  string
	Labels      []string
}

func parseSmartIssueInput(raw, currentUserID string) smartIssueInput {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return smartIssueInput{}
	}
	titleParts := make([]string, 0, len(fields))
	labels := make([]string, 0, 4)
	seenLabels := map[string]struct{}{}
	out := smartIssueInput{}
	appendLabel := func(value string) {
		for _, label := range parseCSV(value) {
			if _, ok := seenLabels[label]; ok {
				continue
			}
			seenLabels[label] = struct{}{}
			labels = append(labels, label)
		}
	}
	for i := 0; i < len(fields); i++ {
		token := strings.TrimSpace(fields[i])
		normalized := strings.ToLower(token)
		switch normalized {
		case "assign", "assignee":
			if i+1 < len(fields) {
				i++
				assignee := strings.TrimSpace(fields[i])
				if strings.EqualFold(assignee, "me") || assignee == "@me" {
					out.AssigneeID = strings.TrimSpace(currentUserID)
				} else {
					out.AssigneeID = assignee
				}
				continue
			}
		case "label", "labels":
			if i+1 < len(fields) {
				i++
				appendLabel(fields[i])
				continue
			}
		case "desc", "description":
			if i+1 < len(fields) {
				out.Description = strings.Join(fields[i+1:], " ")
				i = len(fields)
				continue
			}
		}
		if mapped := smartPriority(normalized); mapped != "" {
			out.Priority = mapped
			continue
		}
		if strings.HasPrefix(token, "#") {
			appendLabel(strings.TrimPrefix(token, "#"))
			continue
		}
		titleParts = append(titleParts, token)
	}
	out.Title = strings.TrimSpace(strings.Join(titleParts, " "))
	out.Labels = labels
	return out
}

func smartPriority(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "p1", "critical":
		return "critical"
	case "p2", "high":
		return "high"
	case "p3", "medium":
		return "medium"
	case "p4", "low":
		return "low"
	default:
		return ""
	}
}

func currentRepoLabel() string {
	output, err := gitOutput("rev-parse", "--show-toplevel")
	if err != nil {
		return ""
	}
	name := strings.ToLower(strings.TrimSpace(filepath.Base(strings.TrimSpace(output))))
	name = sanitizeLocalLabel(name)
	name = strings.Trim(name, "-.")
	if name == "" {
		return ""
	}
	return "repo:" + name
}

func sanitizeLocalLabel(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return b.String()
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
