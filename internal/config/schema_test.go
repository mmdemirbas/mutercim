package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGeneratedSchemaMatchesCommitted(t *testing.T) {
	generated, err := GenerateSchema()
	if err != nil {
		t.Fatalf("GenerateSchema: %v", err)
	}

	root := findModuleRoot(t)
	committed, err := os.ReadFile(filepath.Join(root, "config", "mutercim.schema.json"))
	if err != nil {
		t.Fatalf("read committed schema: %v", err)
	}

	var genObj, comObj any
	if err := json.Unmarshal(generated, &genObj); err != nil {
		t.Fatalf("unmarshal generated: %v", err)
	}
	if err := json.Unmarshal(committed, &comObj); err != nil {
		t.Fatalf("unmarshal committed: %v", err)
	}

	if !reflect.DeepEqual(genObj, comObj) {
		t.Error("committed mutercim.schema.json is out of date; run 'go run ./cmd/gen-schema' to update")
	}
}

func TestGenerateSchema_ValidJSON(t *testing.T) {
	data, err := GenerateSchema()
	if err != nil {
		t.Fatalf("GenerateSchema: %v", err)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Verify top-level schema fields
	if obj["$schema"] == nil {
		t.Error("missing $schema")
	}
	if obj["type"] != "object" {
		t.Errorf("root type should be 'object', got %v", obj["type"])
	}
	props, ok := obj["properties"].(map[string]any)
	if !ok {
		t.Fatal("missing properties")
	}

	// Verify key config sections exist
	for _, key := range []string{"inputs", "output", "cut", "read", "solve", "translate", "write", "knowledge"} {
		if props[key] == nil {
			t.Errorf("missing property %q", key)
		}
	}
}

func TestGenerateSchema_InputsRequiredPath(t *testing.T) {
	data, err := GenerateSchema()
	if err != nil {
		t.Fatalf("GenerateSchema: %v", err)
	}

	var obj map[string]any
	json.Unmarshal(data, &obj)

	props := obj["properties"].(map[string]any)
	inputs := props["inputs"].(map[string]any)
	items := inputs["items"].(map[string]any)

	if items["type"] != "object" {
		t.Errorf("inputs items should be object type, got %v", items["type"])
	}
	required, ok := items["required"].([]any)
	if !ok {
		t.Fatal("inputs items should have required")
	}
	if len(required) != 2 {
		t.Errorf("inputs items should require 2 fields, got %v", required)
	}
	requiredSet := make(map[string]bool)
	for _, r := range required {
		requiredSet[r.(string)] = true
	}
	if !requiredSet["path"] {
		t.Errorf("inputs items should require 'path', got %v", required)
	}
	if !requiredSet["languages"] {
		t.Errorf("inputs items should require 'languages', got %v", required)
	}
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find module root (go.mod)")
		}
		dir = parent
	}
}
