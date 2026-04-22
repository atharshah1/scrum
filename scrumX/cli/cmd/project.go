package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/cli/internal/api"
	"github.com/atharshah1/scrum/scrumX/cli/internal/utils"
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:     "project",
	Aliases: []string{"p"},
	Short:   "Project commands",
	Example: strings.TrimSpace(`
  scrumx project create CORE "Core platform"
  sx p c CORE "Core platform"
  scrumx p -c CORE "Core platform"

  scrumx project list
  sx p l
  scrumx p -l
`),
	RunE: runProjectRoot,
}

var projectCreateCmd = &cobra.Command{
	Use:     "create <key> <name>",
	Aliases: []string{"c"},
	Short:   "Create a project",
	Args:    cobra.MinimumNArgs(2),
	RunE:    runProjectCreate,
}

var projectListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"l"},
	Short:   "List projects",
	RunE:    runProjectList,
}

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectCreateCmd, projectListCmd)
	projectCmd.Flags().StringP("create", "c", "", "Create a project from the project root using '<key> <name>'")
	projectCmd.Flags().BoolP("list", "l", false, "List projects from the project root")
}

func runProjectRoot(cmd *cobra.Command, args []string) error {
	createChanged := cmd.Flags().Changed("create")
	listChanged := cmd.Flags().Changed("list")
	if createChanged && listChanged {
		return fmt.Errorf("choose one project root shortcut: --create/-c or --list/-l")
	}
	if !createChanged && !listChanged {
		return cmd.Help()
	}
	if listChanged {
		if len(args) > 0 {
			return fmt.Errorf("unexpected args for project list shortcut: %s", strings.Join(args, " "))
		}
		return runProjectList(cmd, nil)
	}

	key := strings.TrimSpace(mustString(cmd, "create"))
	if key == "" {
		return fmt.Errorf("project key is required for --create/-c")
	}
	return runProjectCreate(cmd, append([]string{key}, args...))
}

func runProjectCreate(cmd *cobra.Command, args []string) error {
	client, err := newClient()
	if err != nil {
		return err
	}
	key := strings.TrimSpace(args[0])
	name := strings.TrimSpace(strings.Join(args[1:], " "))
	if key == "" || name == "" {
		return fmt.Errorf("project key and name are required")
	}
	project, err := client.CreateProject(api.CreateProjectInput{Key: key, Name: name})
	if err != nil {
		return err
	}
	fmt.Println(utils.SuccessText(fmt.Sprintf("Created project %s (%s)", project.Key, project.ID)))
	return nil
}

func runProjectList(cmd *cobra.Command, args []string) error {
	client, err := newClient()
	if err != nil {
		return err
	}
	projects, err := client.ListProjects()
	if err != nil {
		return err
	}
	if len(projects) == 0 {
		fmt.Println("No projects found")
		return nil
	}
	rows := make([][]string, 0, len(projects))
	for _, project := range projects {
		updatedAt := ""
		if !project.UpdatedAt.IsZero() {
			updatedAt = project.UpdatedAt.Local().Format(time.RFC3339)
		}
		rows = append(rows, []string{project.ID, project.Key, project.Name, project.Role, updatedAt})
	}
	utils.PrintTable([]string{"ID", "KEY", "NAME", "ROLE", "UPDATED"}, rows)
	return nil
}
