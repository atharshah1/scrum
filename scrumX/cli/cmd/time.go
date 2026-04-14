package cmd

import "github.com/spf13/cobra"

var timeCmd = &cobra.Command{Use: "time", Short: "Time tracking commands"}

func init() { rootCmd.AddCommand(timeCmd) }
