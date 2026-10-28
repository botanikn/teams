package domain

import (
	"encoding/json"
	"testing"
)

func TestTaskJSONUsesCreatorNameInsteadOfID(t *testing.T) {
	raw, err := json.Marshal(Task{CreatedBy: 42, CreatedByName: "Alice"})
	if err != nil {
		t.Fatal(err)
	}

	var response map[string]any
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
	if response["created_by"] != "Alice" {
		t.Fatalf("created_by = %v, want Alice", response["created_by"])
	}
}
