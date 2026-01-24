package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/atharshah1/scrum/internal/auth"
	"github.com/atharshah1/scrum/internal/jira"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var commentCmd = &cobra.Command{
	Use:   "comment",
	Short: "View and add comments",
	Long: `View and add comments to Jira issues.
Supports rich text rendering and user mentions.`,
}

var commentAddCmd = &cobra.Command{
	Use:   "add [ISSUE_KEY]",
	Short: "Add a comment (use @[Name] to mention)",
	Long: `Post a new comment to a specific issue.
You can mention users using the syntax @[Name]. If the issue key is omitted,
an interactive selection list is shown.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var issueKey string
		client := &jira.Client{}

		if len(args) > 0 {
			issueKey = args[0]
		} else {
			// Interactive selection
			project, _ := auth.LoadProject()
			jql := "assignee = currentUser() ORDER BY updated DESC"
			if project != "" {
				jql = fmt.Sprintf("project = %s ORDER BY updated DESC", project)
			}

			issues, err := client.SearchIssues(jql)
			if err != nil {
				fmt.Printf("Error fetching issues: %v\n", err)
				os.Exit(1)
			}
			if len(issues) == 0 {
				fmt.Println("No issues found.")
				os.Exit(0)
			}

			prompt := promptui.Select{
				Label: "Select Issue",
				Items: issues,
				Templates: &promptui.SelectTemplates{
					Active:   "\U0001F449 {{ .Key | cyan }} {{ .Fields.Summary }}",
					Inactive: "   {{ .Key | cyan }} {{ .Fields.Summary }}",
					Selected: "\U0001F44D {{ .Key | green }}",
				},
			}
			i, _, err := prompt.Run()
			if err != nil {
				os.Exit(1)
			}
			issueKey = issues[i].Key
		}

		message, _ := cmd.Flags().GetString("message")
		if message == "" {
			prompt := promptui.Prompt{
				Label: "Comment (use @[Name] to mention)",
				Validate: func(input string) error {
					if len(strings.TrimSpace(input)) == 0 {
						return fmt.Errorf("comment cannot be empty")
					}
					return nil
				},
			}
			var err error
			message, err = prompt.Run()
			if err != nil {
				fmt.Printf("Prompt failed %v\n", err)
				os.Exit(1)
			}
		}

		if err := client.AddComment(issueKey, message); err != nil {
			fmt.Printf("Error adding comment: %v\n", err)
			if strings.Contains(err.Error(), "401") {
				fmt.Println("\nUnable to add comment. You might need to log in.")
				fmt.Println("Run: scrum auth login")
			}
			os.Exit(1)
		}
		fmt.Println("Comment added successfully.")
	},
}

var commentLsCmd = &cobra.Command{
	Use:   "ls [ISSUE_KEY]",
	Short: "List comments for an issue",
	Long: `List all comments for a specific issue in chronological order.
Provides an interactive menu to reply to specific comments.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		issueKey := args[0]
		client := &jira.Client{}
		comments, err := client.GetComments(issueKey)
		if err != nil {
			fmt.Printf("Error fetching comments: %v\n", err)
			if strings.Contains(err.Error(), "401") {
				fmt.Println("\nUnable to fetch comments. You might need to log in.")
				fmt.Println("Run: scrum auth login")
			}
			os.Exit(1)
		}

		if len(comments) == 0 {
			fmt.Println("No comments found.")
			return
		}

		// Prepare comments for display
		type DisplayComment struct {
			Original jira.Comment
			Text     string
		}
		displayComments := make([]DisplayComment, len(comments))
		for i, c := range comments {
			displayComments[i] = DisplayComment{
				Original: c,
				Text:     extractTextFromADF(c.Body),
			}
		}

		prompt := promptui.Select{
			Label: "Comments (Select to reply, Esc to exit)",
			Items: displayComments,
			Templates: &promptui.SelectTemplates{
				Active:   "\U0001F449 {{ .Original.Author.DisplayName | cyan }}: {{ .Text | slice 0 50 }}...",
				Inactive: "   {{ .Original.Author.DisplayName | cyan }}: {{ .Text | slice 0 50 }}...",
				Selected: "\U0001F44D Selected comment by {{ .Original.Author.DisplayName }}",
				Details: `
--------- Comment ----------
{{ "Author:" | faint }} {{ .Original.Author.DisplayName }}
{{ "Date:" | faint }} {{ .Original.Created }}
{{ "Body:" | faint }}
{{ .Text }}`,
			},
			Size: 10,
		}

		i, _, err := prompt.Run()
		if err != nil {
			return // Exit if cancelled
		}

		// Reply flow
		fmt.Printf("Replying to %s...\n", displayComments[i].Original.Author.DisplayName)
		msgPrompt := promptui.Prompt{
			Label: "Reply Message (use @[Name] to mention)",
		}
		replyMsg, _ := msgPrompt.Run()
		if replyMsg != "" {
			client.AddComment(issueKey, replyMsg)
			fmt.Println("Reply added successfully.")
		}
	},
}

// extractTextFromADF recursively extracts text from the Atlassian Document Format JSON
func extractTextFromADF(node map[string]interface{}) string {
	var result strings.Builder

	if nodeType, ok := node["type"].(string); ok && nodeType == "text" {
		if text, ok := node["text"].(string); ok {
			return text
		}
	}

	if content, ok := node["content"].([]interface{}); ok {
		for _, child := range content {
			if childMap, ok := child.(map[string]interface{}); ok {
				childText := extractTextFromADF(childMap)
				result.WriteString(childText)
				
				// Add newline for paragraphs
				if typeStr, ok := childMap["type"].(string); ok && typeStr == "paragraph" {
					result.WriteString("\n")
				}
			}
		}
	}

	return result.String()
}

func init() {
	issueCmd.AddCommand(commentCmd)
	commentCmd.AddCommand(commentAddCmd)
	commentCmd.AddCommand(commentLsCmd)

	commentAddCmd.Flags().StringP("message", "m", "", "Comment message")
}