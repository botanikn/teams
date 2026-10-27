package ports

import (
	"context"
	"task-api/internal/domain"
)

type UserRepository interface {
	Create(context.Context, *domain.User) error
	FindByEmail(context.Context, string) (*domain.User, error)
	FindByID(context.Context, int64) (*domain.User, error)
}
