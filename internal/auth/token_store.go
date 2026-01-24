package auth

import "github.com/zalando/go-keyring"

const (
    keyringService = "scrum-cli"
    tokenKey       = "jira-refresh-token"
)

func SaveRefreshToken(token string) error {
    return keyring.Set(keyringService, tokenKey, token)
}

func LoadRefreshToken() (string, error) {
    return keyring.Get(keyringService, tokenKey)
}

func DeleteRefreshToken() error {
    return keyring.Delete(keyringService, tokenKey)
}
