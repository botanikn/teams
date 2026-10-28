package ports

import (
	"context"
	"task-api/internal/domain/entities"
)

type UserRepository interface {
	Create(context.Context, *entities.User) error
	FindByEmail(context.Context, string) (*entities.User, error)
	FindByID(context.Context, int64) (*entities.User, error)
}
