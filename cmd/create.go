package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/atharshah1/scrum/internal/auth"
	"github.com/atharshah1/scrum/internal/jira"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a legacy source issue",
	Long: `Create a new issue in the active source project.
Supports interactive prompts for summary and type if not provided.
You can also assign the issue immediately using the --assignee flag.`,
	Example: `  scrum issue create --project SCRUM --summary "Fix login bug" --type Bug`,
	Run: func(cmd *cobra.Command, args []string) {
		project, _ := cmd.Flags().GetString("project")
		summary, _ := cmd.Flags().GetString("summary")
		issueType, _ := cmd.Flags().GetString("type")
		assignee, _ := cmd.Flags().GetString("assignee")

		if project == "" {
			p, err := auth.LoadProject()
			if err != nil || p == "" {
				fmt.Println("Error: --project is required or set a context with 'scrum project switch'")
				os.Exit(1)
			}
			project = p
		}

		client := &jira.Client{}
		var assigneeID string
		if assignee != "" {
			if assignee == "me" {
				me, err := client.GetMyself()
				if err != nil {
					fmt.Printf("Error fetching current user: %v\n", err)
					os.Exit(1)
				}
				assigneeID = me.AccountID
			} else {
				users, err := client.FindUsers(assignee)
				if err != nil || len(users) == 0 {
					fmt.Println("No users found for assignee.")
					os.Exit(1)
				}
				assigneeID = users[0].AccountID
			}
		}

		key, err := client.CreateIssue(project, summary, issueType, assigneeID)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			if strings.Contains(err.Error(), "401") {
				fmt.Println("\nUnable to create issue. You might need to log in.")
				fmt.Println("Run: scrum auth login")
			}
			os.Exit(1)
		}

		fmt.Printf("Issue created: %s\n", key)
	},
}

func init() {
	issueCmd.AddCommand(createCmd)
	createCmd.Flags().StringP("project", "p", "", "Project Key (e.g. SCRUM)")
	createCmd.Flags().StringP("summary", "s", "", "Issue Summary")
	createCmd.Flags().StringP("type", "t", "Task", "Issue Type (Task, Bug, Story)")
	createCmd.Flags().StringP("assignee", "a", "", "Assignee (name, email, or 'me')")
	_ = createCmd.MarkFlagRequired("summary")
}
