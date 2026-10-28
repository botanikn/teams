package redis

import (
	"encoding/json"
	"testing"

	"task-api/internal/domain/entities"
)

func TestCachedTaskPreservesCreatorID(t *testing.T) {
	task := &entities.Task{ID: 1, CreatedBy: 42, CreatedByName: "Alice"}
	encoded, err := encodeCachedTask(task)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeCachedTask(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.CreatedBy != 42 || decoded.CreatedByName != "Alice" {
		t.Fatalf("decoded task = %+v, want creator ID 42 and name Alice", decoded)
	}
}

func TestCachedTaskListPreservesCreatorID(t *testing.T) {
	tasks := []entities.Task{
		{ID: 1, CreatedBy: 42, CreatedByName: "Alice"},
		{ID: 2, CreatedBy: 43, CreatedByName: "Bob"},
	}
	encoded, err := encodeCachedTasks(tasks)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeCachedTasks(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 2 || decoded[0].CreatedBy != 42 || decoded[1].CreatedBy != 43 {
		t.Fatalf("decoded tasks = %+v, want creator IDs 42 and 43", decoded)
	}

	var cacheRecord []map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &cacheRecord); err != nil {
		t.Fatal(err)
	}
	if len(cacheRecord) != 2 || string(cacheRecord[0]["created_by_id"]) != "42" {
		t.Fatalf("cache record did not persist internal creator ID: %s", encoded)
	}
}
