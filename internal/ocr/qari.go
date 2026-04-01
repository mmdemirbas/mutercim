package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mmdemirbas/mutercim/internal/docker"
)

const (
	// DefaultQariImage is the Docker image used for Qari-OCR.
	DefaultQariImage = "mutercim/qari-ocr:latest"

	// qariContainerName is the Docker container name for the running Qari-OCR server.
	qariContainerName = "mutercim-qari-ocr"

	// healthPollInterval is the interval between health checks during startup.
	healthPollInterval = 500 * time.Millisecond

	// startTimeout is the maximum time to wait for the model to load.
	startTimeout = 120 * time.Second
)

// QariTool implements the Tool interface for Qari-OCR via a Docker HTTP server.
type QariTool struct {
	DockerImage   string
	DockerfileDir string
	port          int // dynamically assigned port
	client        *http.Client
}

// NewQariTool creates a QariTool with the given Docker image.
func NewQariTool(image string) *QariTool {
	if image == "" {
		image = DefaultQariImage
	}
	return &QariTool{
		DockerImage:   image,
		DockerfileDir: docker.FindDockerDir("qari-ocr"),
		client:        &http.Client{Timeout: 120 * time.Second},
	}
}

// Name returns "qari".
func (q *QariTool) Name() string { return "qari" }

// Start ensures the Docker image exists and starts the Qari-OCR container.
// If a container with the same name is already running and healthy, it is reused.
//nolint:cyclop,gocognit // Docker container lifecycle with health checking
func (q *QariTool) Start(ctx context.Context) error {
	// Ensure Docker image exists
	if q.DockerfileDir != "" {
		slog.Info("ensuring qari-ocr docker image", "image", q.DockerImage, "dockerfile_dir", q.DockerfileDir)
		if err := docker.EnsureImage(ctx, q.DockerImage, q.DockerfileDir); err != nil {
			return fmt.Errorf("ensure qari-ocr image: %w", err)
		}
		slog.Info("qari-ocr docker image ready", "image", q.DockerImage)
	}

	// Check if container is already running and healthy
	if port, ok := q.findExistingContainer(ctx); ok {
		q.port = port
		if q.IsReady(ctx) {
			slog.Info("reusing existing qari-ocr container", "port", port)
			return nil
		}
		// Container exists but not healthy — stop and restart
		slog.Warn("qari-ocr container exists but not healthy, restarting", "port", port)
		q.stopContainer(ctx)
	}

	// Find a free port
	port, err := freePort()
	if err != nil {
		return fmt.Errorf("find free port: %w", err)
	}
	q.port = port

	// Start container
	slog.Info("starting qari-ocr container", "port", port, "image", q.DockerImage)
	args := []string{
		"run", "-d", "--rm",
		"--name", qariContainerName,
		"-p", fmt.Sprintf("%d:8000", port),
		q.DockerImage,
	}

	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput() //nolint:gosec // G204: docker is a fixed binary; args are constructed by internal callers
	if err != nil {
		slog.Error("docker run qari-ocr failed", "args", args, "output", strings.TrimSpace(string(out)), "error", err)
		return fmt.Errorf("docker run qari-ocr: %w\noutput: %s", err, string(out))
	}
	slog.Debug("docker run qari-ocr started", "container_id", strings.TrimSpace(string(out)))

	// Poll health endpoint until ready
	start := time.Now()
	ticker := time.NewTicker(healthPollInterval)
	defer ticker.Stop()

	slog.Info("waiting for qari-ocr model to load", "timeout", startTimeout)

	for {
		select {
		case <-ctx.Done():
			slog.Warn("qari-ocr startup interrupted", "elapsed_s", int(time.Since(start).Seconds()))
			return ctx.Err()
		case <-ticker.C:
			if q.IsReady(ctx) {
				elapsed := time.Since(start)
				slog.Info("qari-ocr model loaded", "elapsed_s", int(elapsed.Seconds()), "port", port)
				return nil
			}
			elapsed := time.Since(start)
			if elapsed > startTimeout {
				// Grab container logs before stopping
				logs := q.containerLogs(ctx)
				slog.Error("qari-ocr startup timed out", "elapsed_s", int(elapsed.Seconds()), "timeout_s", int(startTimeout.Seconds()), "container_logs", logs)
				q.stopContainer(ctx)
				return fmt.Errorf("qari-ocr startup timed out after %v — container logs:\n%s", startTimeout, logs)
			}
			// Log progress every 10 seconds
			if int(elapsed.Seconds())%10 == 0 && elapsed.Seconds() > 1 {
				slog.Debug("qari-ocr still loading", "elapsed_s", int(elapsed.Seconds()))
			}
		}
	}
}

// Stop stops the Qari-OCR container. Tolerant of already-stopped containers.
func (q *QariTool) Stop(ctx context.Context) error {
	q.stopContainer(ctx)
	return nil
}

// IsReady checks if the Qari-OCR server is ready to accept requests.
func (q *QariTool) IsReady(ctx context.Context) bool {
	if q.port == 0 {
		return false
	}
	url := fmt.Sprintf("http://localhost:%d/health", q.port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := q.client.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var health struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return false
	}
	return health.Status == "ready"
}

