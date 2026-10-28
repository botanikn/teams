package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestTeamStatsIntegration(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_TEST_DSN to run the MySQL integration test")
	}

	baseConfig, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse MYSQL_TEST_DSN: %v", err)
	}
	adminConfig := *baseConfig
	adminConfig.DBName = ""
	adminDB, err := sql.Open("mysql", adminConfig.FormatDSN())
	if err != nil {
		t.Fatalf("open MySQL admin connection: %v", err)
	}
	defer adminDB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := adminDB.PingContext(ctx); err != nil {
		t.Fatalf("connect to MySQL: %v", err)
	}

	databaseName := fmt.Sprintf("task_api_test_%d", time.Now().UnixNano())
	if _, err := adminDB.ExecContext(ctx, "CREATE DATABASE `"+databaseName+"` CHARACTER SET utf8mb4"); err != nil {
		t.Fatalf("create isolated integration database: %v", err)
	}

	testConfig := *baseConfig
	testConfig.DBName = databaseName
	testDB, err := sql.Open("mysql", testConfig.FormatDSN())
	if err != nil {
		_, _ = adminDB.ExecContext(ctx, "DROP DATABASE `"+databaseName+"`")
		t.Fatalf("open isolated integration database: %v", err)
	}
	defer func() {
		_ = testDB.Close()
		if _, err := adminDB.ExecContext(context.Background(), "DROP DATABASE `"+databaseName+"`"); err != nil {
			t.Errorf("drop isolated integration database: %v", err)
		}
	}()
	if err := testDB.PingContext(ctx); err != nil {
		t.Fatalf("connect to isolated integration database: %v", err)
	}
	if err := Migrate(ctx, testDB); err != nil {
		t.Fatalf("migrate isolated integration database: %v", err)
	}

	creatorID := insertStatsUser(t, ctx, testDB, "creator@example.com", "Creator")
	assigneeAID := insertStatsUser(t, ctx, testDB, "a@example.com", "Assignee A")
	assigneeBID := insertStatsUser(t, ctx, testDB, "b@example.com", "Assignee B")
	assigneeCID := insertStatsUser(t, ctx, testDB, "c@example.com", "Assignee C")
	assigneeDID := insertStatsUser(t, ctx, testDB, "d@example.com", "Assignee D")
	teamID := insertStatsTeam(t, ctx, testDB, "Stats team", creatorID)
	otherTeamID := insertStatsTeam(t, ctx, testDB, "Other team", creatorID)

	now := time.Now().UTC().Truncate(time.Second)
	taskA := insertStatsTask(t, ctx, testDB, teamID, creatorID, &assigneeAID, "done", now.Add(-72*time.Hour), timePtr(now.Add(-24*time.Hour)))
	insertStatsTask(t, ctx, testDB, teamID, creatorID, &assigneeAID, "done", now.Add(-48*time.Hour), timePtr(now.Add(-12*time.Hour)))
	insertStatsTask(t, ctx, testDB, teamID, creatorID, nil, "todo", now, nil)
	taskD := insertStatsTask(t, ctx, testDB, teamID, creatorID, &assigneeBID, "in_progress", now, nil)
	insertStatsTask(t, ctx, testDB, teamID, creatorID, &assigneeBID, "done", now.Add(-72*time.Hour), timePtr(now.Add(-24*time.Hour)))
	insertStatsTask(t, ctx, testDB, teamID, creatorID, &assigneeCID, "done", now.Add(-72*time.Hour), timePtr(now.Add(-24*time.Hour)))
	insertStatsTask(t, ctx, testDB, teamID, creatorID, &assigneeDID, "done", now.Add(-72*time.Hour), timePtr(now.Add(-24*time.Hour)))
	otherTask := insertStatsTask(t, ctx, testDB, otherTeamID, creatorID, &assigneeBID, "done", now.Add(-48*time.Hour), timePtr(now.Add(-24*time.Hour)))
	insertStatsComment(t, ctx, testDB, taskA, creatorID, "first")
	insertStatsComment(t, ctx, testDB, taskA, assigneeAID, "second")
	insertStatsComment(t, ctx, testDB, taskD, creatorID, "third")
	insertStatsComment(t, ctx, testDB, otherTask, creatorID, "other team")

	stats, err := NewRepository(testDB).TeamStats(ctx, teamID)
	if err != nil {
		t.Fatalf("read team stats: %v", err)
	}
	if stats.TasksByStatus["done"] != 5 || stats.TasksByStatus["todo"] != 1 || stats.TasksByStatus["in_progress"] != 1 {
		t.Fatalf("unexpected status counts: %#v", stats.TasksByStatus)
	}
	if len(stats.TopAssignees) != 3 || stats.TopAssignees[0].UserID != assigneeAID ||
		stats.TopAssignees[0].ClosedTasks != 2 || stats.TopAssignees[1].UserID != assigneeBID ||
		stats.TopAssignees[1].ClosedTasks != 1 || stats.TopAssignees[2].UserID != assigneeCID ||
		stats.TopAssignees[2].ClosedTasks != 1 {
		t.Fatalf("unexpected top assignees: %#v", stats.TopAssignees)
	}
	if stats.AverageClosureSeconds != 164160 {
		t.Fatalf("average closure seconds = %v, want 164160", stats.AverageClosureSeconds)
	}
	if stats.TotalComments != 3 {
		t.Fatalf("total comments = %d, want 3", stats.TotalComments)
	}
	commentCounts := make(map[int64]int64, len(stats.CommentsByTask))
	for _, item := range stats.CommentsByTask {
		commentCounts[item.TaskID] = item.CommentCount
	}
	if len(commentCounts) != 7 || commentCounts[taskA] != 2 || commentCounts[taskD] != 1 {
		t.Fatalf("unexpected per-task comment counts: %#v", commentCounts)
	}
}

func insertStatsUser(t *testing.T, ctx context.Context, db *sql.DB, email, name string) int64 {
	t.Helper()
	result, err := db.ExecContext(ctx, `INSERT INTO users (email, password_hash, name) VALUES (?, ?, ?)`, email, "test-hash", name)
	if err != nil {
		t.Fatalf("insert integration user: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read integration user id: %v", err)
	}
	return id
}

func insertStatsTeam(t *testing.T, ctx context.Context, db *sql.DB, name string, ownerID int64) int64 {
	t.Helper()
	result, err := db.ExecContext(ctx, `INSERT INTO teams (name, created_by) VALUES (?, ?)`, name, ownerID)
	if err != nil {
		t.Fatalf("insert integration team: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read integration team id: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO team_members (team_id, user_id, role) VALUES (?, ?, 'owner')`, id, ownerID); err != nil {
		t.Fatalf("insert integration team owner: %v", err)
	}
	return id
}

func insertStatsTask(t *testing.T, ctx context.Context, db *sql.DB, teamID, creatorID int64, assigneeID *int64, status string, createdAt time.Time, closedAt *time.Time) int64 {
	t.Helper()
	result, err := db.ExecContext(ctx, `INSERT INTO tasks (team_id, title, description, status, created_by, assignee_id, created_at, closed_at)
		VALUES (?, ?, '', ?, ?, ?, ?, ?)`, teamID, "task", status, creatorID, assigneeID, createdAt, closedAt)
	if err != nil {
		t.Fatalf("insert integration task: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read integration task id: %v", err)
	}
	return id
}

func insertStatsComment(t *testing.T, ctx context.Context, db *sql.DB, taskID, userID int64, content string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `INSERT INTO task_comments (task_id, user_id, content) VALUES (?, ?, ?)`, taskID, userID, content); err != nil {
		t.Fatalf("insert integration comment: %v", err)
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
