package auth

import (
	"errors"
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
		"sub":    userID.String(),
		"org_id": orgID.String(),
		"role":   role,
		"jti":    uuid.NewString(),
		"exp":    refreshExpiry.Unix(),
		"iat":    now.Unix(),
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

func ParseRefreshTokenMeta(refreshToken, secret string) (tokenID string, exp int64, err error) {
	claims, err := ParseRefreshClaims(refreshToken, secret)
	if err != nil {
		return "", 0, err
	}
	return claims.TokenID, claims.ExpiresAt, nil
}

type RefreshClaims struct {
	UserID    uuid.UUID
	OrgID     uuid.UUID
	Role      string
	TokenID   string
	ExpiresAt int64
}

func ParseRefreshClaims(refreshToken, secret string) (RefreshClaims, error) {
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	token, err := parser.Parse(refreshToken, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return RefreshClaims{}, errors.New("invalid refresh token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return RefreshClaims{}, errors.New("invalid token claims")
	}
	jti, _ := claims["jti"].(string)
	if jti == "" {
		return RefreshClaims{}, errors.New("invalid refresh token id")
	}
	expFloat, ok := claims["exp"].(float64)
	if !ok {
		return RefreshClaims{}, errors.New("invalid expiry")
	}
	userID, err := uuid.Parse(asString(claims["sub"]))
	if err != nil {
		return RefreshClaims{}, errors.New("invalid subject claim")
	}
	orgID, err := uuid.Parse(asString(claims["org_id"]))
	if err != nil {
		return RefreshClaims{}, errors.New("invalid org claim")
	}
	return RefreshClaims{
		UserID:    userID,
		OrgID:     orgID,
		Role:      asString(claims["role"]),
		TokenID:   jti,
		ExpiresAt: int64(expFloat),
	}, nil
}

func asString(value interface{}) string {
	if value == nil {
		return ""
	}
	if v, ok := value.(string); ok {
		return v
	}
	return ""
}
