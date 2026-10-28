package ports

import (
	"context"
	"encoding/json"
	"task-api/internal/domain/entities"
)

type TaskRepository interface {
	Create(context.Context, *entities.Task, int64) error
	List(context.Context, entities.TaskFilter) ([]entities.Task, error)
	FindByID(context.Context, int64) (*entities.Task, error)
	UpdateAtomic(context.Context, *entities.Task, int64, int64, json.RawMessage) error
	CheckVersion(context.Context, int64, int64) error
}

type TaskHistoryRepository interface {
	ListHistory(context.Context, int64) ([]entities.TaskHistory, error)
}

type TaskCommentRepository interface {
	AddComment(context.Context, *entities.TaskComment) error
	ListComments(context.Context, int64) ([]entities.TaskComment, error)
}

type TeamStatsRepository interface {
	TeamStats(context.Context, int64) (*entities.TeamStats, error)
}
