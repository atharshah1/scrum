package cmd

import (
"fmt"
"strings"

"github.com/atharshah1/scrum/scrumX/cli/internal/utils"
"github.com/spf13/cobra"
)

var boardCmd = &cobra.Command{Use: "board", Short: "Board commands"}

var boardViewCmd = &cobra.Command{
Use:   "view",
Short: "View board grouped by columns",
RunE: func(cmd *cobra.Command, args []string) error {
client, err := newClient()
if err != nil {
return err
}
cfg, err := cfgStore.Load()
if err != nil {
return err
}
boardID, _ := cmd.Flags().GetString("board-id")
if strings.TrimSpace(boardID) == "" {
boardID = cfg.CurrentBoardID
}
if strings.TrimSpace(boardID) == "" {
return fmt.Errorf("--board-id is required (or set active context board_id)")
}
sprintID, _ := cmd.Flags().GetString("sprint-id")
board, err := client.GetBoard(boardID, sprintID)
if err != nil {
return err
}
fmt.Println(utils.HeaderText(fmt.Sprintf("Board %s", board.BoardID)))
for _, col := range board.Columns {
fmt.Printf("\n%s (%d)\n", utils.HeaderText(col.Name), len(col.Issues))
if len(col.Issues) == 0 {
fmt.Println("  -")
continue
}
rows := make([][]string, 0, len(col.Issues))
for _, issue := range col.Issues {
rows = append(rows, []string{issue.ID, utils.StatusColor(issue.Status), issue.Priority, issue.Title})
}
utils.PrintTable([]string{"ID", "STATUS", "PRIORITY", "TITLE"}, rows)
}
return nil
},
}

func init() {
rootCmd.AddCommand(boardCmd)
boardCmd.AddCommand(boardViewCmd)
boardViewCmd.Flags().String("board-id", "", "Board UUID (defaults to active context)")
boardViewCmd.Flags().String("sprint-id", "", "Optional sprint UUID filter")
}
