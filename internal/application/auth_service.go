package application

import (
	"context"
	"net/mail"
	"strings"

	"task-api/internal/domain/entities"
	"task-api/internal/domain/errrors"
	"task-api/internal/domain/ports"
)

type AuthService struct {
	users    ports.UserRepository
	password ports.PasswordHasher
	tokens   ports.TokenIssuer
}

func NewAuthService(users ports.UserRepository, password ports.PasswordHasher, tokens ports.TokenIssuer) *AuthService {
	return &AuthService{users: users, password: password, tokens: tokens}
}

func (s *AuthService) Register(ctx context.Context, email, password, name string) (*entities.User, error) {
	email, name = strings.ToLower(strings.TrimSpace(email)), strings.TrimSpace(name)
	parsedEmail, err := mail.ParseAddress(email)
	if err != nil || parsedEmail.Address != email || len(email) > 320 || len(password) < 8 || len(password) > 72 || len(name) == 0 || len(name) > 150 {
		return nil, errrors.ErrInvalid
	}
	hash, err := s.password.Hash(password)
	if err != nil {
		return nil, err
	}
	user := &entities.User{Email: email, Name: name, PasswordHash: hash}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	user, err := s.users.FindByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return "", errrors.ErrUnauthenticated
	}
	if err := s.password.Compare(user.PasswordHash, password); err != nil {
		return "", errrors.ErrUnauthenticated
	}
	return s.tokens.Issue(user.ID)
}
