package domain

import "time"

type Task struct {
	ID            int64      `json:"id"`
	TeamID        int64      `json:"team_id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Status        string     `json:"status"`
	CreatedBy     int64      `json:"-"`
	CreatedByName string     `json:"created_by"`
	AssigneeID    *int64     `json:"assignee_id"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ClosedAt      *time.Time `json:"closed_at"`
	Version       int64      `json:"version"`
}

type TaskFilter struct {
	TeamID     int64  `json:"team_id"`
	Status     string `json:"status,omitempty"`
	AssigneeID int64  `json:"assignee_id,omitempty"`
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
}

type TaskChange struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Status      *string `json:"status,omitempty"`
	AssigneeID  *int64  `json:"assignee_id,omitempty"`
}
