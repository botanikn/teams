package ports

import (
	"context"
	"task-api/internal/domain"
)

type TeamRepository interface {
	CreateAtomic(context.Context, *domain.Team) error
	ListForUser(context.Context, int64) ([]domain.Team, error)
	RoleForUser(context.Context, int64, int64) (domain.Role, error)
	AddMemberAtomic(context.Context, int64, int64, domain.Role) error
	UpdateMemberRole(context.Context, int64, int64, domain.Role) error
	RemoveMember(context.Context, int64, int64) error
}
