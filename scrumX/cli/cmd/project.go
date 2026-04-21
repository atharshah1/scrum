package cmd

import "github.com/spf13/cobra"

var projectCmd = &cobra.Command{Use: "project", Aliases: []string{"p"}, Short: "Project commands"}

func init() { rootCmd.AddCommand(projectCmd) }
