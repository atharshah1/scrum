package auth

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type TokenResponse struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    Scope        string `json:"scope"`
}

func ExchangeCode(code string) (*TokenResponse, error) {
    clientID, err := ClientID()
    if err != nil {
        return nil, err
    }

    secret, err := ClientSecret()
    if err != nil {
        return nil, err
    }

    body := map[string]string{
        "grant_type":    "authorization_code",
        "client_id":     clientID,
        "client_secret": secret,
        "code":          code,
        "redirect_uri":  RedirectURI,
    }

    b, _ := json.Marshal(body)

    req, _ := http.NewRequest("POST", TokenURL, bytes.NewBuffer(b))
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var tr TokenResponse
    err = json.NewDecoder(resp.Body).Decode(&tr)
    return &tr, err
}

func RefreshAccessToken() (*TokenResponse, error) {
    refreshToken, err := LoadRefreshToken()
    if err != nil {
        return nil, err
    }

    clientID, err := ClientID()
    if err != nil {
        return nil, err
    }

    secret, err := ClientSecret()
    if err != nil {
        return nil, err
    }

    body := map[string]string{
        "grant_type":    "refresh_token",
        "client_id":     clientID,
        "client_secret": secret,
        "refresh_token": refreshToken,
    }

    b, _ := json.Marshal(body)

    req, _ := http.NewRequest("POST", TokenURL, bytes.NewBuffer(b))
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("failed to refresh token: %s", resp.Status)
    }

    var tr TokenResponse
    err = json.NewDecoder(resp.Body).Decode(&tr)
    if err != nil {
        return nil, err
    }

    if tr.RefreshToken != "" {
        _ = SaveRefreshToken(tr.RefreshToken)
    }

    return &tr, nil
}

func GetCloudID() (string, error) {
    // 1. Try to load from cache first
    if id, err := LoadSite(); err == nil && id != "" {
        return id, nil
    }

    // 2. If not found, get a fresh token to fetch it from API
    token, err := RefreshAccessToken()
    if err != nil {
        return "", err
    }

    req, _ := http.NewRequest("GET", "https://api.atlassian.com/oauth/token/accessible-resources", nil)
    req.Header.Set("Authorization", "Bearer "+token.AccessToken)
    req.Header.Set("Accept", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("failed to get accessible resources: %s", resp.Status)
    }

    var resources []struct {
        ID string `json:"id"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&resources); err != nil {
        return "", err
    }

    if len(resources) == 0 {
        return "", fmt.Errorf("no accessible resources found")
    }

    // 3. Save the ID for next time
    cloudID := resources[0].ID
    _ = SaveSite(cloudID)

    return cloudID, nil
}