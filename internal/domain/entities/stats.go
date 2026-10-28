package entities

type TeamStats struct {
	TasksByStatus         map[string]int64  `json:"tasks_by_status"`
	TopAssignees          []TopAssignee     `json:"top_assignees"`
	AverageClosureSeconds float64           `json:"average_closure_seconds"`
	CommentsByTask        []TaskCommentStat `json:"comments_by_task"`
	TotalComments         int64             `json:"total_comments"`
}

type TopAssignee struct {
	UserID      int64  `json:"user_id"`
	Name        string `json:"name"`
	ClosedTasks int64  `json:"closed_tasks"`
}

type TaskCommentStat struct {
	TaskID       int64 `json:"task_id"`
	CommentCount int64 `json:"comment_count"`
}
