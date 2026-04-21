package cmd

import (
	"fmt"
	"os"

	"github.com/atharshah1/scrum/internal/jira"
	"github.com/spf13/cobra"
)

var issuesCmd = &cobra.Command{
	Use:   "issues",
	Short: "List legacy source issues",
	Long: `Quickly list source issues matching a JQL query.
This is a shortcut for legacy migration work similar to 'scrum issue ls'.`,
	Run: func(cmd *cobra.Command, args []string) {
		jql, _ := cmd.Flags().GetString("jql")
		if jql == "" {
			jql = "assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC"
		}

		client := &jira.Client{}
		issues, err := client.SearchIssues(jql)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		for _, issue := range issues {
			fmt.Printf("%s: %s (%s)\n", issue.Key, issue.Fields.Summary, issue.Fields.Status.Name)
		}
	},
}

func init() {
	rootCmd.AddCommand(issuesCmd)
	issuesCmd.Flags().String("jql", "", "Source-system JQL query to filter issues")
}
