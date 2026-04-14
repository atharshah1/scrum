package auth

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           uuid.UUID `json:"id"`
	OrgID        uuid.UUID `json:"org_id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
}

type Service struct {
	mu            sync.RWMutex
	usersByEmail  map[string]User
	usersByID     map[uuid.UUID]User
	accessSecret  string
	refreshSecret string
}

func NewService(accessSecret, refreshSecret string) *Service {
	return &Service{usersByEmail: map[string]User{}, usersByID: map[uuid.UUID]User{}, accessSecret: accessSecret, refreshSecret: refreshSecret}
}

func (s *Service) Register(_ context.Context, email, password string) (User, TokenPair, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.usersByEmail[email]; exists {
		return User{}, TokenPair{}, errors.New("email already registered")
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, TokenPair{}, err
	}
	user := User{ID: uuid.New(), OrgID: uuid.New(), Email: email, PasswordHash: string(passwordHash), Role: "Admin"}
	s.usersByEmail[email] = user
	s.usersByID[user.ID] = user
	tokens, err := GenerateTokens(user.ID, user.OrgID, user.Role, s.accessSecret, s.refreshSecret)
	return user, tokens, err
}

func (s *Service) Login(_ context.Context, email, password string) (User, TokenPair, error) {
	s.mu.RLock()
	user, exists := s.usersByEmail[email]
	s.mu.RUnlock()
	if !exists || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return User{}, TokenPair{}, errors.New("invalid credentials")
	}
	tokens, err := GenerateTokens(user.ID, user.OrgID, user.Role, s.accessSecret, s.refreshSecret)
	return user, tokens, err
}

func (s *Service) GetByID(_ context.Context, userID uuid.UUID) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.usersByID[userID]
	if !ok {
		return User{}, errors.New("user not found")
	}
	return user, nil
}
