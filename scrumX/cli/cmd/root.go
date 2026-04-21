package cmd

import (
	"fmt"
	"os"

	"github.com/atharshah1/scrum/scrumX/cli/internal/api"
	"github.com/atharshah1/scrum/scrumX/cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfgStore       *config.Store
	overrideAPIURL string
)

var rootCmd = &cobra.Command{
	Use:     "scrumx",
	Aliases: []string{"sx"},
	Short:   "Developer-first, offline-safe task CLI",
	Long: `scrumX is built for fast task work with offline safety, sync visibility,
and Git-like conflict protection.

Core daily flow:
  sx i c "fix login bug p1 assign me #auth"
  sx i l --status in_progress
  sx sync st
  sx i cf l
  sx i cf r <issue-id>`,
	Example: "  sx i c \"fix login bug p1 assign me #auth\"\n  sx i l --status open\n  sx sync st\n  sx i cf l\n  sx i cf r <issue-id>",
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

var completionCmd = &cobra.Command{
	Use:       "completion [bash|zsh|fish|powershell]",
	Short:     "Generate shell completion script",
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	Example:   "  scrumx completion bash > /etc/bash_completion.d/scrumx\n  scrumx completion zsh > ~/.zsh/completions/_scrumx",
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		default:
			return fmt.Errorf("unsupported shell: %s", args[0])
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&overrideAPIURL, "api-url", "", "Override API base URL (e.g. http://localhost:8080/api/v1)")
	rootCmd.AddCommand(completionCmd)
}

func newClient() (*api.Client, error) {
	if cfgStore == nil {
		cfgStore = config.NewStore("")
	}
	return api.NewClient(cfgStore), nil
}
