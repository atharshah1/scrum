/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/atharshah1/scrum/internal/auth"
	"github.com/atharshah1/scrum/internal/jira"
	"github.com/spf13/cobra"
)

// lsCmd represents the ls command
var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List issues in the current context",
	Long: `List source issues from the current migration context or via an explicit JQL query.
By default, it shows unresolved issues assigned to the current user.
Prefer scrumx issue list/search for primary daily workflows.`,
	Run: func(cmd *cobra.Command, args []string) {
		showAll, _ := cmd.Flags().GetBool("all")
		jql, _ := cmd.Flags().GetString("jql")

		currentProject, _ := auth.LoadProject()

		if jql == "" {
			if showAll {
				if currentProject == "" {
					fmt.Println("Error: -a requires a project context. Use 'scrum project switch <KEY>'")
					os.Exit(1)
				}
				jql = fmt.Sprintf("project = %s ORDER BY updated DESC", currentProject)
			} else if currentProject != "" {
				jql = fmt.Sprintf("project = %s AND assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC", currentProject)
			} else {
				jql = "assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC"
			}
		}

		client := &jira.Client{}
		issues, err := client.SearchIssues(jql)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			if strings.Contains(err.Error(), "401") {
				fmt.Println("\nUnable to fetch issues. You might need to log in.")
				fmt.Println("Run: scrum auth login")
			}
			os.Exit(1)
		}

		for _, issue := range issues {
			fmt.Printf("%s: %s (%s)\n", issue.Key, issue.Fields.Summary, issue.Fields.Status.Name)
		}
	},
}

func init() {
	issueCmd.AddCommand(lsCmd)
	lsCmd.Flags().String("jql", "", "Source-system JQL query to filter issues")
	lsCmd.Flags().BoolP("all", "a", false, "Show all issues in the current project")
}
