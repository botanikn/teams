package httpapi

import (
	"encoding/json"
	"time"

	"task-api/internal/domain"
)

type userResponse struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type teamResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	OwnerName string    `json:"owner_name,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type taskResponse struct {
	ID          int64      `json:"id"`
	TeamID      int64      `json:"team_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	CreatedBy   string     `json:"created_by"`
	AssigneeID  *int64     `json:"assignee_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ClosedAt    *time.Time `json:"closed_at"`
	Version     int64      `json:"version"`
}

type taskHistoryResponse struct {
	ID        int64           `json:"id"`
	TaskID    int64           `json:"task_id"`
	ChangedBy int64           `json:"changed_by"`
	Changes   json.RawMessage `json:"changes"`
	CreatedAt time.Time       `json:"created_at"`
}

type taskCommentResponse struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	UserID    int64     `json:"user_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type teamStatsResponse struct {
	TasksByStatus         map[string]int64          `json:"tasks_by_status"`
	TopAssignees          []topAssigneeResponse     `json:"top_assignees"`
	AverageClosureSeconds float64                   `json:"average_closure_seconds"`
	CommentsByTask        []taskCommentStatResponse `json:"comments_by_task"`
	TotalComments         int64                     `json:"total_comments"`
}

type topAssigneeResponse struct {
	UserID      int64  `json:"user_id"`
	Name        string `json:"name"`
	ClosedTasks int64  `json:"closed_tasks"`
}

type taskCommentStatResponse struct {
	TaskID       int64 `json:"task_id"`
	CommentCount int64 `json:"comment_count"`
}

func toUserResponse(user *domain.User) userResponse {
	return userResponse{ID: user.ID, Email: user.Email, Name: user.Name, CreatedAt: user.CreatedAt}
}

func toTeamResponse(team domain.Team) teamResponse {
	return teamResponse{ID: team.ID, Name: team.Name, OwnerName: team.OwnerName, CreatedAt: team.CreatedAt}
}

func toTeamResponses(teams []domain.Team) []teamResponse {
	responses := make([]teamResponse, len(teams))
	for i := range teams {
		responses[i] = toTeamResponse(teams[i])
	}
	return responses
}

func toTaskResponse(task *domain.Task) taskResponse {
	return taskResponse{
		ID: task.ID, TeamID: task.TeamID, Title: task.Title, Description: task.Description,
		Status: task.Status, CreatedBy: task.CreatedByName, AssigneeID: task.AssigneeID,
		CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt, ClosedAt: task.ClosedAt, Version: task.Version,
	}
}

func toTaskResponses(tasks []domain.Task) []taskResponse {
	responses := make([]taskResponse, len(tasks))
	for i := range tasks {
		responses[i] = toTaskResponse(&tasks[i])
	}
	return responses
}

func toTaskHistoryResponses(items []domain.TaskHistory) []taskHistoryResponse {
	responses := make([]taskHistoryResponse, len(items))
	for i := range items {
		responses[i] = taskHistoryResponse{
			ID: items[i].ID, TaskID: items[i].TaskID, ChangedBy: items[i].ChangedBy,
			Changes: items[i].Changes, CreatedAt: items[i].CreatedAt,
		}
	}
	return responses
}

func toTaskCommentResponse(comment *domain.TaskComment) taskCommentResponse {
	return taskCommentResponse{
		ID: comment.ID, TaskID: comment.TaskID, UserID: comment.UserID,
		Content: comment.Content, CreatedAt: comment.CreatedAt,
	}
}

func toTaskCommentResponses(comments []domain.TaskComment) []taskCommentResponse {
	responses := make([]taskCommentResponse, len(comments))
	for i := range comments {
		responses[i] = toTaskCommentResponse(&comments[i])
	}
	return responses
}

func toTeamStatsResponse(stats *domain.TeamStats) teamStatsResponse {
	response := teamStatsResponse{
		TasksByStatus:         stats.TasksByStatus,
		AverageClosureSeconds: stats.AverageClosureSeconds,
		TotalComments:         stats.TotalComments,
	}
	response.TopAssignees = make([]topAssigneeResponse, len(stats.TopAssignees))
	for i := range stats.TopAssignees {
		item := stats.TopAssignees[i]
		response.TopAssignees[i] = topAssigneeResponse{
			UserID: item.UserID, Name: item.Name, ClosedTasks: item.ClosedTasks,
		}
	}
	response.CommentsByTask = make([]taskCommentStatResponse, len(stats.CommentsByTask))
	for i := range stats.CommentsByTask {
		item := stats.CommentsByTask[i]
		response.CommentsByTask[i] = taskCommentStatResponse{
			TaskID: item.TaskID, CommentCount: item.CommentCount,
		}
	}
	return response
}