// RecognizeRegions OCRs cropped regions from a page image via POST /ocr/regions.
//
//nolint:cyclop,funlen // HTTP multipart construction with multi-step error handling
func (q *QariTool) RecognizeRegions(ctx context.Context, imagePath string, regions []RegionInput) (*Result, error) {
	if q.port == 0 {
		return nil, fmt.Errorf("qari-ocr not started")
	}

	slog.Debug("ocr regions request", "image", imagePath, "regions", len(regions), "port", q.port)

	// Build multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add image file
	if err := addFileField(writer, "image", imagePath); err != nil {
		return nil, fmt.Errorf("add image to request: %w", err)
	}

	// Add regions JSON
	type regionReq struct {
		ID   string `json:"id"`
		BBox [4]int `json:"bbox"`
	}
	reqs := make([]regionReq, len(regions))
	for i, r := range regions {
		reqs[i] = regionReq(r)
	}
	regionsJSON, err := json.Marshal(reqs)
	if err != nil {
		return nil, fmt.Errorf("marshal regions: %w", err)
	}
	if err := writer.WriteField("regions", string(regionsJSON)); err != nil {
		return nil, fmt.Errorf("add regions field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	url := fmt.Sprintf("http://localhost:%d/ocr/regions", q.port)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := q.client.Do(req)
	if err != nil {
		slog.Error("ocr regions request failed", "image", imagePath, "error", err, "port", q.port)
		return nil, fmt.Errorf("ocr regions request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		slog.Error("ocr regions failed", "image", imagePath, "status", resp.StatusCode, "body", string(respBody))
		return nil, fmt.Errorf("ocr regions failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var apiResp struct {
		Results []struct {
			ID        string `json:"id"`
			Text      string `json:"text"`
			ElapsedMs int    `json:"elapsed_ms"`
		} `json:"results"`
		Model        string `json:"model"`
		TotalElapsed int    `json:"total_elapsed_ms"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode ocr regions response: %w", err)
	}

	results := make([]RegionResult, len(apiResp.Results))
	for i, r := range apiResp.Results {
		results[i] = RegionResult{
			ID:        r.ID,
			Text:      r.Text,
			ElapsedMs: r.ElapsedMs,
		}
	}

	slog.Debug("ocr regions complete", "image", imagePath, "regions", len(results), "total_ms", apiResp.TotalElapsed)

	return &Result{
		Regions: results,
		Model:   apiResp.Model,
		TotalMs: apiResp.TotalElapsed,
	}, nil
}

// RecognizeFullPage OCRs an entire page image via POST /ocr.
func (q *QariTool) RecognizeFullPage(ctx context.Context, imagePath string) (*Result, error) {
	if q.port == 0 {
		return nil, fmt.Errorf("qari-ocr not started")
	}

	slog.Debug("ocr full page request", "image", imagePath, "port", q.port)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := addFileField(writer, "image", imagePath); err != nil {
		return nil, fmt.Errorf("add image to request: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	url := fmt.Sprintf("http://localhost:%d/ocr", q.port)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := q.client.Do(req)
	if err != nil {
		slog.Error("ocr full page request failed", "image", imagePath, "error", err, "port", q.port)
		return nil, fmt.Errorf("ocr request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		slog.Error("ocr full page failed", "image", imagePath, "status", resp.StatusCode, "body", string(respBody))
		return nil, fmt.Errorf("ocr failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var apiResp struct {
		Text      string `json:"text"`
		Model     string `json:"model"`
		ElapsedMs int    `json:"elapsed_ms"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode ocr response: %w", err)
	}

	slog.Debug("ocr full page complete", "image", imagePath, "chars", len(apiResp.Text), "elapsed_ms", apiResp.ElapsedMs)

	return &Result{
		FullText: apiResp.Text,
		Model:    apiResp.Model,
		TotalMs:  apiResp.ElapsedMs,
	}, nil
}

// findExistingContainer checks if a Qari-OCR container is already running and returns its port.
func (q *QariTool) findExistingContainer(ctx context.Context) (int, bool) {
	out, err := exec.CommandContext(ctx, "docker", "inspect",
		"--format", "{{range .NetworkSettings.Ports}}{{range .}}{{.HostPort}}{{end}}{{end}}",
		qariContainerName).CombinedOutput()
	if err != nil {
		slog.Debug("no existing qari-ocr container found", "error", err)
		return 0, false
	}
	portStr := strings.TrimSpace(string(out))
	if portStr == "" {
		slog.Debug("existing qari-ocr container has no port mapping", "output", string(out))
		return 0, false
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		slog.Warn("failed to parse qari-ocr container port", "port_str", portStr, "error", err)
		return 0, false
	}
	slog.Debug("found existing qari-ocr container", "port", port)
	return port, true
}

// stopContainer stops the Qari-OCR container. Tolerant of already-stopped containers.
func (q *QariTool) stopContainer(ctx context.Context) {
	slog.Debug("stopping qari-ocr container", "container", qariContainerName)
	out, err := exec.CommandContext(ctx, "docker", "stop", qariContainerName).CombinedOutput()
	if err != nil {
		slog.Debug("qari-ocr container stop (may already be stopped)", "output", strings.TrimSpace(string(out)), "error", err)
	} else {
		slog.Info("qari-ocr container stopped")
	}
}

// containerLogs returns recent logs from the Qari-OCR container (last 50 lines).
func (q *QariTool) containerLogs(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "docker", "logs", "--tail", "50", qariContainerName).CombinedOutput()
	if err != nil {
		return fmt.Sprintf("<failed to get container logs: %v>", err)
	}
	return strings.TrimSpace(string(out))
}

// addFileField adds a file to a multipart writer.
func addFileField(writer *multipart.Writer, fieldName, filePath string) error {
	f, err := os.Open(filePath) //nolint:gosec // G304: filePath is an internal image path, not user-controlled HTTP input
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	part, err := writer.CreateFormFile(fieldName, f.Name())
	if err != nil {
		return err
	}
	_, err = io.Copy(part, f)
	return err
}

// freePort finds a free TCP port by binding to :0 and releasing it.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port, nil
}
