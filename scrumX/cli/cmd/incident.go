package cmd

import "github.com/spf13/cobra"

var incidentCmd = &cobra.Command{Use: "incident", Short: "Incident commands"}

func init() { rootCmd.AddCommand(incidentCmd) }
