package cmd

import (
"fmt"
"strings"

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
projectID, _ := cmd.Flags().GetString("project-id")
if strings.TrimSpace(projectID) == "" {
return fmt.Errorf("--project-id is required")
}
description, _ := cmd.Flags().GetString("description")
issueType, _ := cmd.Flags().GetString("type")
priority, _ := cmd.Flags().GetString("priority")
issue, err := client.CreateIssue(args[0], projectID, description, issueType, priority)
if err != nil {
return err
}
fmt.Printf("Created issue %s: %s\n", issue.ID, issue.Title)
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
fmt.Println("ID\tSTATUS\tPRIORITY\tTITLE")
for _, it := range issues {
fmt.Printf("%s\t%s\t%s\t%s\n", it.ID, it.Status, it.Priority, it.Title)
}
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
fmt.Printf("ID: %s\nTitle: %s\nStatus: %s\nPriority: %s\nType: %s\nProject: %s\nDescription: %s\n",
issue.ID, issue.Title, issue.Status, issue.Priority, issue.IssueType, issue.ProjectID, issue.Description)
return nil
},
}

func init() {
rootCmd.AddCommand(issueCmd)
issueCmd.AddCommand(issueCreateCmd, issueListCmd, issueViewCmd)

issueCreateCmd.Flags().String("project-id", "", "Project UUID")
issueCreateCmd.Flags().String("description", "", "Issue description")
issueCreateCmd.Flags().String("type", "task", "Issue type")
issueCreateCmd.Flags().String("priority", "medium", "Issue priority")
}
