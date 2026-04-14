package cmd

import "github.com/spf13/cobra"

var sprintCmd = &cobra.Command{Use: "sprint", Short: "Sprint commands"}

func init() { rootCmd.AddCommand(sprintCmd) }
