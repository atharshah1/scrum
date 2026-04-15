package cmd

import "github.com/spf13/cobra"

var releaseCmd = &cobra.Command{Use: "release", Short: "Release commands"}

func init() { rootCmd.AddCommand(releaseCmd) }
