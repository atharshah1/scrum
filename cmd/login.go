package cmd

import (
    "context"
    "fmt"
	"net/url"
    "os/exec"
    "strings"
	"runtime"

    "github.com/spf13/cobra"
    "github.com/atharshah1/scrum/internal/auth"
)

var authLoginCmd = &cobra.Command{
    Use: "login",
    RunE: func(cmd *cobra.Command, args []string) error {
        clientID, err := auth.ClientID()
        if err != nil {
            return fmt.Errorf("missing configuration: %w. Please set SCRUM_JIRA_CLIENT_ID", err)
        }

        codeCh := make(chan string)
        srv := auth.StartCallbackServer(codeCh)
        defer srv.Shutdown(context.Background())

		u, _ := url.Parse(auth.AuthURL)
		q := u.Query()
		q.Set("audience", "api.atlassian.com")
		q.Set("client_id", clientID)
		q.Set("scope", auth.Scopes)
		q.Set("redirect_uri", auth.RedirectURI)
		q.Set("response_type", "code")
		q.Set("prompt", "consent")
		u.RawQuery = strings.ReplaceAll(q.Encode(), "+", "%20")

		switch runtime.GOOS {
		case "linux":
			exec.Command("xdg-open", u.String()).Start()
		case "windows":
			exec.Command("rundll32", "url.dll,FileProtocolHandler", u.String()).Start()
		case "darwin":
			exec.Command("open", u.String()).Start()
		default:
			fmt.Printf("Please open this URL in your browser:\n%s\n", u.String())
		}

        code := <-codeCh

        token, err := auth.ExchangeCode(code)
        if err != nil {
            return err
        }

        fmt.Printf("Login Successful!\nGranted Scopes: %s\n", token.Scope)

        return auth.SaveRefreshToken(token.RefreshToken)
    },
}

func init() {
    authCmd.AddCommand(authLoginCmd)
}
