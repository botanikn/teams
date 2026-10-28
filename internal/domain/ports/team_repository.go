package ports

import (
	"context"
	"task-api/internal/domain/entities"
)

type TeamRepository interface {
	CreateAtomic(context.Context, *entities.Team) error
	ListForUser(context.Context, int64) ([]entities.Team, error)
	RoleForUser(context.Context, int64, int64) (entities.Role, error)
	AddMemberAtomic(context.Context, int64, int64, entities.Role) error
	UpdateMemberRole(context.Context, int64, int64, entities.Role) error
	RemoveMember(context.Context, int64, int64) error
}
