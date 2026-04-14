package cmd

import (
"fmt"
"strings"

"github.com/atharshah1/scrum/scrumX/cli/internal/utils"
"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{Use: "context", Short: "Manage active org/project/board context"}

var contextViewCmd = &cobra.Command{
Use:   "view",
Short: "View active context",
RunE: func(cmd *cobra.Command, args []string) error {
cfg, err := cfgStore.Load()
if err != nil {
return err
}
fmt.Printf("api_url: %s\norg_id: %s\nproject_id: %s\nboard_id: %s\n",
cfg.APIURL,
nonEmpty(cfg.CurrentOrgID, cfg.OrgID, "<unset>"),
nonEmpty(cfg.CurrentProjectID, "<unset>"),
nonEmpty(cfg.CurrentBoardID, "<unset>"),
)
return nil
},
}

var contextSwitchCmd = &cobra.Command{
Use:   "switch",
Short: "Switch active context",
RunE: func(cmd *cobra.Command, args []string) error {
cfg, err := cfgStore.Load()
if err != nil {
return err
}
orgID, _ := cmd.Flags().GetString("org-id")
projectID, _ := cmd.Flags().GetString("project-id")
boardID, _ := cmd.Flags().GetString("board-id")
interactive, _ := cmd.Flags().GetBool("interactive")
if interactive {
orgID, err = utils.PromptOptional("Org ID", nonEmpty(orgID, cfg.CurrentOrgID, cfg.OrgID))
if err != nil {
return err
}
projectID, err = utils.PromptOptional("Project ID", nonEmpty(projectID, cfg.CurrentProjectID))
if err != nil {
return err
}
boardID, err = utils.PromptOptional("Board ID", nonEmpty(boardID, cfg.CurrentBoardID))
if err != nil {
return err
}
}
if cmd.Flags().Changed("org-id") || interactive {
cfg.CurrentOrgID = strings.TrimSpace(orgID)
}
if cmd.Flags().Changed("project-id") || interactive {
cfg.CurrentProjectID = strings.TrimSpace(projectID)
}
if cmd.Flags().Changed("board-id") || interactive {
cfg.CurrentBoardID = strings.TrimSpace(boardID)
}
if cfg.CurrentOrgID == "" {
cfg.CurrentOrgID = cfg.OrgID
}
if err := cfgStore.Save(cfg); err != nil {
return err
}
fmt.Println(utils.SuccessText("Context updated"))
return nil
},
}

func init() {
rootCmd.AddCommand(contextCmd)
contextCmd.AddCommand(contextViewCmd, contextSwitchCmd)
contextSwitchCmd.Flags().String("org-id", "", "Active org UUID")
contextSwitchCmd.Flags().String("project-id", "", "Active project UUID")
contextSwitchCmd.Flags().String("board-id", "", "Active board UUID")
contextSwitchCmd.Flags().Bool("interactive", false, "Prompt to switch context")
}

func nonEmpty(values ...string) string {
for _, value := range values {
if strings.TrimSpace(value) != "" {
return strings.TrimSpace(value)
}
}
return ""
}
