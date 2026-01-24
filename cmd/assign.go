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

var assignCmd = &cobra.Command{
	Use:   "assign [ISSUE_KEY]",
	Short: "Assign an issue to a user",
	Long: `Assign an issue to a specific user.
You can assign to yourself, or search for other users by name or email.
If no issue key is provided, you can select one interactively.`,
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

		// Select User
		myself, err := client.GetMyself()
		if err != nil {
			fmt.Printf("Error fetching current user: %v\n", err)
			os.Exit(1)
		}

		searchOption := jira.User{DisplayName: "🔍 Search for user...", AccountID: "SEARCH"}
		options := []jira.User{*myself, searchOption}

		prompt := promptui.Select{
			Label: "Select Assignee",
			Items: options,
			Templates: &promptui.SelectTemplates{
				Label:    "{{ . }}?",
				Active:   "\U0001F449 {{ .DisplayName | cyan }}",
				Inactive: "   {{ .DisplayName | cyan }}",
				Selected: "\U0001F44D Assignee: {{ .DisplayName | green }}",
			},
		}

		i, _, err := prompt.Run()
		if err != nil {
			os.Exit(1)
		}

		var accountID string
		if options[i].AccountID == "SEARCH" {
			searchPrompt := promptui.Prompt{Label: "Search User (Name or Email)"}
			query, _ := searchPrompt.Run()
			users, err := client.FindUsers(query)
			if err != nil || len(users) == 0 {
				fmt.Println("No users found or error searching.")
				os.Exit(1)
			}
			userSelect := promptui.Select{
				Label: "Select User",
				Items: users,
				Templates: &promptui.SelectTemplates{
					Active:   "\U0001F449 {{ .DisplayName | cyan }} ({{ .EmailAddress }})",
					Inactive: "   {{ .DisplayName | cyan }} ({{ .EmailAddress }})",
					Selected: "\U0001F44D Assignee: {{ .DisplayName | green }}",
				},
			}
			j, _, _ := userSelect.Run()
			accountID = users[j].AccountID
		} else {
			accountID = options[i].AccountID
		}

		if err := client.AssignIssue(issueKey, accountID); err != nil {
			fmt.Printf("Error assigning issue: %v\n", err)
			if strings.Contains(err.Error(), "401") {
				fmt.Println("\nUnable to assign issue. You might need to log in.")
				fmt.Println("Run: scrum auth login")
			}
			os.Exit(1)
		}
		fmt.Printf("Issue %s assigned successfully.\n", issueKey)
	},
}

func init() {
	issueCmd.AddCommand(assignCmd)
}