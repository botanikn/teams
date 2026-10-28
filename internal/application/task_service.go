package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"task-api/internal/domain"
	"task-api/internal/domain/ports"
)

type TaskService struct {
	tasks    ports.TaskRepository
	history  ports.TaskHistoryRepository
	comments ports.TaskCommentRepository
	stats    ports.TeamStatsRepository
	teams    ports.TeamRepository
}

func NewTaskService(tasks ports.TaskRepository, history ports.TaskHistoryRepository, comments ports.TaskCommentRepository, stats ports.TeamStatsRepository, teams ports.TeamRepository) *TaskService {
	return &TaskService{tasks: tasks, history: history, comments: comments, stats: stats, teams: teams}
}

func (s *TaskService) Create(ctx context.Context, actorID, teamID int64, title, description, status string, assigneeID *int64) (*domain.Task, error) {
	if strings.TrimSpace(title) == "" || len(strings.TrimSpace(title)) > 255 || len(description) > 65535 || !validStatus(status) {
		return nil, domain.ErrInvalid
	}
	if _, err := s.teams.RoleForUser(ctx, teamID, actorID); err != nil {
		return nil, err
	}
	task := &domain.Task{
		TeamID: teamID, Title: strings.TrimSpace(title), Description: description,
		Status: status, CreatedBy: actorID, AssigneeID: assigneeID,
	}
	if err := s.tasks.Create(ctx, task, actorID); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *TaskService) List(ctx context.Context, actorID int64, filter domain.TaskFilter) ([]domain.Task, error) {
	if filter.TeamID <= 0 {
		return nil, domain.ErrInvalid
	}
	if _, err := s.teams.RoleForUser(ctx, filter.TeamID, actorID); err != nil {
		return nil, err
	}
	if filter.Status != "" && !validStatus(filter.Status) {
		return nil, domain.ErrInvalid
	}
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if filter.Limit < 0 || filter.Limit > 100 || filter.Offset < 0 || filter.AssigneeID < 0 {
		return nil, domain.ErrInvalid
	}
	return s.tasks.List(ctx, filter)
}

func (s *TaskService) Update(ctx context.Context, actorID, taskID, version int64, change domain.TaskChange) (*domain.Task, error) {
	if version <= 0 || (change.Title == nil && change.Description == nil && change.Status == nil && change.AssigneeID == nil) {
		return nil, domain.ErrInvalid
	}
	task, err := s.tasks.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	role, err := s.teams.RoleForUser(ctx, task.TeamID, actorID)
	if err != nil {
		return nil, err
	}
	canEditAny := role == domain.RoleOwner || role == domain.RoleAdmin
	isCreator, isAssignee := task.CreatedBy == actorID, task.AssigneeID != nil && *task.AssigneeID == actorID
	if !canEditAny && !isCreator && !isAssignee {
		return nil, domain.ErrForbidden
	}
	if isAssignee && !isCreator && !canEditAny && (change.Title != nil || change.Description != nil || change.AssigneeID != nil) {
		return nil, domain.ErrForbidden
	}
	changes := make(map[string]any)
	addChange := func(key string, oldValue, newValue any) error {
		if oldValue == newValue {
			return nil
		}
		changes[key] = map[string]any{"old": oldValue, "new": newValue}
		return nil
	}
	if change.Title != nil {
		value := strings.TrimSpace(*change.Title)
		if value == "" || len(value) > 255 {
			return nil, domain.ErrInvalid
		}
		addChange("title", task.Title, value)
		task.Title = value
	}
	if change.Description != nil {
		if len(*change.Description) > 65535 {
			return nil, domain.ErrInvalid
		}
		addChange("description", task.Description, *change.Description)
		task.Description = *change.Description
	}
	if change.Status != nil {
		if !validStatus(*change.Status) {
			return nil, domain.ErrInvalid
		}
		addChange("status", task.Status, *change.Status)
		task.Status = *change.Status
	}
	if change.AssigneeID != nil {
		if *change.AssigneeID < 0 {
			return nil, domain.ErrInvalid
		}
		var oldAssignee any
		if task.AssigneeID != nil {
			oldAssignee = *task.AssigneeID
		}
		var newAssignee any
		if *change.AssigneeID > 0 {
			newAssignee = *change.AssigneeID
			task.AssigneeID = change.AssigneeID
		} else {
			task.AssigneeID = nil
		}
		addChange("assignee_id", oldAssignee, newAssignee)
	}
	if len(changes) == 0 {
		if err := s.tasks.CheckVersion(ctx, taskID, version); err != nil {
			return nil, err
		}
		return task, nil
	}
	encodedChanges, err := json.Marshal(changes)
	if err != nil {
		return nil, fmt.Errorf("encode task changes: %w", err)
	}
	if err := s.tasks.UpdateAtomic(ctx, task, actorID, version, encodedChanges); err != nil {
		return nil, err
	}
	task.Version = version + 1
	return task, nil
}

func (s *TaskService) History(ctx context.Context, actorID, taskID int64) ([]domain.TaskHistory, error) {
	task, err := s.tasks.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if _, err := s.teams.RoleForUser(ctx, task.TeamID, actorID); err != nil {
		return nil, err
	}
	return s.history.ListHistory(ctx, taskID)
}

func (s *TaskService) AddComment(ctx context.Context, actorID, taskID int64, content string) (*domain.TaskComment, error) {
	task, err := s.tasks.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if _, err := s.teams.RoleForUser(ctx, task.TeamID, actorID); err != nil {
		return nil, err
	}
	content = strings.TrimSpace(content)
	if content == "" || len(content) > 10000 {
		return nil, domain.ErrInvalid
	}
	comment := &domain.TaskComment{TaskID: taskID, UserID: actorID, Content: content}
	if err := s.comments.AddComment(ctx, comment); err != nil {
		return nil, err
	}
	return comment, nil
}

func (s *TaskService) Comments(ctx context.Context, actorID, taskID int64) ([]domain.TaskComment, error) {
	task, err := s.tasks.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if _, err := s.teams.RoleForUser(ctx, task.TeamID, actorID); err != nil {
		return nil, err
	}
	return s.comments.ListComments(ctx, taskID)
}

func (s *TaskService) Stats(ctx context.Context, actorID, teamID int64) (*domain.TeamStats, error) {
	role, err := s.teams.RoleForUser(ctx, teamID, actorID)
	if err != nil {
		return nil, err
	}
	if role != domain.RoleOwner && role != domain.RoleAdmin {
		return nil, domain.ErrForbidden
	}
	return s.stats.TeamStats(ctx, teamID)
}

func validStatus(status string) bool {
	return status == "todo" || status == "in_progress" || status == "done"
}
