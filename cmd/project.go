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

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage project context",
	Long: `Manage the legacy source project context used by the migration bridge.
Switching projects sets the default source project for legacy issue commands.`,
}

var projectLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List available projects",
	Long:  `Display source projects available to your connected Atlassian account.`,
	Run: func(cmd *cobra.Command, args []string) {
		client := &jira.Client{}
		projects, err := client.GetProjects()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		current, _ := auth.LoadProject()

		for _, p := range projects {
			prefix := "  "
			if p.Key == current {
				prefix = "* "
			}
			fmt.Printf("%s%s (%s)\n", prefix, p.Key, p.Name)
		}
	},
}

var projectCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a legacy source project",
	Long: `Create a new project in the connected Atlassian source.
You will be prompted for a project key, name, and lead if not provided via flags.`,
	Run: func(cmd *cobra.Command, args []string) {
		key, _ := cmd.Flags().GetString("key")
		name, _ := cmd.Flags().GetString("name")

		if key == "" {
			prompt := promptui.Prompt{
				Label: "Project Key",
				Validate: func(input string) error {
					if len(input) == 0 {
						return fmt.Errorf("key cannot be empty")
					}
					return nil
				},
			}
			var err error
			key, err = prompt.Run()
			if err != nil {
				fmt.Printf("Prompt failed %v\n", err)
				os.Exit(1)
			}
		}

		if name == "" {
			prompt := promptui.Prompt{
				Label: "Project Name",
				Validate: func(input string) error {
					if len(input) == 0 {
						return fmt.Errorf("name cannot be empty")
					}
					return nil
				},
			}
			var err error
			name, err = prompt.Run()
			if err != nil {
				fmt.Printf("Prompt failed %v\n", err)
				os.Exit(1)
			}
		}

		client := &jira.Client{}

		// 1. Get Myself for default option
		myself, err := client.GetMyself()
		if err != nil {
			fmt.Printf("Error fetching current user: %v\n", err)
			os.Exit(1)
		}

		// 2. Prompt for Project Lead
		var leadAccountId string
		searchOption := jira.User{DisplayName: "🔍 Search for user...", AccountID: "SEARCH"}
		options := []jira.User{*myself, searchOption}

		prompt := promptui.Select{
			Label: "Select Project Lead",
			Items: options,
			Templates: &promptui.SelectTemplates{
				Label:    "{{ . }}?",
				Active:   "\U0001F449 {{ .DisplayName | cyan }}",
				Inactive: "   {{ .DisplayName | cyan }}",
				Selected: "\U0001F44D Project Lead: {{ .DisplayName | green }}",
			},
		}

		i, _, err := prompt.Run()
		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			os.Exit(1)
		}

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
					Selected: "\U0001F44D Project Lead: {{ .DisplayName | green }}",
				},
			}
			j, _, _ := userSelect.Run()
			leadAccountId = users[j].AccountID
		} else {
			leadAccountId = options[i].AccountID
		}

		createdKey, err := client.CreateProject(key, name, leadAccountId)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			if strings.Contains(err.Error(), "401") {
				fmt.Println("\nUnable to create project. You might need to log in to grant new permissions.")
				fmt.Println("Run: scrum auth login")
			}
			os.Exit(1)
		}
		fmt.Printf("Project created: %s\n", createdKey)
	},
}

var projectSwitchCmd = &cobra.Command{
	Use:   "switch [KEY]",
	Short: "Switch project context",
	Long: `Set the active source project context for the legacy CLI.
This saves the project key locally for legacy create/list flows.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var key string
		if len(args) > 0 {
			key = args[0]
		} else {
			// Interactive mode
			client := &jira.Client{}
			projects, err := client.GetProjects()
			if err != nil {
				fmt.Printf("Error fetching projects: %v\n", err)
				os.Exit(1)
			}

			if len(projects) == 0 {
				fmt.Println("No projects found.")
				os.Exit(0)
			}

			prompt := promptui.Select{
				Label: "Select Project",
				Items: projects,
				Templates: &promptui.SelectTemplates{
					Label:    "{{ . }}?",
					Active:   "\U0001F449 {{ .Key | cyan }} ({{ .Name }})",
					Inactive: "   {{ .Key | cyan }} ({{ .Name }})",
					Selected: "\U0001F44D {{ .Key | green }}",
				},
			}

			i, _, err := prompt.Run()
			if err != nil {
				fmt.Printf("Prompt failed %v\n", err)
				os.Exit(1)
			}
			key = projects[i].Key
		}

		err := auth.SaveProject(key)
		if err != nil {
			fmt.Printf("Error saving project: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Switched to project: %s\n", key)
	},
}

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectLsCmd)
	projectCmd.AddCommand(projectCreateCmd)
	projectCmd.AddCommand(projectSwitchCmd)

	projectCreateCmd.Flags().StringP("key", "k", "", "Project Key (e.g. SCRUM)")
	projectCreateCmd.Flags().StringP("name", "n", "", "Project Display Name")
}
