package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"task-api/internal/domain"
)

type teamRepositoryStub struct {
	roles map[int64]domain.Role
}

func (s teamRepositoryStub) CreateAtomic(context.Context, *domain.Team) error { return nil }
func (s teamRepositoryStub) ListForUser(context.Context, int64) ([]domain.Team, error) {
	return nil, nil
}
func (s teamRepositoryStub) RoleForUser(_ context.Context, _, userID int64) (domain.Role, error) {
	role, ok := s.roles[userID]
	if !ok {
		return "", domain.ErrForbidden
	}
	return role, nil
}
func (s teamRepositoryStub) AddMemberAtomic(context.Context, int64, int64, domain.Role) error {
	return nil
}
func (s teamRepositoryStub) UpdateMemberRole(context.Context, int64, int64, domain.Role) error {
	return nil
}
func (s teamRepositoryStub) RemoveMember(context.Context, int64, int64) error { return nil }

type taskRepositoryStub struct {
	task            domain.Task
	updateCalls     int
	expectedVersion int64
	changes         json.RawMessage
}

func (s *taskRepositoryStub) Create(context.Context, *domain.Task, int64) error { return nil }
func (s *taskRepositoryStub) List(context.Context, domain.TaskFilter) ([]domain.Task, error) {
	return []domain.Task{s.task}, nil
}
func (s *taskRepositoryStub) FindByID(context.Context, int64) (*domain.Task, error) {
	task := s.task
	return &task, nil
}
func (s *taskRepositoryStub) UpdateAtomic(_ context.Context, _ *domain.Task, _, version int64, changes json.RawMessage) error {
	s.updateCalls++
	s.expectedVersion = version
	s.changes = changes
	return nil
}
func (s *taskRepositoryStub) ListHistory(context.Context, int64) ([]domain.TaskHistory, error) {
	return nil, nil
}
func (s *taskRepositoryStub) AddComment(context.Context, *domain.TaskComment) error { return nil }
func (s *taskRepositoryStub) ListComments(context.Context, int64) ([]domain.TaskComment, error) {
	return nil, nil
}
func (s *taskRepositoryStub) TeamStats(context.Context, int64) (*domain.TeamStats, error) {
	return nil, nil
}

func TestTaskAssigneeCanOnlyChangeStatus(t *testing.T) {
	assigneeID := int64(7)
	repository := &taskRepositoryStub{task: domain.Task{
		ID: 3, TeamID: 2, CreatedBy: 5, AssigneeID: &assigneeID, Status: "todo", Version: 4,
	}}
	service := NewTaskService(repository, teamRepositoryStub{
		roles: map[int64]domain.Role{7: domain.RoleMember},
	})
	title := "changed by assignee"
	if _, err := service.Update(context.Background(), 7, 3, 4, domain.TaskChange{Title: &title}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected forbidden title change, got %v", err)
	}
	if repository.updateCalls != 0 {
		t.Fatal("forbidden update reached repository")
	}

	status := "in_progress"
	if _, err := service.Update(context.Background(), 7, 3, 4, domain.TaskChange{Status: &status}); err != nil {
		t.Fatalf("assignee status update failed: %v", err)
	}
	if repository.updateCalls != 1 || repository.expectedVersion != 4 {
		t.Fatalf("unexpected repository update: calls=%d version=%d", repository.updateCalls, repository.expectedVersion)
	}
	var changes map[string]map[string]any
	if err := json.Unmarshal(repository.changes, &changes); err != nil {
		t.Fatal(err)
	}
	if changes["status"]["old"] != "todo" || changes["status"]["new"] != "in_progress" {
		t.Fatalf("unexpected history diff: %s", repository.changes)
	}
}

func TestTaskUpdateRequiresTeamMembershipAndVersion(t *testing.T) {
	repository := &taskRepositoryStub{task: domain.Task{ID: 3, TeamID: 2, CreatedBy: 5, Status: "todo", Version: 1}}
	service := NewTaskService(repository, teamRepositoryStub{roles: map[int64]domain.Role{}})
	title := "updated"
	if _, err := service.Update(context.Background(), 9, 3, 1, domain.TaskChange{Title: &title}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected membership denial, got %v", err)
	}
	if repository.updateCalls != 0 {
		t.Fatal("non-member update reached repository")
	}

	service = NewTaskService(repository, teamRepositoryStub{roles: map[int64]domain.Role{5: domain.RoleMember}})
	if _, err := service.Update(context.Background(), 5, 3, 0, domain.TaskChange{Title: &title}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("expected invalid stale version, got %v", err)
	}
}

func TestTaskListValidatesPaginationAndStatus(t *testing.T) {
	repository := &taskRepositoryStub{}
	service := NewTaskService(repository, teamRepositoryStub{roles: map[int64]domain.Role{5: domain.RoleMember}})
	if _, err := service.List(context.Background(), 5, domain.TaskFilter{TeamID: 2, Status: "unknown", Limit: 20}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("expected invalid status error, got %v", err)
	}
	if _, err := service.List(context.Background(), 5, domain.TaskFilter{TeamID: 2, Limit: 101}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("expected invalid limit error, got %v", err)
	}
}
