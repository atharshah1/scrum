package cmd

import (
	"fmt"
	"strings"

	"github.com/atharshah1/scrum/scrumX/cli/internal/utils"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate and store access tokens",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}

		cfg, err := cfgStore.Load()
		if err != nil {
			return err
		}

		openBrowser, _ := cmd.Flags().GetBool("open-browser")
		if openBrowser {
			_ = utils.OpenBrowser(strings.TrimSuffix(cfg.APIURL, "/") + "/auth/login")
		}

		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")
		if email == "" {
			email, err = utils.Prompt("Email")
			if err != nil {
				return err
			}
		}
		if password == "" {
			password, err = utils.PromptPassword("Password")
			if err != nil {
				return err
			}
		}

		user, tokens, err := client.Login(strings.TrimSpace(email), strings.TrimSpace(password))
		if err != nil {
			return err
		}

		cfg.AccessToken = tokens.AccessToken
		cfg.RefreshToken = tokens.RefreshToken
		cfg.OrgID = user.OrgID
		cfg.CurrentOrgID = user.OrgID
		cfg.UserID = user.ID
		if err := cfgStore.Save(cfg); err != nil {
			return err
		}

		fmt.Printf("Logged in as %s\n", user.Email)
		return nil
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear local authentication state",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := cfgStore.Load()
		if err != nil {
			return err
		}
		cfg.AccessToken = ""
		cfg.RefreshToken = ""
		cfg.UserID = ""
		cfg.OrgID = ""
		cfg.CurrentOrgID = ""
		if err := cfgStore.Save(cfg); err != nil {
			return err
		}
		fmt.Println("Logged out")
		return nil
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current authenticated identity",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient()
		if err != nil {
			return err
		}
		cfg, err := cfgStore.Load()
		if err != nil {
			return err
		}
		if cfg.AccessToken == "" {
			return fmt.Errorf("not logged in")
		}
		if err := client.WhoAmI(); err != nil {
			return err
		}
		fmt.Printf("user_id: %s\norg_id: %s\n", cfg.UserID, cfg.OrgID)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(loginCmd, logoutCmd, whoamiCmd)

	loginCmd.Flags().String("email", "", "Account email")
	loginCmd.Flags().String("password", "", "Account password")
	loginCmd.Flags().Bool("open-browser", true, "Open browser before login flow")
}
