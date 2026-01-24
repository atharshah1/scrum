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

var statusCmd = &cobra.Command{
	Use:   "status [ISSUE_KEY]",
	Short: "Transition an issue to a new status",
	Long: `Move an issue through its workflow (e.g., To Do -> Done).
Fetches valid transitions from Jira and presents an interactive selection menu.`,
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

		transitions, err := client.GetTransitions(issueKey)
		if err != nil {
			fmt.Printf("Error fetching transitions: %v\n", err)
			if strings.Contains(err.Error(), "401") {
				fmt.Println("\nUnable to fetch transitions. You might need to log in.")
				fmt.Println("Run: scrum auth login")
			}
			os.Exit(1)
		}

		if len(transitions) == 0 {
			fmt.Println("No transitions available for this issue.")
			os.Exit(0)
		}

		prompt := promptui.Select{
			Label: "Select Transition",
			Items: transitions,
			Templates: &promptui.SelectTemplates{
				Label:    "{{ . }}?",
				Active:   "\U0001F449 {{ .Name | cyan }} (to {{ .To.Name }})",
				Inactive: "   {{ .Name | cyan }} (to {{ .To.Name }})",
				Selected: "\U0001F44D {{ .Name | green }}",
				Details: `
--------- Transition ----------
{{ "Name:" | faint }}	{{ .Name }}
{{ "To Status:" | faint }}	{{ .To.Name }}`,
			},
		}

		i, _, err := prompt.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			os.Exit(1)
		}

		if err := client.TransitionIssue(issueKey, transitions[i].ID); err != nil {
			fmt.Printf("Error transitioning issue: %v\n", err)
			if strings.Contains(err.Error(), "401") {
				fmt.Println("\nUnable to transition issue. You might need to log in.")
				fmt.Println("Run: scrum auth login")
			}
			os.Exit(1)
		}
		fmt.Printf("Issue %s transitioned to %s\n", issueKey, transitions[i].To.Name)
	},
}

func init() {
	issueCmd.AddCommand(statusCmd)
}