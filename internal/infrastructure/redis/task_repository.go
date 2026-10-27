package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"task-api/internal/domain"
	"task-api/internal/domain/ports"
)

const taskListTTL = 5 * time.Minute

type CachedTaskRepository struct {
	next  ports.TaskRepository
	cache *redis.Client
}

func NewCachedTaskRepository(next ports.TaskRepository, cache *redis.Client) *CachedTaskRepository {
	return &CachedTaskRepository{next: next, cache: cache}
}

func (r *CachedTaskRepository) Create(ctx context.Context, task *domain.Task, changedBy int64) error {
	if err := r.next.Create(ctx, task, changedBy); err != nil {
		return err
	}
	return r.InvalidateTaskLists(ctx, task.TeamID)
}

func (r *CachedTaskRepository) List(ctx context.Context, filter domain.TaskFilter) ([]domain.Task, error) {
	versionKey := fmt.Sprintf("task-list-version:%d", filter.TeamID)
	version, err := r.cache.Get(ctx, versionKey).Result()
	if err == redis.Nil {
		version = "0"
	} else if err != nil {
		return nil, fmt.Errorf("read task cache version: %w", err)
	}
	rawFilter, err := json.Marshal(filter)
	if err != nil {
		return nil, fmt.Errorf("encode task cache key: %w", err)
	}
	digest := sha256.Sum256(rawFilter)
	key := fmt.Sprintf("task-list:%d:%s:%s", filter.TeamID, version, hex.EncodeToString(digest[:]))
	cached, err := r.cache.Get(ctx, key).Bytes()
	if err == nil {
		var tasks []domain.Task
		if err := json.Unmarshal(cached, &tasks); err != nil {
			return nil, fmt.Errorf("decode cached task list: %w", err)
		}
		return tasks, nil
	}
	if err != redis.Nil {
		return nil, fmt.Errorf("read task cache: %w", err)
	}
	tasks, err := r.next.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(tasks)
	if err != nil {
		return nil, fmt.Errorf("encode task list cache: %w", err)
	}
	if err := r.cache.Set(ctx, key, encoded, taskListTTL).Err(); err != nil {
		return nil, fmt.Errorf("write task cache: %w", err)
	}
	return tasks, nil
}

func (r *CachedTaskRepository) FindByID(ctx context.Context, id int64) (*domain.Task, error) {
	return r.next.FindByID(ctx, id)
}

func (r *CachedTaskRepository) UpdateAtomic(ctx context.Context, task *domain.Task, actorID, expectedVersion int64, changes json.RawMessage) error {
	if err := r.next.UpdateAtomic(ctx, task, actorID, expectedVersion, changes); err != nil {
		return err
	}
	return r.InvalidateTaskLists(ctx, task.TeamID)
}

func (r *CachedTaskRepository) ListHistory(ctx context.Context, taskID int64) ([]domain.TaskHistory, error) {
	return r.next.ListHistory(ctx, taskID)
}

func (r *CachedTaskRepository) AddComment(ctx context.Context, comment *domain.TaskComment) error {
	return r.next.AddComment(ctx, comment)
}

func (r *CachedTaskRepository) ListComments(ctx context.Context, taskID int64) ([]domain.TaskComment, error) {
	return r.next.ListComments(ctx, taskID)
}

func (r *CachedTaskRepository) TeamStats(ctx context.Context, teamID int64) (*domain.TeamStats, error) {
	return r.next.TeamStats(ctx, teamID)
}

func (r *CachedTaskRepository) InvalidateTaskLists(ctx context.Context, teamID int64) error {
	if err := r.cache.Incr(ctx, fmt.Sprintf("task-list-version:%d", teamID)).Err(); err != nil {
		return fmt.Errorf("invalidate task cache: %w", err)
	}
	return nil
}
