package layout

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/docker"
	"github.com/mmdemirbas/mutercim/internal/model"
)

// DefaultSuryaImage is the Docker image used for Surya layout detection.
const DefaultSuryaImage = "mutercim/surya:latest"

// SuryaTool uses the Surya layout detection model running in Docker
// to detect text regions with precise bounding boxes.
type SuryaTool struct {
	// DockerImage is the Docker image to use. Defaults to DefaultSuryaImage.
	DockerImage string

	// DockerfileDir is the path to docker/surya/ for auto-building.
	// Empty means skip auto-build (used in tests).
	DockerfileDir string

	// commander abstracts command execution for testing.
	commander Commander
}

// Commander abstracts os/exec for testability.
type Commander interface {
	// Run executes a command and returns its combined output.
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// execCommander uses os/exec.CommandContext.
type execCommander struct{}

func (e execCommander) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput() //nolint:gosec // G204: name is a docker binary path, args are trusted internal values
}

// NewSuryaTool creates a SuryaTool with the given Docker image.
// If image is empty, DefaultSuryaImage is used.
func NewSuryaTool(image string) *SuryaTool {
	if image == "" {
		image = DefaultSuryaImage
	}
	return &SuryaTool{
		DockerImage:   image,
		DockerfileDir: docker.FindDockerDir("surya"),
		commander:     execCommander{},
	}
}

// newSuryaToolWithCommander creates a SuryaTool with a custom commander for testing.
func newSuryaToolWithCommander(image string, cmd Commander) *SuryaTool {
	if image == "" {
		image = DefaultSuryaImage
	}
	return &SuryaTool{
		DockerImage: image,
		commander:   cmd,
	}
}

// Name returns "surya".
func (s *SuryaTool) Name() string {
	return model.LayoutSourceSurya
}

// Available checks if Docker is running and the Surya image exists.
func (s *SuryaTool) Available(ctx context.Context) bool {
	out, err := s.commander.Run(ctx, "docker", "info", "--format", "{{.ServerVersion}}")
	if err != nil {
		slog.Debug("docker not available", "error", err)
		return false
	}
	if strings.TrimSpace(string(out)) == "" {
		slog.Debug("docker info returned empty version")
		return false
	}

	out, err = s.commander.Run(ctx, "docker", "image", "inspect", s.DockerImage, "--format", "{{.ID}}")
	if err != nil {
		slog.Debug("surya image not found", "image", s.DockerImage, "error", err)
		return false
	}
	if strings.TrimSpace(string(out)) == "" {
		slog.Debug("surya image inspect returned empty ID", "image", s.DockerImage)
		return false
	}

	return true
}

// suryaOutput is the JSON structure returned by the Surya Docker container.
type suryaOutput struct {
	Regions []suryaRegion `json:"regions"`
}

type suryaRegion struct {
	BBox [4]int `json:"bbox"` // [x, y, width, height]
	Text string `json:"text"`
}

// knownSuryaParams lists the parameter names this tool recognizes.
var knownSuryaParams = map[string]bool{
	"languages": true,
}

// DetectRegions runs the Surya Docker container on the given image and
// returns detected regions with bounding boxes and preliminary OCR text.
//
// params supports these keys (all optional):
//   - languages (string): comma-separated OCR language codes, default "ar"
func (s *SuryaTool) DetectRegions(ctx context.Context, imagePath string, params map[string]any) (*DetectResult, error) {
	// Auto-build Docker image if needed
	if s.DockerfileDir != "" {
		if err := docker.EnsureImage(ctx, s.DockerImage, s.DockerfileDir); err != nil {
			return nil, fmt.Errorf("ensure surya image: %w", err)
		}
	}

	// Warn on unknown params
	for k := range params {
		if !knownSuryaParams[k] {
			slog.Warn("unknown layout_tool_param ignored", "param", k, "tool", "surya")
		}
	}

	dir := filepath.Dir(imagePath)
	base := filepath.Base(imagePath)
	args := []string{
		"run", "--rm",
		"-v", dir + ":/input:ro",
		s.DockerImage,
	}

	// Append tool params as CLI flags
	if v, ok := getString(params, "languages"); ok {
		args = append(args, "--languages", v)
	}

	args = append(args, "/input/"+base)

	out, err := s.commander.Run(ctx, "docker", args...)
	if err != nil {
		return nil, fmt.Errorf("surya docker run: %w\noutput: %s", err, string(out))
	}

	var result suryaOutput
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("surya parse output: %w\nraw: %s", err, string(out))
	}

	regions := make([]model.Region, len(result.Regions))
	for i, sr := range result.Regions {
		regions[i] = model.Region{
			ID:           fmt.Sprintf("r%d", i+1),
			BBox:         model.BBox(sr.BBox),
			Text:         sr.Text,
			LayoutSource: model.LayoutSourceSurya,
		}
	}

	return &DetectResult{Regions: regions}, nil
}

// getString extracts a string from a map[string]any.
func getString(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
