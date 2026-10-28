package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"task-api/internal/domain/entities"
	"task-api/internal/domain/errrors"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) CreateAtomic(ctx context.Context, team *entities.Team) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO teams (name, created_by) VALUES (?, ?)`, team.Name, team.CreatedBy)
	if err != nil {
		return mapDBError(err)
	}
	team.ID, err = result.LastInsertId()
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO team_members (team_id, user_id, role) VALUES (?, ?, 'owner')`, team.ID, team.CreatedBy); err != nil {
		return mapDBError(err)
	}
	return tx.Commit()
}

func (r *Repository) ListForUser(ctx context.Context, userID int64) ([]entities.Team, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT t.id, t.name, u.name, t.created_at
		FROM teams t
		JOIN team_members m ON m.team_id = t.id
		JOIN users u ON u.id = t.created_by
		WHERE m.user_id = ? ORDER BY t.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	teams := make([]entities.Team, 0)
	for rows.Next() {
		var team entities.Team
		if err := rows.Scan(&team.ID, &team.Name, &team.OwnerName, &team.CreatedAt); err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}
	return teams, rows.Err()
}

func (r *Repository) RoleForUser(ctx context.Context, teamID, userID int64) (entities.Role, error) {
	var role entities.Role
	err := r.db.QueryRowContext(ctx, `SELECT role FROM team_members WHERE team_id = ? AND user_id = ?`, teamID, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errrors.ErrForbidden
	}
	return role, err
}

func (r *Repository) AddMemberAtomic(ctx context.Context, teamID, userID int64, role entities.Role) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO team_members (team_id, user_id, role) VALUES (?, ?, ?)`, teamID, userID, role); err != nil {
		return mapDBError(err)
	}
	return tx.Commit()
}

func (r *Repository) UpdateMemberRole(ctx context.Context, teamID, userID int64, role entities.Role) error {
	result, err := r.db.ExecContext(ctx, `UPDATE team_members SET role = ? WHERE team_id = ? AND user_id = ? AND role <> 'owner'`, role, teamID, userID)
	if err != nil {
		return mapDBError(err)
	}
	return requireAffected(result)
}

func (r *Repository) RemoveMember(ctx context.Context, teamID, userID int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM team_members WHERE team_id = ? AND user_id = ? AND role <> 'owner'`, teamID, userID)
	if err != nil {
		return err
	}
	return requireAffected(result)
}

