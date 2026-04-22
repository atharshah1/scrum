package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/cli/internal/api"
	"github.com/atharshah1/scrum/scrumX/cli/internal/config"
	"github.com/atharshah1/scrum/scrumX/internal/offline"
	"github.com/spf13/cobra"
)

var demoCmd = &cobra.Command{
	Use:     "demo",
	Aliases: []string{"start"},
	Short:   "Create an instant demo project and preload a conflict",
	Long: `Creates the fastest proof loop in one command:
  1. create an instant demo project
  2. create starter issues
  3. preload a conflict for review`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		cfg, err := cfgStore.Load()
		if err != nil {
			return err
		}

		project, err := client.CreateProject(api.CreateProjectInput{
			Key:  demoProjectKey(),
			Name: "Instant demo project",
		})
		if err != nil {
			return err
		}

		starterIssue, err := client.CreateIssueSmart(api.CreateIssueInput{
			ProjectID:   project.ID,
			Title:       "Demo: create or edit work fast",
			Description: "Use this issue first to prove the create → edit loop.",
			IssueType:   "task",
			Priority:    "medium",
		})
		if err != nil {
			return err
		}
		conflictIssue, err := client.CreateIssueSmart(api.CreateIssueInput{
			ProjectID:   project.ID,
			Title:       "Demo: resolve the preloaded conflict",
			Description: "Open this issue in the web app to try the recommended resolution.",
			IssueType:   "task",
			Priority:    "high",
			Labels:      []string{"demo-conflict"},
		})
		if err != nil {
			return err
		}

		if err := seedDemoConflict(cfg, project, conflictIssue); err != nil {
			return err
		}

		cfg.CurrentProjectID = project.ID
		if err := cfgStore.Save(cfg); err != nil {
			return err
		}

		fmt.Printf("Created demo project %s (%s)\n", project.Name, project.ID)
		fmt.Printf("Starter issue: %s\n", starterIssue.ID)
		fmt.Printf("Conflict issue: %s\n", conflictIssue.ID)
		fmt.Println("Conflict simulation is ready.")
		fmt.Println("Next:")
		fmt.Println("  sx sync status")
		fmt.Printf("  sx i cf show %s\n", conflictIssue.ID)
		fmt.Printf("  open the web issue detail for %s and use the recommended action button\n", conflictIssue.ID)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(demoCmd)
}

func demoProjectKey() string {
	var suffix [2]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return fmt.Sprintf("DEMO%d", time.Now().UnixNano()%1000000)
	}
	return "DEMO" + strings.ToUpper(hex.EncodeToString(suffix[:]))
}

func seedDemoConflict(cfg config.Config, project api.Project, issue api.Issue) error {
	scope := strings.TrimSpace(cfg.APIURL) + "|" + strings.TrimSpace(cfg.CurrentOrgID) + "|" + strings.TrimSpace(cfg.OrgID)
	store, err := offline.NewStore(filepath.Dir(cfgStore.Path()), scope)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	localIssue := issue
	localIssue.Title = issue.Title + " (local edit)"
	localIssue.Description = strings.TrimSpace(issue.Description + "\n\nLocal note: keep this edit if you want the fastest safe resolution.")
	localIssue.UpdatedAt = now

	serverSnapshot, _ := json.Marshal(issue)
	localSnapshot, _ := json.Marshal(localIssue)
	_, err = store.Update(func(state *offline.State) error {
		state.Projects[project.ID] = offline.Project{ID: project.ID, Name: project.Name}
		offline.UpsertConflict(state, offline.ConflictRecord{
			Entity:         "issue",
			TargetID:       issue.ID,
			ActorID:        strings.TrimSpace(cfg.UserID),
			LocalSnapshot:  localSnapshot,
			ServerSnapshot: serverSnapshot,
			Fields: []offline.ConflictField{
				{Field: "title", LocalValue: localIssue.Title, ServerValue: issue.Title},
				{Field: "description", LocalValue: localIssue.Description, ServerValue: issue.Description},
			},
			CreatedAt: now,
		})
		return nil
	})
	return err
}
