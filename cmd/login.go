package cmd

import (
    "context"
    "fmt"
    "os/exec"

    "github.com/spf13/cobra"
    "github.com/atharshah1/scrum/internal/auth"
)

var authLoginCmd = &cobra.Command{
    Use: "login",
    RunE: func(cmd *cobra.Command, args []string) error {
        codeCh := make(chan string)
        srv := auth.StartCallbackServer(codeCh)
        defer srv.Shutdown(context.Background())

        url := fmt.Sprintf(
            "%s?audience=api.atlassian.com&client_id=%s&scope=%s&redirect_uri=%s&response_type=code&prompt=consent",
            auth.AuthURL,
            must(auth.ClientID()),
            auth.Scopes,
            auth.RedirectURI,
        )

        exec.Command("open", url).Start()

        code := <-codeCh

        token, err := auth.ExchangeCode(code)
        if err != nil {
            return err
        }

        return auth.SaveRefreshToken(token.RefreshToken)
    },
}

func must(v string, err error) string {
    if err != nil {
        panic(err)
    }
    return v
}

func init() {
    authCmd.AddCommand(authLoginCmd)
}

