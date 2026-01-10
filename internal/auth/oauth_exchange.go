package auth

import (
    "bytes"
    "encoding/json"
    "net/http"
)

type TokenResponse struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
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
