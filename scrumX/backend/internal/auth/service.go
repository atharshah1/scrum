package auth

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
)

type User struct {
	ID       uuid.UUID `json:"id"`
	OrgID    uuid.UUID `json:"org_id"`
	Email    string    `json:"email"`
	Password string    `json:"-"`
	Role     string    `json:"role"`
}

type Service struct {
	mu            sync.RWMutex
	usersByEmail  map[string]User
	accessSecret  string
	refreshSecret string
}

func NewService(accessSecret, refreshSecret string) *Service {
	return &Service{usersByEmail: map[string]User{}, accessSecret: accessSecret, refreshSecret: refreshSecret}
}

func (s *Service) Register(_ context.Context, email, password string) (User, TokenPair, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.usersByEmail[email]; exists {
		return User{}, TokenPair{}, errors.New("email already registered")
	}
	user := User{ID: uuid.New(), OrgID: uuid.New(), Email: email, Password: password, Role: "Admin"}
	s.usersByEmail[email] = user
	tokens, err := GenerateTokens(user.ID, user.OrgID, user.Role, s.accessSecret, s.refreshSecret)
	return user, tokens, err
}

func (s *Service) Login(_ context.Context, email, password string) (User, TokenPair, error) {
	s.mu.RLock()
	user, exists := s.usersByEmail[email]
	s.mu.RUnlock()
	if !exists || user.Password != password {
		return User{}, TokenPair{}, errors.New("invalid credentials")
	}
	tokens, err := GenerateTokens(user.ID, user.OrgID, user.Role, s.accessSecret, s.refreshSecret)
	return user, tokens, err
}
