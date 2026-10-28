package httpapi

import (
	"encoding/json"
	"strings"
	"testing"

	"task-api/internal/domain"
)

func TestTaskResponseExposesCreatorNameWithoutInternalID(t *testing.T) {
	raw, err := json.Marshal(toTaskResponse(&domain.Task{
		ID: 3, TeamID: 2, CreatedBy: 42, CreatedByName: "Alice", Version: 1,
	}))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if !strings.Contains(body, `"created_by":"Alice"`) || strings.Contains(body, `"created_by_id"`) || strings.Contains(body, `"CreatedBy"`) {
		t.Fatalf("unexpected task response: %s", body)
	}
}

func TestUserResponseDoesNotExposePasswordHash(t *testing.T) {
	raw, err := json.Marshal(toUserResponse(&domain.User{
		ID: 1, Email: "alice@example.com", Name: "Alice", PasswordHash: "must-not-leak",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "must-not-leak") || strings.Contains(string(raw), "password_hash") {
		t.Fatalf("user response leaked password data: %s", raw)
	}
}
