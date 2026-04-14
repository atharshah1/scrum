package auth

import (
"time"

"github.com/golang-jwt/jwt/v5"
"github.com/google/uuid"
)

type TokenPair struct {
AccessToken  string `json:"access_token"`
RefreshToken string `json:"refresh_token"`
TokenType    string `json:"token_type"`
ExpiresIn    int64  `json:"expires_in"`
}

func GenerateTokens(userID uuid.UUID, orgID uuid.UUID, role, accessSecret, refreshSecret string) (TokenPair, error) {
now := time.Now().UTC()
accessExpiry := now.Add(15 * time.Minute)
refreshExpiry := now.Add(7 * 24 * time.Hour)

access := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
"sub":    userID.String(),
"org_id": orgID.String(),
"role":   role,
"exp":    accessExpiry.Unix(),
"iat":    now.Unix(),
})
refresh := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
"sub": userID.String(),
"exp": refreshExpiry.Unix(),
"iat": now.Unix(),
})

accessToken, err := access.SignedString([]byte(accessSecret))
if err != nil {
return TokenPair{}, err
}
refreshToken, err := refresh.SignedString([]byte(refreshSecret))
if err != nil {
return TokenPair{}, err
}

return TokenPair{AccessToken: accessToken, RefreshToken: refreshToken, TokenType: "Bearer", ExpiresIn: int64((15 * time.Minute).Seconds())}, nil
}
