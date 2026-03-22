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
	Quantize      string // "8bit" or "none"
	port          int    // dynamically assigned port
	client        *http.Client
}

// NewQariTool creates a QariTool with the given Docker image and quantization setting.
func NewQariTool(image, quantize string) *QariTool {
	if image == "" {
		image = DefaultQariImage
	}
	if quantize == "" {
		quantize = "8bit"
	}
	return &QariTool{
		DockerImage:   image,
		DockerfileDir: docker.FindDockerDir("qari-ocr"),
		Quantize:      quantize,
		client:        &http.Client{Timeout: 120 * time.Second},
	}
}

// Name returns "qari".
func (q *QariTool) Name() string { return "qari" }

// Start ensures the Docker image exists and starts the Qari-OCR container.
// If a container with the same name is already running and healthy, it is reused.
func (q *QariTool) Start(ctx context.Context) error {
	// Ensure Docker image exists
	if q.DockerfileDir != "" {
		slog.Info("building qari-ocr docker image (first run, may take several minutes)")
		if err := docker.EnsureImage(ctx, q.DockerImage, q.DockerfileDir); err != nil {
			return fmt.Errorf("ensure qari-ocr image: %w", err)
		}
	}

	// Check if container is already running and healthy
	if port, ok := q.findExistingContainer(ctx); ok {
		q.port = port
		if q.IsReady(ctx) {
			slog.Info("reusing existing qari-ocr container", "port", port)
			return nil
		}
		// Container exists but not healthy — stop and restart
		q.stopContainer(ctx)
	}

	// Find a free port
	port, err := freePort()
	if err != nil {
		return fmt.Errorf("find free port: %w", err)
	}
	q.port = port

	// Start container
	quantizeEnv := q.Quantize
	if quantizeEnv == "" {
		quantizeEnv = "8bit"
	}

	slog.Info("starting qari-ocr container", "port", port, "quantize", quantizeEnv)
	args := []string{
		"run", "-d", "--rm",
		"--name", qariContainerName,
		"-p", fmt.Sprintf("%d:8000", port),
		"-e", "QUANTIZE=" + quantizeEnv,
		q.DockerImage,
	}

	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker run qari-ocr: %w\noutput: %s", err, string(out))
	}

	// Poll health endpoint until ready
	start := time.Now()
	ticker := time.NewTicker(healthPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if q.IsReady(ctx) {
				elapsed := time.Since(start)
				slog.Info("qari-ocr model loaded", "elapsed_s", int(elapsed.Seconds()))
				return nil
			}
			if time.Since(start) > startTimeout {
				q.stopContainer(ctx)
				return fmt.Errorf("qari-ocr startup timed out after %v", startTimeout)
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
	defer resp.Body.Close()

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
func (q *QariTool) RecognizeRegions(ctx context.Context, imagePath string, regions []RegionInput) (*Result, error) {
	if q.port == 0 {
		return nil, fmt.Errorf("qari-ocr not started")
	}

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
		reqs[i] = regionReq{ID: r.ID, BBox: r.BBox}
	}
	regionsJSON, err := json.Marshal(reqs)
	if err != nil {
		return nil, fmt.Errorf("marshal regions: %w", err)
	}
	if err := writer.WriteField("regions", string(regionsJSON)); err != nil {
		return nil, fmt.Errorf("add regions field: %w", err)
	}

	writer.Close()

	url := fmt.Sprintf("http://localhost:%d/ocr/regions", q.port)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := q.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ocr regions request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
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
		return nil, fmt.Errorf("decode response: %w", err)
	}

	results := make([]RegionResult, len(apiResp.Results))
	for i, r := range apiResp.Results {
		results[i] = RegionResult{
			ID:        r.ID,
			Text:      r.Text,
			ElapsedMs: r.ElapsedMs,
		}
	}

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

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := addFileField(writer, "image", imagePath); err != nil {
		return nil, fmt.Errorf("add image to request: %w", err)
	}
	writer.Close()

	url := fmt.Sprintf("http://localhost:%d/ocr", q.port)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := q.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ocr request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ocr failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var apiResp struct {
		Text      string `json:"text"`
		Model     string `json:"model"`
		ElapsedMs int    `json:"elapsed_ms"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

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
		return 0, false
	}
	portStr := strings.TrimSpace(string(out))
	if portStr == "" {
		return 0, false
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		return 0, false
	}
	return port, true
}

// stopContainer stops the Qari-OCR container. Tolerant of already-stopped containers.
func (q *QariTool) stopContainer(ctx context.Context) {
	out, err := exec.CommandContext(ctx, "docker", "stop", qariContainerName).CombinedOutput()
	if err != nil {
		slog.Debug("qari-ocr container stop", "output", strings.TrimSpace(string(out)), "error", err)
	} else {
		slog.Info("qari-ocr container stopped")
	}
}

// addFileField adds a file to a multipart writer.
func addFileField(writer *multipart.Writer, fieldName, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

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
	l.Close()
	return port, nil
}