func (r *Repository) Create(ctx context.Context, task *entities.Task, changedBy int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := ensureTeamMember(ctx, tx, task.TeamID, changedBy); err != nil {
		return err
	}
	if task.AssigneeID != nil {
		if err := ensureTeamMember(ctx, tx, task.TeamID, *task.AssigneeID); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO tasks (team_id, title, description, status, created_by, assignee_id, closed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, task.TeamID, task.Title, task.Description, task.Status, task.CreatedBy, task.AssigneeID, closedAtForStatus(task.Status, nil))
	if err != nil {
		return mapDBError(err)
	}
	task.ID, err = result.LastInsertId()
	if err != nil {
		return err
	}
	var closed sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT t.created_at, t.updated_at, t.closed_at, u.name
		FROM tasks t JOIN users u ON u.id = t.created_by WHERE t.id = ?`, task.ID).
		Scan(&task.CreatedAt, &task.UpdatedAt, &closed, &task.CreatedByName); err != nil {
		return err
	}
	if closed.Valid {
		task.ClosedAt = &closed.Time
	}
	task.Version = 1
	var snapshot json.RawMessage
	snapshot, err = json.Marshal(map[string]any{"new": task})
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO task_history (task_id, changed_by, changes) VALUES (?, ?, ?)`, task.ID, changedBy, snapshot); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (r *Repository) List(ctx context.Context, filter entities.TaskFilter) ([]entities.Task, error) {
	query := `SELECT t.id, t.team_id, t.title, t.description, t.status, t.created_by, u.name, t.assignee_id, t.created_at, t.updated_at, t.closed_at, t.version
		FROM tasks t JOIN users u ON u.id = t.created_by WHERE t.team_id = ?`
	args := []any{filter.TeamID}
	if filter.Status != "" {
		query += ` AND t.status = ?`
		args = append(args, filter.Status)
	}
	if filter.AssigneeID > 0 {
		query += ` AND t.assignee_id = ?`
		args = append(args, filter.AssigneeID)
	}
	query += ` ORDER BY t.id DESC LIMIT ? OFFSET ?`
	args = append(args, filter.Limit, filter.Offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := make([]entities.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

type rowScanner interface{ Scan(...any) error }

func scanTask(row rowScanner) (entities.Task, error) {
	var task entities.Task
	var assignee sql.NullInt64
	var closed sql.NullTime
	if err := row.Scan(&task.ID, &task.TeamID, &task.Title, &task.Description, &task.Status, &task.CreatedBy, &task.CreatedByName, &assignee, &task.CreatedAt, &task.UpdatedAt, &closed, &task.Version); err != nil {
		return task, err
	}
	if assignee.Valid {
		task.AssigneeID = &assignee.Int64
	}
	if closed.Valid {
		task.ClosedAt = &closed.Time
	}
	return task, nil
}

func (r *Repository) FindByID(ctx context.Context, id int64) (*entities.Task, error) {
	task, err := scanTask(r.db.QueryRowContext(ctx, `SELECT t.id, t.team_id, t.title, t.description, t.status, t.created_by, u.name, t.assignee_id, t.created_at, t.updated_at, t.closed_at, t.version
		FROM tasks t JOIN users u ON u.id = t.created_by WHERE t.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *Repository) CheckVersion(ctx context.Context, taskID, expectedVersion int64) error {
	var version int64
	err := r.db.QueryRowContext(ctx, `SELECT version FROM tasks WHERE id = ?`, taskID).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return errrors.ErrNotFound
	}
	if err != nil {
		return err
	}
	if version != expectedVersion {
		return errrors.ErrConflict
	}
	return nil
}

func (r *Repository) UpdateAtomic(ctx context.Context, task *entities.Task, changedBy, expectedVersion int64, changes json.RawMessage) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var currentVersion, currentCreator, currentTeam int64
	var currentAssignee sql.NullInt64
	var currentStatus string
	if err := tx.QueryRowContext(ctx, `SELECT team_id, created_by, assignee_id, version, status FROM tasks WHERE id = ? FOR UPDATE`, task.ID).
		Scan(&currentTeam, &currentCreator, &currentAssignee, &currentVersion, &currentStatus); errors.Is(err, sql.ErrNoRows) {
		return errrors.ErrNotFound
	} else if err != nil {
		return err
	}
	var actorRole entities.Role
	if err := tx.QueryRowContext(ctx, `SELECT role FROM team_members WHERE team_id = ? AND user_id = ? FOR UPDATE`, currentTeam, changedBy).
		Scan(&actorRole); errors.Is(err, sql.ErrNoRows) {
		return errrors.ErrForbidden
	} else if err != nil {
		return err
	}
	canEditAny := actorRole == entities.RoleOwner || actorRole == entities.RoleAdmin
	isCreator := currentCreator == changedBy
	isAssignee := currentAssignee.Valid && currentAssignee.Int64 == changedBy
	if !canEditAny && !isCreator && !isAssignee {
		return errrors.ErrForbidden
	}
	if !canEditAny && !isCreator && isAssignee {
		var changedFields map[string]json.RawMessage
		if err := json.Unmarshal(changes, &changedFields); err != nil {
			return fmt.Errorf("decode task changes: %w", err)
		}
		for field := range changedFields {
			if field != "status" {
				return errrors.ErrForbidden
			}
		}
	}
	if currentVersion != expectedVersion {
		return errrors.ErrConflict
	}
	if task.AssigneeID != nil {
		if err := ensureTeamMember(ctx, tx, task.TeamID, *task.AssigneeID); err != nil {
			return err
		}
	}
	closedAt := task.ClosedAt
	if task.Status != currentStatus {
		closedAt = closedAtForStatus(task.Status, nil)
	}
	_, err = tx.ExecContext(ctx, `UPDATE tasks SET title = ?, description = ?, status = ?, assignee_id = ?, closed_at = ?, version = version + 1 WHERE id = ? AND version = ?`,
		task.Title, task.Description, task.Status, task.AssigneeID, closedAt, task.ID, expectedVersion)
	if err != nil {
		return mapDBError(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO task_history (task_id, changed_by, changes) VALUES (?, ?, ?)`, task.ID, changedBy, changes); err != nil {
		return err
	}
	var updatedAt time.Time
	var closed sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT updated_at, closed_at FROM tasks WHERE id = ?`, task.ID).
		Scan(&updatedAt, &closed); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	task.ClosedAt = closedAt
	task.Version = expectedVersion + 1
	task.UpdatedAt = updatedAt
	return nil
}

func closedAtForStatus(status string, existing *time.Time) *time.Time {
	if status != "done" {
		return nil
	}
	if existing != nil {
		return existing
	}
	now := time.Now().UTC()
	return &now
}

type executor interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func ensureTeamMember(ctx context.Context, queryer executor, teamID, userID int64) error {
	var exists int
	err := queryer.QueryRowContext(ctx, `SELECT 1 FROM team_members WHERE team_id = ? AND user_id = ? FOR UPDATE`, teamID, userID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return errrors.ErrInvalid
	}
	return err
}

func (r *Repository) ListHistory(ctx context.Context, taskID int64) ([]entities.TaskHistory, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT h.id, h.task_id, h.changed_by, h.changes, h.created_at, u.name
		FROM task_history h
		JOIN tasks t ON t.id = h.task_id
		JOIN users u ON u.id = t.created_by
		WHERE h.task_id = ? ORDER BY h.id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	history := make([]entities.TaskHistory, 0)
	for rows.Next() {
		var item entities.TaskHistory
		var creatorName string
		if err := rows.Scan(&item.ID, &item.TaskID, &item.ChangedBy, &item.Changes, &item.CreatedAt, &creatorName); err != nil {
			return nil, err
		}
		item.Changes, err = replaceHistoryCreatorID(item.Changes, creatorName)
		if err != nil {
			return nil, err
		}
		history = append(history, item)
	}
	return history, rows.Err()
}

func replaceHistoryCreatorID(changes json.RawMessage, creatorName string) (json.RawMessage, error) {
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(changes, &entries); err != nil {
		return nil, fmt.Errorf("decode task history changes: %w", err)
	}
	rawTask, ok := entries["new"]
	if !ok {
		return changes, nil
	}
	var task map[string]json.RawMessage
	if err := json.Unmarshal(rawTask, &task); err != nil {
		return nil, fmt.Errorf("decode task history snapshot: %w", err)
	}
	if _, ok := task["created_by"]; !ok {
		return changes, nil
	}
	rawCreatorName, err := json.Marshal(creatorName)
	if err != nil {
		return nil, fmt.Errorf("encode task creator name: %w", err)
	}
	task["created_by"] = rawCreatorName
	entries["new"], err = json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("encode task history snapshot: %w", err)
	}
	updated, err := json.Marshal(entries)
	if err != nil {
		return nil, fmt.Errorf("encode task history changes: %w", err)
	}
	return updated, nil
}

func (r *Repository) AddComment(ctx context.Context, comment *entities.TaskComment) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var teamID int64
	if err := tx.QueryRowContext(ctx, `SELECT team_id FROM tasks WHERE id = ? FOR UPDATE`, comment.TaskID).Scan(&teamID); errors.Is(err, sql.ErrNoRows) {
		return errrors.ErrNotFound
	} else if err != nil {
		return err
	}
	if err := ensureTeamMember(ctx, tx, teamID, comment.UserID); err != nil {
		if errors.Is(err, errrors.ErrInvalid) {
			return errrors.ErrForbidden
		}
		return err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO task_comments (task_id, user_id, content) VALUES (?, ?, ?)`, comment.TaskID, comment.UserID, comment.Content)
	if err != nil {
		return mapDBError(err)
	}
	comment.ID, err = result.LastInsertId()
	if err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `SELECT created_at FROM task_comments WHERE id = ?`, comment.ID).Scan(&comment.CreatedAt); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) ListComments(ctx context.Context, taskID int64) ([]entities.TaskComment, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, task_id, user_id, content, created_at FROM task_comments WHERE task_id = ? ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	comments := make([]entities.TaskComment, 0)
	for rows.Next() {
		var comment entities.TaskComment
		if err := rows.Scan(&comment.ID, &comment.TaskID, &comment.UserID, &comment.Content, &comment.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	return comments, rows.Err()
}

func (r *Repository) TeamStats(ctx context.Context, teamID int64) (*entities.TeamStats, error) {
	const query = `WITH status_counts AS (
		SELECT status, COUNT(*) AS task_count FROM tasks WHERE team_id = ? GROUP BY status
	), top_assignees AS (
		SELECT t.assignee_id AS user_id, u.name, COUNT(*) AS closed_tasks
		FROM tasks t JOIN users u ON u.id = t.assignee_id
		WHERE t.team_id = ? AND t.status = 'done' AND t.closed_at >= UTC_TIMESTAMP() - INTERVAL 30 DAY
		GROUP BY t.assignee_id, u.name ORDER BY closed_tasks DESC, t.assignee_id LIMIT 3
	), closure AS (
		SELECT COALESCE(AVG(TIMESTAMPDIFF(SECOND, created_at, closed_at)), 0) AS average_seconds
		FROM tasks WHERE team_id = ? AND status = 'done' AND closed_at IS NOT NULL
	), comments AS (
		SELECT t.id AS task_id, COUNT(c.id) AS comment_count
		FROM tasks t LEFT JOIN task_comments c ON c.task_id = t.id
		WHERE t.team_id = ? GROUP BY t.id
	)
	SELECT
		COALESCE((SELECT JSON_OBJECTAGG(status, task_count) FROM status_counts), JSON_OBJECT()),
		COALESCE((SELECT JSON_ARRAYAGG(JSON_OBJECT('user_id', user_id, 'name', name, 'closed_tasks', closed_tasks)) FROM top_assignees), JSON_ARRAY()),
		(SELECT average_seconds FROM closure),
		COALESCE((SELECT JSON_ARRAYAGG(JSON_OBJECT('task_id', task_id, 'comment_count', comment_count)) FROM comments), JSON_ARRAY()),
		COALESCE((SELECT SUM(comment_count) FROM comments), 0)`
	var statuses, top, comments []byte
	stats := new(entities.TeamStats)
	if err := r.db.QueryRowContext(ctx, query, teamID, teamID, teamID, teamID).Scan(&statuses, &top, &stats.AverageClosureSeconds, &comments, &stats.TotalComments); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(statuses, &stats.TasksByStatus); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(top, &stats.TopAssignees); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(comments, &stats.CommentsByTask); err != nil {
		return nil, err
	}
	return stats, nil
}

func requireAffected(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return errrors.ErrNotFound
	}
	return nil
}

func mapDBError(err error) error {
	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return errrors.ErrConflict
	}
	if errors.As(err, &mysqlErr) && (mysqlErr.Number == 1452 || mysqlErr.Number == 1451) {
		return errrors.ErrInvalid
	}
	return fmt.Errorf("mysql: %w", err)
}
