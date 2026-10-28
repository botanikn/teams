package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"task-api/internal/domain/entities"
	"task-api/internal/domain/errrors"
)

type teamRepositoryStub struct {
	roles map[int64]entities.Role
}

func (s teamRepositoryStub) CreateAtomic(context.Context, *entities.Team) error { return nil }
func (s teamRepositoryStub) ListForUser(context.Context, int64) ([]entities.Team, error) {
	return nil, nil
}
func (s teamRepositoryStub) RoleForUser(_ context.Context, _, userID int64) (entities.Role, error) {
	role, ok := s.roles[userID]
	if !ok {
		return "", errrors.ErrForbidden
	}
	return role, nil
}
func (s teamRepositoryStub) AddMemberAtomic(context.Context, int64, int64, entities.Role) error {
	return nil
}
func (s teamRepositoryStub) UpdateMemberRole(context.Context, int64, int64, entities.Role) error {
	return nil
}
func (s teamRepositoryStub) RemoveMember(context.Context, int64, int64) error { return nil }

type taskRepositoryStub struct {
	task            entities.Task
	updateCalls     int
	expectedVersion int64
	changes         json.RawMessage
	checkVersionErr error
	versionChecks   int
}

func (s *taskRepositoryStub) Create(context.Context, *entities.Task, int64) error { return nil }
func (s *taskRepositoryStub) List(context.Context, entities.TaskFilter) ([]entities.Task, error) {
	return []entities.Task{s.task}, nil
}
func (s *taskRepositoryStub) FindByID(context.Context, int64) (*entities.Task, error) {
	task := s.task
	return &task, nil
}
func (s *taskRepositoryStub) UpdateAtomic(_ context.Context, _ *entities.Task, _, version int64, changes json.RawMessage) error {
	s.updateCalls++
	s.expectedVersion = version
	s.changes = changes
	return nil
}
func (s *taskRepositoryStub) CheckVersion(context.Context, int64, int64) error {
	s.versionChecks++
	return s.checkVersionErr
}
func (s *taskRepositoryStub) ListHistory(context.Context, int64) ([]entities.TaskHistory, error) {
	return nil, nil
}
func (s *taskRepositoryStub) AddComment(context.Context, *entities.TaskComment) error { return nil }
func (s *taskRepositoryStub) ListComments(context.Context, int64) ([]entities.TaskComment, error) {
	return nil, nil
}
func (s *taskRepositoryStub) TeamStats(context.Context, int64) (*entities.TeamStats, error) {
	return nil, nil
}

func TestTaskAssigneeCanOnlyChangeStatus(t *testing.T) {
	assigneeID := int64(7)
	repository := &taskRepositoryStub{task: entities.Task{
		ID: 3, TeamID: 2, CreatedBy: 5, AssigneeID: &assigneeID, Status: "todo", Version: 4,
	}}
	service := NewTaskService(repository, repository, repository, repository, teamRepositoryStub{
		roles: map[int64]entities.Role{7: entities.RoleMember},
	})
	title := "changed by assignee"
	if _, err := service.Update(context.Background(), 7, 3, 4, entities.TaskChange{Title: &title}); !errors.Is(err, errrors.ErrForbidden) {
		t.Fatalf("expected forbidden title change, got %v", err)
	}
	if repository.updateCalls != 0 {
		t.Fatal("forbidden update reached repository")
	}

	status := "in_progress"
	if _, err := service.Update(context.Background(), 7, 3, 4, entities.TaskChange{Status: &status}); err != nil {
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
	repository := &taskRepositoryStub{task: entities.Task{ID: 3, TeamID: 2, CreatedBy: 5, Status: "todo", Version: 1}}
	service := NewTaskService(repository, repository, repository, repository, teamRepositoryStub{roles: map[int64]entities.Role{}})
	title := "updated"
	if _, err := service.Update(context.Background(), 9, 3, 1, entities.TaskChange{Title: &title}); !errors.Is(err, errrors.ErrForbidden) {
		t.Fatalf("expected membership denial, got %v", err)
	}
	if repository.updateCalls != 0 {
		t.Fatal("non-member update reached repository")
	}

	service = NewTaskService(repository, repository, repository, repository, teamRepositoryStub{roles: map[int64]entities.Role{5: entities.RoleMember}})
	if _, err := service.Update(context.Background(), 5, 3, 0, entities.TaskChange{Title: &title}); !errors.Is(err, errrors.ErrInvalid) {
		t.Fatalf("expected invalid stale version, got %v", err)
	}
}

func TestNoopTaskUpdateStillChecksVersion(t *testing.T) {
	repository := &taskRepositoryStub{
		task:            entities.Task{ID: 3, TeamID: 2, CreatedBy: 5, Title: "same", Status: "todo", Version: 4},
		checkVersionErr: errrors.ErrConflict,
	}
	service := NewTaskService(repository, repository, repository, repository, teamRepositoryStub{
		roles: map[int64]entities.Role{5: entities.RoleMember},
	})
	title := "same"
	if _, err := service.Update(context.Background(), 5, 3, 3, entities.TaskChange{Title: &title}); !errors.Is(err, errrors.ErrConflict) {
		t.Fatalf("expected stale no-op update to conflict, got %v", err)
	}
	if repository.versionChecks != 1 || repository.updateCalls != 0 {
		t.Fatalf("version checks=%d, updates=%d; want one version check and no update", repository.versionChecks, repository.updateCalls)
	}
}

func TestTaskListValidatesPaginationAndStatus(t *testing.T) {
	repository := &taskRepositoryStub{}
	service := NewTaskService(repository, repository, repository, repository, teamRepositoryStub{roles: map[int64]entities.Role{5: entities.RoleMember}})
	if _, err := service.List(context.Background(), 5, entities.TaskFilter{TeamID: 2, Status: "unknown", Limit: 20}); !errors.Is(err, errrors.ErrInvalid) {
		t.Fatalf("expected invalid status error, got %v", err)
	}
	if _, err := service.List(context.Background(), 5, entities.TaskFilter{TeamID: 2, Limit: 101}); !errors.Is(err, errrors.ErrInvalid) {
		t.Fatalf("expected invalid limit error, got %v", err)
	}
}
