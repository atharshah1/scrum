package cmd

import (
	"github.com/atharshah1/scrum/scrumX/cli/internal/api"
	"github.com/atharshah1/scrum/scrumX/cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfgStore       *config.Store
	overrideAPIURL string
)

var rootCmd = &cobra.Command{
	Use:   "scrumx",
	Short: "scrumX CLI",
	Long:  "scrumX is a production-grade CLI for working with projects, sprints, and issues.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cfgStore = config.NewStore("")
		cfg, err := cfgStore.Load()
		if err != nil {
			return err
		}
		if overrideAPIURL != "" {
			cfg.APIURL = overrideAPIURL
			if err := cfgStore.Save(cfg); err != nil {
				return err
			}
		}
		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&overrideAPIURL, "api-url", "", "Override API base URL (e.g. http://localhost:8080/api/v1)")
}

func newClient() (*api.Client, error) {
	if cfgStore == nil {
		cfgStore = config.NewStore("")
	}
	return api.NewClient(cfgStore), nil
}
