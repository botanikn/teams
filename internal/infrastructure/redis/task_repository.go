package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"task-api/internal/domain/entities"
	"task-api/internal/domain/ports"
)

const taskListTTL = 5 * time.Minute
const taskTTL = time.Minute
const taskHistoryTTL = time.Minute
const taskCommentsTTL = time.Minute

type CachedTaskRepository struct {
	next     ports.TaskRepository
	history  ports.TaskHistoryRepository
	comments ports.TaskCommentRepository
	stats    ports.TeamStatsRepository
	cache    *redis.Client
}

func NewCachedTaskRepository(next ports.TaskRepository, history ports.TaskHistoryRepository, comments ports.TaskCommentRepository, stats ports.TeamStatsRepository, cache *redis.Client) *CachedTaskRepository {
	return &CachedTaskRepository{next: next, history: history, comments: comments, stats: stats, cache: cache}
}

func (r *CachedTaskRepository) Create(ctx context.Context, task *entities.Task, changedBy int64) error {
	if err := r.next.Create(ctx, task, changedBy); err != nil {
		return err
	}
	r.invalidateTaskListsAfterWrite(ctx, task.TeamID)
	return nil
}

func (r *CachedTaskRepository) List(ctx context.Context, filter entities.TaskFilter) ([]entities.Task, error) {
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
	key := fmt.Sprintf("task-list:v2:%d:%s:%s", filter.TeamID, version, hex.EncodeToString(digest[:]))
	cached, err := r.cache.Get(ctx, key).Bytes()
	if err == nil {
		tasks, err := decodeCachedTasks(cached)
		if err != nil {
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
	encoded, err := encodeCachedTasks(tasks)
	if err != nil {
		return nil, fmt.Errorf("encode task list cache: %w", err)
	}
	if err := r.cache.Set(ctx, key, encoded, taskListTTL).Err(); err != nil {
		return nil, fmt.Errorf("write task cache: %w", err)
	}
	return tasks, nil
}

func (r *CachedTaskRepository) FindByID(ctx context.Context, id int64) (*entities.Task, error) {
	key := fmt.Sprintf("task:%d", id)
	cached, err := r.cache.Get(ctx, key).Bytes()
	if err == nil {
		task, err := decodeCachedTask(cached)
		if err != nil {
			return nil, fmt.Errorf("decode cached task: %w", err)
		}
		return task, nil
	}
	if err != redis.Nil {
		return nil, fmt.Errorf("read task cache: %w", err)
	}

	task, err := r.next.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	encoded, err := encodeCachedTask(task)
	if err != nil {
		return nil, fmt.Errorf("encode task cache: %w", err)
	}
	if err := r.cache.Set(ctx, key, encoded, taskTTL).Err(); err != nil {
		return nil, fmt.Errorf("write task cache: %w", err)
	}
	return task, nil
}

func (r *CachedTaskRepository) UpdateAtomic(ctx context.Context, task *entities.Task, actorID, expectedVersion int64, changes json.RawMessage) error {
	if err := r.next.UpdateAtomic(ctx, task, actorID, expectedVersion, changes); err != nil {
		return err
	}
	for _, err := range []error{
		r.invalidateTask(ctx, task.ID),
		r.invalidateTaskHistory(ctx, task.ID),
		r.InvalidateTaskLists(ctx, task.TeamID),
	} {
		if err != nil {
			slog.ErrorContext(ctx, "invalidate task cache after committed update", "task_id", task.ID, "error", err)
		}
	}
	return nil
}

func (r *CachedTaskRepository) CheckVersion(ctx context.Context, taskID, expectedVersion int64) error {
	return r.next.CheckVersion(ctx, taskID, expectedVersion)
}

func (r *CachedTaskRepository) ListHistory(ctx context.Context, taskID int64) ([]entities.TaskHistory, error) {
	key := fmt.Sprintf("task-history:%d", taskID)
	cached, err := r.cache.Get(ctx, key).Bytes()
	if err == nil {
		var history []entities.TaskHistory
		if err := json.Unmarshal(cached, &history); err != nil {
			return nil, fmt.Errorf("decode cached task history: %w", err)
		}
		return history, nil
	}
	if err != redis.Nil {
		return nil, fmt.Errorf("read task history cache: %w", err)
	}

	history, err := r.history.ListHistory(ctx, taskID)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(history)
	if err != nil {
		return nil, fmt.Errorf("encode task history cache: %w", err)
	}
	if err := r.cache.Set(ctx, key, encoded, taskHistoryTTL).Err(); err != nil {
		return nil, fmt.Errorf("write task history cache: %w", err)
	}
	return history, nil
}

func (r *CachedTaskRepository) AddComment(ctx context.Context, comment *entities.TaskComment) error {
	if err := r.comments.AddComment(ctx, comment); err != nil {
		return err
	}
	if err := r.invalidateTaskComments(ctx, comment.TaskID); err != nil {
		slog.ErrorContext(ctx, "invalidate task comments cache after committed comment", "task_id", comment.TaskID, "error", err)
	}
	return nil
}

func (r *CachedTaskRepository) ListComments(ctx context.Context, taskID int64) ([]entities.TaskComment, error) {
	key := fmt.Sprintf("task-comments:%d", taskID)
	cached, err := r.cache.Get(ctx, key).Bytes()
	if err == nil {
		var comments []entities.TaskComment
		if err := json.Unmarshal(cached, &comments); err != nil {
			return nil, fmt.Errorf("decode cached task comments: %w", err)
		}
		return comments, nil
	}
	if err != redis.Nil {
		return nil, fmt.Errorf("read task comments cache: %w", err)
	}

	comments, err := r.comments.ListComments(ctx, taskID)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(comments)
	if err != nil {
		return nil, fmt.Errorf("encode task comments cache: %w", err)
	}
	if err := r.cache.Set(ctx, key, encoded, taskCommentsTTL).Err(); err != nil {
		return nil, fmt.Errorf("write task comments cache: %w", err)
	}
	return comments, nil
}

func (r *CachedTaskRepository) TeamStats(ctx context.Context, teamID int64) (*entities.TeamStats, error) {
	return r.stats.TeamStats(ctx, teamID)
}

func (r *CachedTaskRepository) InvalidateTaskLists(ctx context.Context, teamID int64) error {
	if err := r.cache.Incr(ctx, fmt.Sprintf("task-list-version:%d", teamID)).Err(); err != nil {
		return fmt.Errorf("invalidate task cache: %w", err)
	}
	return nil
}

type cachedTask struct {
	Task      *entities.Task `json:"task"`
	CreatedBy int64          `json:"created_by_id"`
}

func encodeCachedTask(task *entities.Task) ([]byte, error) {
	return json.Marshal(cachedTask{Task: task, CreatedBy: task.CreatedBy})
}

func decodeCachedTask(raw []byte) (*entities.Task, error) {
	var cached cachedTask
	if err := json.Unmarshal(raw, &cached); err != nil {
		return nil, err
	}
	if cached.Task == nil {
		return nil, fmt.Errorf("cached task is missing")
	}
	cached.Task.CreatedBy = cached.CreatedBy
	return cached.Task, nil
}

func encodeCachedTasks(tasks []entities.Task) ([]byte, error) {
	cached := make([]cachedTask, len(tasks))
	for i := range tasks {
		cached[i] = cachedTask{Task: &tasks[i], CreatedBy: tasks[i].CreatedBy}
	}
	return json.Marshal(cached)
}

func decodeCachedTasks(raw []byte) ([]entities.Task, error) {
	var cached []cachedTask
	if err := json.Unmarshal(raw, &cached); err != nil {
		return nil, err
	}
	tasks := make([]entities.Task, len(cached))
	for i := range cached {
		if cached[i].Task == nil {
			return nil, fmt.Errorf("cached task at index %d is missing", i)
		}
		tasks[i] = *cached[i].Task
		tasks[i].CreatedBy = cached[i].CreatedBy
	}
	return tasks, nil
}

func (r *CachedTaskRepository) invalidateTask(ctx context.Context, taskID int64) error {
	return r.deleteCacheKey(ctx, fmt.Sprintf("task:%d", taskID), "task")
}

func (r *CachedTaskRepository) invalidateTaskHistory(ctx context.Context, taskID int64) error {
	return r.deleteCacheKey(ctx, fmt.Sprintf("task-history:%d", taskID), "task history")
}

func (r *CachedTaskRepository) invalidateTaskComments(ctx context.Context, taskID int64) error {
	return r.deleteCacheKey(ctx, fmt.Sprintf("task-comments:%d", taskID), "task comments")
}

func (r *CachedTaskRepository) deleteCacheKey(ctx context.Context, key, resource string) error {
	if err := r.cache.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("invalidate %s cache: %w", resource, err)
	}
	return nil
}

func (r *CachedTaskRepository) invalidateTaskListsAfterWrite(ctx context.Context, teamID int64) {
	if err := r.InvalidateTaskLists(ctx, teamID); err != nil {
		slog.ErrorContext(ctx, "invalidate task list cache after committed write", "team_id", teamID, "error", err)
	}
}
