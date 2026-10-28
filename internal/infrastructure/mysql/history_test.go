package mysql

import (
	"encoding/json"
	"testing"
)

func TestReplaceHistoryCreatorID(t *testing.T) {
	changes := json.RawMessage(`{"new":{"id":7,"created_by":42,"title":"Task"}}`)
	got, err := replaceHistoryCreatorID(changes, "Alice")
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]map[string]any
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatal(err)
	}
	if result["new"]["created_by"] != "Alice" {
		t.Fatalf("created_by = %v, want Alice", result["new"]["created_by"])
	}
}

func TestReplaceHistoryCreatorIDLeavesOtherChangesUntouched(t *testing.T) {
	changes := json.RawMessage(`{"title":{"old":"Before","new":"After"}}`)
	got, err := replaceHistoryCreatorID(changes, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(changes) {
		t.Fatalf("changes = %s, want unchanged %s", got, changes)
	}
}
