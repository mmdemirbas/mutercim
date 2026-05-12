package layout

import "testing"

func TestNewToolWithBackend_PropagatesBackendToDocLayout(t *testing.T) {
	tests := []struct {
		name        string
		toolName    string
		backend     string
		wantNil     bool
		wantBackend string
	}{
		{"doclayout-empty-backend", "doclayout-yolo", "", false, ""},
		{"doclayout-docker", "doclayout-yolo", "docker", false, "docker"},
		{"doclayout-uv", "doclayout-yolo", "uv", false, "uv"},
		{"surya-still-built", "surya", "uv", false, ""}, // surya migration pending; backend ignored
		{"unknown-tool-nil", "x", "uv", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewToolWithBackend(tt.toolName, tt.backend)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil for tool=%q, got %T", tt.toolName, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected non-nil for %q", tt.toolName)
			}
			if d, ok := got.(*DocLayoutTool); ok {
				if d.Backend != tt.wantBackend {
					t.Errorf("DocLayoutTool.Backend = %q, want %q", d.Backend, tt.wantBackend)
				}
			}
		})
	}
}

func TestNewTool_LegacyDefaultsToDockerBackend(t *testing.T) {
	tool := NewTool("doclayout-yolo")
	d, ok := tool.(*DocLayoutTool)
	if !ok {
		t.Fatalf("expected *DocLayoutTool, got %T", tool)
	}
	// Legacy NewTool delegates to NewToolWithBackend("", ...) so Backend
	// must remain empty (docker default). Pins back-compat for existing
	// callers — silently moving them to uv would surprise users.
	if d.Backend != "" {
		t.Errorf("legacy NewTool should leave Backend empty, got %q", d.Backend)
	}
}
