package cmd

import "github.com/spf13/cobra"

var automationCmd = &cobra.Command{Use: "automation", Short: "Automation commands"}

func init() { rootCmd.AddCommand(automationCmd) }
