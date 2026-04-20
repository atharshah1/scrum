package cmd

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	oauthCLIClientID  = "scrumx-cli"
	oauthRedirectURI  = "http://127.0.0.1:8787/callback"
	oauthDefaultScope = "read:issues write:issues read:projects write:projects read:releases write:releases admin:org automation:execute"
)

type callbackResult struct {
	Code  string
	State string
	Error string
}

func startOAuthCallbackServer() (*http.Server, <-chan callbackResult, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:8787")
	if err != nil {
		return nil, nil, err
	}
	resultCh := make(chan callbackResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		result := callbackResult{
			Code:  strings.TrimSpace(r.URL.Query().Get("code")),
			State: strings.TrimSpace(r.URL.Query().Get("state")),
			Error: strings.TrimSpace(r.URL.Query().Get("error")),
		}
		select {
		case resultCh <- result:
		default:
		}
		_, _ = w.Write([]byte("Login complete. You can close this window."))
	})
	srv := &http.Server{Handler: mux}
	go func() {
		_ = srv.Serve(listener)
	}()
	return srv, resultCh, nil
}

func randomURLToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func buildAuthorizeURL(baseURL, state, verifier string) string {
	q := url.Values{}
	q.Set("client_id", oauthCLIClientID)
	q.Set("redirect_uri", oauthRedirectURI)
	q.Set("response_type", "code")
	q.Set("scope", oauthDefaultScope)
	q.Set("state", state)
	q.Set("code_challenge", pkceChallenge(verifier))
	q.Set("code_challenge_method", "S256")
	return fmt.Sprintf("%s/oauth/authorize?%s", strings.TrimRight(baseURL, "/"), q.Encode())
}

func waitForOAuthCallback(ctx context.Context, resultCh <-chan callbackResult) (callbackResult, error) {
	select {
	case result := <-resultCh:
		return result, nil
	case <-ctx.Done():
		return callbackResult{}, ctx.Err()
	}
}

func shutdownCallbackServer(server *http.Server) {
	if server == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}
