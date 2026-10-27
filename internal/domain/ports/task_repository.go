package ports

import (
	"context"
	"encoding/json"
	"task-api/internal/domain"
)

type TaskRepository interface {
	Create(context.Context, *domain.Task, int64) error
	List(context.Context, domain.TaskFilter) ([]domain.Task, error)
	FindByID(context.Context, int64) (*domain.Task, error)
	UpdateAtomic(context.Context, *domain.Task, int64, int64, json.RawMessage) error
	ListHistory(context.Context, int64) ([]domain.TaskHistory, error)
	AddComment(context.Context, *domain.TaskComment) error
	ListComments(context.Context, int64) ([]domain.TaskComment, error)
	TeamStats(context.Context, int64) (*domain.TeamStats, error)
}
