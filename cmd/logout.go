package cmd

import (
	"fmt"

	"github.com/atharshah1/scrum/internal/auth"
	"github.com/spf13/cobra"
)

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out and clear stored credentials",
	Long: `Securely remove stored OAuth tokens from the system keyring
and delete local configuration files (project context, site ID).`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Logging out...")

		// 1. Delete Refresh Token
		if err := auth.DeleteRefreshToken(); err != nil {
			// Don't fail if token is already gone
			fmt.Printf("Note: Token cleanup: %v\n", err)
		}

		// 2. Delete Cached Files
		_ = auth.DeleteSite()
		_ = auth.DeleteProject()

		fmt.Println("Logged out successfully. All tokens and cache cleared.")
	},
}

func init() {
	authCmd.AddCommand(authLogoutCmd)
}