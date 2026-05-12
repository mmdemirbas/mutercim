package ocr

import "testing"

func TestNewToolWithBackend_dispatchesBackend(t *testing.T) {
	tests := []struct {
		name        string
		tool        string
		backend     string
		wantNil     bool
		wantBackend string
	}{
		{"qari-default-empty-backend", "qari", "", false, ""},
		{"qari-docker-backend", "qari", "docker", false, "docker"},
		{"qari-uv-backend", "qari", "uv", false, "uv"},
		{"unknown-tool-nil", "unknown", "uv", true, ""},
		{"empty-tool-nil", "", "uv", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := NewToolWithBackend(tt.tool, tt.backend)
			if tt.wantNil {
				if tool != nil {
					t.Errorf("expected nil for tool=%q, got %T", tt.tool, tool)
				}
				return
			}
			if tool == nil {
				t.Fatalf("expected non-nil tool for tool=%q backend=%q", tt.tool, tt.backend)
			}
			qari, ok := tool.(*QariTool)
			if !ok {
				t.Fatalf("expected *QariTool, got %T", tool)
			}
			if qari.Backend != tt.wantBackend {
				t.Errorf("Backend = %q, want %q", qari.Backend, tt.wantBackend)
			}
		})
	}
}

func TestNewTool_legacyDefaultsToDockerBackend(t *testing.T) {
	tool := NewTool("qari")
	qari, ok := tool.(*QariTool)
	if !ok {
		t.Fatalf("expected *QariTool, got %T", tool)
	}
	// Legacy NewTool delegates to NewToolWithBackend("", ...) so Backend
	// must remain empty (which the dispatch interprets as docker). This
	// pins back-compat so existing callers don't silently move to uv.
	if qari.Backend != "" {
		t.Errorf("legacy NewTool should leave Backend empty (docker default), got %q", qari.Backend)
	}
}
