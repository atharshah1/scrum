package auth

import (
    "errors"
    "os"
)

const (
    AuthURL      = "https://auth.atlassian.com/authorize"
    TokenURL     = "https://auth.atlassian.com/oauth/token"
    RedirectURI  = "http://localhost:8085/callback"
    Scopes       = "read:jira-work write:jira-work manage:jira-project manage:jira-configuration read:jira-user offline_access"
)

func ClientID() (string, error) {
    v := os.Getenv("SCRUM_JIRA_CLIENT_ID")
    if v == "" {
        return "", errors.New("SCRUM_JIRA_CLIENT_ID not set")
    }
    return v, nil
}

func ClientSecret() (string, error) {
    v := os.Getenv("SCRUM_JIRA_CLIENT_SECRET")
    if v == "" {
        return "", errors.New("SCRUM_JIRA_CLIENT_SECRET not set")
    }
    return v, nil
}
