package middleware

import "github.com/google/uuid"

type contextKey string

const (
userIDKey contextKey = "userID"
orgIDKey  contextKey = "orgID"
roleKey   contextKey = "role"
)

func SetUserID(c interface{ Locals(string, ...interface{}) interface{} }, id uuid.UUID) {
c.Locals(string(userIDKey), id)
}

func SetOrgID(c interface{ Locals(string, ...interface{}) interface{} }, id uuid.UUID) {
c.Locals(string(orgIDKey), id)
}

func SetRole(c interface{ Locals(string, ...interface{}) interface{} }, role string) {
c.Locals(string(roleKey), role)
}
