package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/cli/internal/api"
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

		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")
		if strings.TrimSpace(email) != "" || strings.TrimSpace(password) != "" {
			if strings.TrimSpace(email) == "" || strings.TrimSpace(password) == "" {
				return fmt.Errorf("email and password must be provided together")
			}
			user, tokens, err := client.Login(strings.TrimSpace(email), strings.TrimSpace(password))
			if err != nil {
				return err
			}
			cfg.AccessToken = tokens.AccessToken
			cfg.RefreshToken = tokens.RefreshToken
			cfg.OAuthClientID = ""
			cfg.OAuthScope = ""
			cfg.OrgID = user.OrgID
			cfg.CurrentOrgID = user.OrgID
			cfg.UserID = user.ID
			if err := cfgStore.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("Logged in as %s\n", user.Email)
			return nil
		}

		server, resultCh, err := startOAuthCallbackServer()
		if err != nil {
			return err
		}
		defer shutdownCallbackServer(server)

		state, err := randomURLToken(24)
		if err != nil {
			return err
		}
		verifier, err := randomURLToken(48)
		if err != nil {
			return err
		}
		authURL := buildAuthorizeURL(apiServerBaseURL(cfg.APIURL), state, verifier)
		openBrowser, _ := cmd.Flags().GetBool("open-browser")
		if openBrowser {
			if err := utils.OpenBrowser(authURL); err != nil {
				fmt.Printf("Open this URL in your browser:\n%s\n", authURL)
			}
		} else {
			fmt.Printf("Open this URL in your browser:\n%s\n", authURL)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		result, err := waitForOAuthCallback(ctx, resultCh)
		if err != nil {
			return err
		}
		if result.Error != "" {
			return fmt.Errorf("oauth authorization failed: %s", result.Error)
		}
		if result.Code == "" || result.State != state {
			return fmt.Errorf("invalid oauth callback response")
		}
		tokens, err := client.OAuthExchangeCode(oauthCLIClientID, result.Code, oauthRedirectURI, verifier)
		if err != nil {
			return err
		}
		userInfo, err := client.OAuthUserInfo(tokens.AccessToken)
		if err != nil {
			return err
		}
		cfg.AccessToken = tokens.AccessToken
		cfg.RefreshToken = tokens.RefreshToken
		cfg.OAuthClientID = oauthCLIClientID
		cfg.OAuthScope = tokens.Scope
		cfg.UserID = userInfo.Sub
		cfg.OrgID = userInfo.OrgID
		cfg.CurrentOrgID = userInfo.OrgID
		if err := cfgStore.Save(cfg); err != nil {
			return err
		}

		fmt.Printf("Logged in as %s\n", userInfo.Email)
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
		cfg.OAuthClientID = ""
		cfg.OAuthScope = ""
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

func apiServerBaseURL(apiURL string) string {
	return api.ParseServerBaseURL(apiURL)
}
