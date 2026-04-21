/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/spf13/cobra"
)

// issueCmd represents the issue command
var issueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Legacy source issue commands",
	Long: `Use these commands when you need to inspect or move source-system work during migration.
For primary issue workflows, use scrumx issue ...`,
}

func init() {
	rootCmd.AddCommand(issueCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// issueCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// issueCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
