package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/cli/internal/api"
	"github.com/atharshah1/scrum/scrumX/cli/internal/utils"
	"github.com/spf13/cobra"
)

var issueConflictsCmd = &cobra.Command{
	Use:     "conflicts",
	Aliases: []string{"conflict", "cf"},
	Short:   "Resolve offline issue update conflicts",
}

var issueConflictsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List issue conflicts",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		includeAll, _ := cmd.Flags().GetBool("all")
		issueID, _ := cmd.Flags().GetString("issue")
		asJSON, _ := cmd.Flags().GetBool("json")

		conflicts, err := client.ListIssueConflicts(api.ConflictFilter{
			IssueID:         strings.TrimSpace(issueID),
			IncludeResolved: includeAll,
		})
		if err != nil {
			return err
		}
		if asJSON {
			payload, _ := json.MarshalIndent(conflicts, "", "  ")
			fmt.Println(string(payload))
			return nil
		}
		if len(conflicts) == 0 {
			fmt.Println("No conflicts found")
			return nil
		}
		rows := make([][]string, 0, len(conflicts))
		for _, item := range conflicts {
			actor := strings.TrimSpace(item.ActorDisplayName)
			if actor == "" {
				actor = strings.TrimSpace(item.ActorID)
			}
			if actor == "" {
				actor = "-"
			}
			state := "open"
			if item.Resolved {
				state = "resolved"
			}
			rows = append(rows, []string{
				item.ID,
				item.TargetID,
				state,
				fmt.Sprintf("%d", len(item.Fields)),
				actor,
				item.CreatedAt.Local().Format(time.RFC3339),
			})
		}
		utils.PrintTable([]string{"CONFLICT_ID", "ISSUE_ID", "STATE", "FIELDS", "ACTOR", "CREATED"}, rows)
		fmt.Printf("\nTotal conflicts: %d\n", len(conflicts))
		if !includeAll {
			fmt.Println("Hint: sx i cf show <conflict-id|issue-id>")
			fmt.Println("      sx i cf resolve <...> --action keep-mine|keep-server|keep-both|later")
		}
		return nil
	},
}

var issueConflictsShowCmd = &cobra.Command{
	Use:     "show <conflict-id|issue-id>",
	Aliases: []string{"view", "v"},
	Short:   "Show conflict details",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		includeResolved, _ := cmd.Flags().GetBool("all")
		asJSON, _ := cmd.Flags().GetBool("json")
		item, err := client.GetIssueConflict(args[0], includeResolved)
		if err != nil {
			return err
		}
		if asJSON {
			payload, _ := json.MarshalIndent(item, "", "  ")
			fmt.Println(string(payload))
			return nil
		}
		fmt.Printf("Conflict: %s\nIssue: %s\nEntity: %s\nCreated: %s\n",
			item.ID, item.TargetID, item.Entity, item.CreatedAt.Local().Format(time.RFC3339))
		origin := strings.TrimSpace(item.ActorDisplayName)
		if origin == "" {
			origin = strings.TrimSpace(item.ActorID)
		}
		if origin == "" {
			origin = "unknown"
		}
		fmt.Printf("Origin actor: %s\n", origin)
		if item.Resolved {
			fmt.Printf("Resolved: yes (%s at %s by %s)\n",
				item.Resolution,
				item.ResolvedAt.Local().Format(time.RFC3339),
				coalesce(item.ResolvedByName, item.ResolvedByID, "unknown"),
			)
		} else {
			fmt.Println("Resolved: no")
		}
		fmt.Println("\nField differences:")
		if len(item.Fields) == 0 {
			fmt.Println("  (no field-level metadata available)")
		} else {
			for _, field := range item.Fields {
				fieldKind := "scalar"
				if strings.EqualFold(strings.TrimSpace(field.Field), "labels") {
					fieldKind = "additive"
				}
				fmt.Printf("- %s [%s]\n  local : %s\n  server: %s\n", field.Field, fieldKind, safe(field.LocalValue), safe(field.ServerValue))
			}
		}
		fmt.Println("\nLocal snapshot:")
		if len(item.LocalSnapshot) == 0 {
			fmt.Println("  (missing)")
		} else {
			fmt.Println(string(item.LocalSnapshot))
		}
		fmt.Println("\nServer snapshot:")
		if len(item.ServerSnapshot) == 0 {
			fmt.Println("  (missing)")
		} else {
			fmt.Println(string(item.ServerSnapshot))
		}
		return nil
	},
}

var issueConflictsResolveCmd = &cobra.Command{
	Use:     "resolve <conflict-id|issue-id>",
	Aliases: []string{"r"},
	Short:   "Resolve a conflict",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		action := strings.ToLower(strings.TrimSpace(mustString(cmd, "action")))
		if action == "" {
			action, err = utils.PromptSelect("Resolve action", []string{
				api.ConflictResolutionKeepMine,
				api.ConflictResolutionKeepServer,
				api.ConflictResolutionKeepBoth,
				api.ConflictResolutionLater,
			})
			if err != nil {
				return err
			}
			action = strings.ToLower(strings.TrimSpace(action))
		}
		if action == api.ConflictResolutionKeepMine || action == api.ConflictResolutionKeepServer || action == api.ConflictResolutionKeepBoth || action == api.ConflictResolutionLater {
			// valid
		} else {
			return fmt.Errorf("invalid --action: %s", action)
		}
		yes, _ := cmd.Flags().GetBool("yes")
		if !yes {
			confirm, promptErr := utils.Prompt(fmt.Sprintf("Confirm resolve %q with action %q (type yes)", strings.TrimSpace(args[0]), action))
			if promptErr != nil {
				return promptErr
			}
			if !strings.EqualFold(strings.TrimSpace(confirm), "yes") {
				return fmt.Errorf("aborted")
			}
		}
		item, err := client.ResolveIssueConflict(args[0], action)
		if err != nil {
			return err
		}
		fmt.Println(utils.SuccessText(fmt.Sprintf("Resolved conflict %s on issue %s with %s", item.ID, item.TargetID, item.Resolution)))
		return nil
	},
}

func init() {
	issueCmd.AddCommand(issueConflictsCmd)
	issueConflictsCmd.AddCommand(issueConflictsListCmd, issueConflictsShowCmd, issueConflictsResolveCmd)

	issueConflictsListCmd.Flags().Bool("all", false, "Include resolved conflicts")
	issueConflictsListCmd.Flags().String("issue", "", "Filter by issue ID")
	issueConflictsListCmd.Flags().Bool("json", false, "Output JSON")

	issueConflictsShowCmd.Flags().Bool("all", false, "Allow showing resolved conflicts")
	issueConflictsShowCmd.Flags().Bool("json", false, "Output JSON")

	issueConflictsResolveCmd.Flags().String("action", "", "Resolution action: keep-mine|keep-server|keep-both|later")
	issueConflictsResolveCmd.Flags().Bool("yes", false, "Skip confirmation prompt")
}

func safe(v string) string {
	if strings.TrimSpace(v) == "" {
		return "<empty>"
	}
	return v
}

func coalesce(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
