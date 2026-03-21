package layout

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mmdemirbas/mutercim/internal/model"
)

// mockCommander records calls and returns configured responses.
type mockCommander struct {
	calls   []mockCall
	returns []mockReturn
}

type mockCall struct {
	name string
	args []string
}

type mockReturn struct {
	output []byte
	err    error
}

func (m *mockCommander) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	m.calls = append(m.calls, mockCall{name: name, args: args})
	idx := len(m.calls) - 1
	if idx < len(m.returns) {
		return m.returns[idx].output, m.returns[idx].err
	}
	return nil, fmt.Errorf("unexpected call %d: %s %v", idx, name, args)
}

func TestNoneTool_DetectRegions(t *testing.T) {
	tool := NoneTool{}
	regions, err := tool.DetectRegions(context.Background(), "/any/path.png")
	if err != nil {
		t.Fatalf("DetectRegions: %v", err)
	}
	if regions != nil {
		t.Errorf("regions = %v, want nil", regions)
	}
}

func TestNoneTool_Available(t *testing.T) {
	tool := NoneTool{}
	if !tool.Available(context.Background()) {
		t.Error("Available = false, want true")
	}
}

func TestNoneTool_Name(t *testing.T) {
	tool := NoneTool{}
	if got := tool.Name(); got != "" {
		t.Errorf("Name = %q, want empty", got)
	}
}

func TestSuryaTool_Name(t *testing.T) {
	tool := NewSuryaTool("")
	if got := tool.Name(); got != "surya" {
		t.Errorf("Name = %q, want %q", got, "surya")
	}
}

func TestSuryaTool_DefaultImage(t *testing.T) {
	tool := NewSuryaTool("")
	if tool.DockerImage != DefaultSuryaImage {
		t.Errorf("DockerImage = %q, want %q", tool.DockerImage, DefaultSuryaImage)
	}
}

func TestSuryaTool_CustomImage(t *testing.T) {
	tool := NewSuryaTool("my-surya:v2")
	if tool.DockerImage != "my-surya:v2" {
		t.Errorf("DockerImage = %q, want %q", tool.DockerImage, "my-surya:v2")
	}
}

func TestSuryaTool_Available_DockerNotRunning(t *testing.T) {
	cmd := &mockCommander{
		returns: []mockReturn{
			{output: nil, err: fmt.Errorf("Cannot connect to the Docker daemon")},
		},
	}
	tool := newSuryaToolWithCommander("", cmd)

	if tool.Available(context.Background()) {
		t.Error("Available = true, want false (docker not running)")
	}
	if len(cmd.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(cmd.calls))
	}
	if cmd.calls[0].name != "docker" {
		t.Errorf("call[0].name = %q, want %q", cmd.calls[0].name, "docker")
	}
}

func TestSuryaTool_Available_EmptyDockerVersion(t *testing.T) {
	cmd := &mockCommander{
		returns: []mockReturn{
			{output: []byte(""), err: nil},
		},
	}
	tool := newSuryaToolWithCommander("", cmd)

	if tool.Available(context.Background()) {
		t.Error("Available = true, want false (empty version)")
	}
}

func TestSuryaTool_Available_ImageNotPulled(t *testing.T) {
	cmd := &mockCommander{
		returns: []mockReturn{
			{output: []byte("24.0.7\n"), err: nil},
			{output: nil, err: fmt.Errorf("No such image")},
		},
	}
	tool := newSuryaToolWithCommander("", cmd)

	if tool.Available(context.Background()) {
		t.Error("Available = true, want false (image not pulled)")
	}
	if len(cmd.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(cmd.calls))
	}
}

func TestSuryaTool_Available_EmptyImageID(t *testing.T) {
	cmd := &mockCommander{
		returns: []mockReturn{
			{output: []byte("24.0.7\n"), err: nil},
			{output: []byte("  \n"), err: nil},
		},
	}
	tool := newSuryaToolWithCommander("", cmd)

	if tool.Available(context.Background()) {
		t.Error("Available = true, want false (empty image ID)")
	}
}

func TestSuryaTool_Available_Success(t *testing.T) {
	cmd := &mockCommander{
		returns: []mockReturn{
			{output: []byte("24.0.7\n"), err: nil},
			{output: []byte("sha256:abc123\n"), err: nil},
		},
	}
	tool := newSuryaToolWithCommander("", cmd)

	if !tool.Available(context.Background()) {
		t.Error("Available = false, want true")
	}
	if len(cmd.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(cmd.calls))
	}

	// Verify docker image inspect uses correct image name
	inspectArgs := cmd.calls[1].args
	found := false
	for _, arg := range inspectArgs {
		if arg == DefaultSuryaImage {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("docker image inspect should use image %q, args = %v", DefaultSuryaImage, inspectArgs)
	}
}

