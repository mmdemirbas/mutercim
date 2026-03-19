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
	for _, key := range []string{"book", "inputs", "read", "translate", "write", "sections", "retry", "rate_limit"} {
		if props[key] == nil {
			t.Errorf("missing property %q", key)
		}
	}
}

func TestGenerateSchema_SectionTypeEnumsFromModel(t *testing.T) {
	data, err := GenerateSchema()
	if err != nil {
		t.Fatalf("GenerateSchema: %v", err)
	}

	var obj map[string]any
	json.Unmarshal(data, &obj)

	// Navigate to sections[].type.enum
	props := obj["properties"].(map[string]any)
	sections := props["sections"].(map[string]any)
	items := sections["items"].(map[string]any)
	itemProps := items["properties"].(map[string]any)
	typeField := itemProps["type"].(map[string]any)
	enumRaw := typeField["enum"].([]any)

	enums := make([]string, len(enumRaw))
	for i, v := range enumRaw {
		enums[i] = v.(string)
	}

	// Verify all model.ValidSectionTypes are represented
	got := sectionTypeEnums()
	if !reflect.DeepEqual(enums, got) {
		t.Errorf("section type enums mismatch:\n  schema: %v\n  model:  %v", enums, got)
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
	if len(required) != 1 || required[0] != "path" {
		t.Errorf("inputs items should require 'path', got %v", required)
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