func TestSuryaTool_DetectRegions_Success(t *testing.T) {
	suryaJSON := suryaOutput{
		Regions: []suryaRegion{
			{BBox: [4]int{400, 50, 700, 60}, Text: "حرف الألف"},
			{BBox: [4]int{800, 150, 600, 600}, Text: "اذهبي فاطعمي"},
			{BBox: [4]int{100, 150, 600, 600}, Text: "اذهبي فقد"},
		},
	}
	out, _ := json.Marshal(suryaJSON)

	cmd := &mockCommander{
		returns: []mockReturn{
			{output: out, err: nil},
		},
	}
	tool := newSuryaToolWithCommander("", cmd)

	regions, err := tool.DetectRegions(context.Background(), "/tmp/page.png")
	if err != nil {
		t.Fatalf("DetectRegions: %v", err)
	}
	if len(regions) != 3 {
		t.Fatalf("len(regions) = %d, want 3", len(regions))
	}

	// Verify regions
	if regions[0].ID != "r1" {
		t.Errorf("regions[0].ID = %q, want %q", regions[0].ID, "r1")
	}
	if regions[0].BBox != (model.BBox{400, 50, 700, 60}) {
		t.Errorf("regions[0].BBox = %v, want [400,50,700,60]", regions[0].BBox)
	}
	if regions[0].Text != "حرف الألف" {
		t.Errorf("regions[0].Text = %q, want %q", regions[0].Text, "حرف الألف")
	}
	if regions[0].LayoutSource != model.LayoutSourceSurya {
		t.Errorf("regions[0].LayoutSource = %q, want %q", regions[0].LayoutSource, model.LayoutSourceSurya)
	}

	if regions[2].ID != "r3" {
		t.Errorf("regions[2].ID = %q, want %q", regions[2].ID, "r3")
	}

	// Verify docker command args
	call := cmd.calls[0]
	if call.name != "docker" {
		t.Errorf("call.name = %q, want %q", call.name, "docker")
	}
	// Check that the directory is mounted (not the single file)
	foundMount := false
	for _, arg := range call.args {
		if arg == "/tmp:/input:ro" {
			foundMount = true
			break
		}
	}
	if !foundMount {
		t.Errorf("expected directory volume mount /tmp:/input:ro, args = %v", call.args)
	}
	// Check that the container path uses the base filename
	foundPath := false
	for _, arg := range call.args {
		if arg == "/input/page.png" {
			foundPath = true
			break
		}
	}
	if !foundPath {
		t.Errorf("expected container path /input/page.png, args = %v", call.args)
	}
}

func TestSuryaTool_DetectRegions_DockerError(t *testing.T) {
	cmd := &mockCommander{
		returns: []mockReturn{
			{output: []byte("error running container"), err: fmt.Errorf("exit status 1")},
		},
	}
	tool := newSuryaToolWithCommander("", cmd)

	_, err := tool.DetectRegions(context.Background(), "/tmp/page.png")
	if err == nil {
		t.Fatal("DetectRegions: expected error, got nil")
	}
	if got := err.Error(); !contains(got, "surya docker run") {
		t.Errorf("error = %q, want to contain %q", got, "surya docker run")
	}
}

func TestSuryaTool_DetectRegions_InvalidJSON(t *testing.T) {
	cmd := &mockCommander{
		returns: []mockReturn{
			{output: []byte("not json at all"), err: nil},
		},
	}
	tool := newSuryaToolWithCommander("", cmd)

	_, err := tool.DetectRegions(context.Background(), "/tmp/page.png")
	if err == nil {
		t.Fatal("DetectRegions: expected error, got nil")
	}
	if got := err.Error(); !contains(got, "surya parse output") {
		t.Errorf("error = %q, want to contain %q", got, "surya parse output")
	}
}

func TestSuryaTool_DetectRegions_EmptyRegions(t *testing.T) {
	out, _ := json.Marshal(suryaOutput{Regions: []suryaRegion{}})
	cmd := &mockCommander{
		returns: []mockReturn{
			{output: out, err: nil},
		},
	}
	tool := newSuryaToolWithCommander("", cmd)

	regions, err := tool.DetectRegions(context.Background(), "/tmp/page.png")
	if err != nil {
		t.Fatalf("DetectRegions: %v", err)
	}
	if len(regions) != 0 {
		t.Errorf("len(regions) = %d, want 0", len(regions))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
