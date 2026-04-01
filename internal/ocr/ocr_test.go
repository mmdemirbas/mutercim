package ocr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// mockTool implements Tool for testing.
type mockTool struct {
	name       string
	started    bool
	stopped    bool
	ready      bool
	regionsFn  func(ctx context.Context, imagePath string, regions []RegionInput) (*Result, error)
	fullPageFn func(ctx context.Context, imagePath string) (*Result, error)
}

func (m *mockTool) Name() string                   { return m.name }
func (m *mockTool) Start(_ context.Context) error  { m.started = true; return nil }
func (m *mockTool) Stop(_ context.Context) error   { m.stopped = true; return nil }
func (m *mockTool) IsReady(_ context.Context) bool { return m.ready }
func (m *mockTool) RecognizeRegions(ctx context.Context, imagePath string, regions []RegionInput) (*Result, error) {
	if m.regionsFn != nil {
		return m.regionsFn(ctx, imagePath, regions)
	}
	return &Result{}, nil
}
func (m *mockTool) RecognizeFullPage(ctx context.Context, imagePath string) (*Result, error) {
	if m.fullPageFn != nil {
		return m.fullPageFn(ctx, imagePath)
	}
	return &Result{}, nil
}

func TestNewTool_qari(t *testing.T) {
	tool := NewTool("qari")
	if tool == nil {
		t.Fatal("NewTool(qari) returned nil")
	}
	if tool.Name() != "qari" {
		t.Errorf("Name() = %q, want qari", tool.Name())
	}
}

func TestNewTool_empty(t *testing.T) {
	tool := NewTool("")
	if tool != nil {
		t.Errorf("NewTool('') should return nil, got %v", tool)
	}
}

func TestNewTool_unknown(t *testing.T) {
	tool := NewTool("unknown")
	if tool != nil {
		t.Errorf("NewTool(unknown) should return nil, got %v", tool)
	}
}

func TestNoopTool(t *testing.T) {
	noop := &NoopTool{}
	ctx := context.Background()

	if noop.Name() != "" {
		t.Errorf("NoopTool.Name() = %q, want empty", noop.Name())
	}
	if err := noop.Start(ctx); err != nil {
		t.Errorf("NoopTool.Start() error: %v", err)
	}
	if err := noop.Stop(ctx); err != nil {
		t.Errorf("NoopTool.Stop() error: %v", err)
	}
	if noop.IsReady(ctx) {
		t.Error("NoopTool.IsReady() should return false")
	}

	_, err := noop.RecognizeRegions(ctx, "test.png", nil)
	if err != ErrDisabled {
		t.Errorf("NoopTool.RecognizeRegions() error = %v, want ErrDisabled", err)
	}

	_, err = noop.RecognizeFullPage(ctx, "test.png")
	if err != ErrDisabled {
		t.Errorf("NoopTool.RecognizeFullPage() error = %v, want ErrDisabled", err)
	}
}

func TestMockTool_interface(t *testing.T) {
	// Ensure mockTool implements Tool interface
	var _ Tool = &mockTool{}
}

func TestRegionInput(t *testing.T) {
	r := RegionInput{
		ID:   "r1",
		BBox: [4]int{10, 20, 100, 200},
	}
	if r.ID != "r1" {
		t.Errorf("ID = %q, want r1", r.ID)
	}
	if r.BBox[0] != 10 || r.BBox[1] != 20 || r.BBox[2] != 100 || r.BBox[3] != 200 {
		t.Errorf("BBox = %v, want [10,20,100,200]", r.BBox)
	}
}

func TestResult_regions(t *testing.T) {
	r := &Result{
		Regions: []RegionResult{
			{ID: "r1", Text: "hello", ElapsedMs: 100},
			{ID: "r2", Text: "world", ElapsedMs: 200},
		},
		Model:   "test-model",
		TotalMs: 300,
	}
	if len(r.Regions) != 2 {
		t.Fatalf("Regions len = %d, want 2", len(r.Regions))
	}
	if r.Regions[0].Text != "hello" {
		t.Errorf("Regions[0].Text = %q, want hello", r.Regions[0].Text)
	}
	if r.Model != "test-model" {
		t.Errorf("Model = %q, want test-model", r.Model)
	}
}

func TestResult_fullText(t *testing.T) {
	r := &Result{
		FullText: "full page text",
		Model:    "test-model",
		TotalMs:  500,
	}
	if r.FullText != "full page text" {
		t.Errorf("FullText = %q, want 'full page text'", r.FullText)
	}
	if len(r.Regions) != 0 {
		t.Errorf("Regions should be empty for full text, got %d", len(r.Regions))
	}
}

func TestQariTool_health(t *testing.T) {
	// Create a test HTTP server that responds to /health
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready", "model": "test"})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	// Parse port from test server URL
	var port int
	if _, err := fmt.Sscanf(srv.URL, "http://127.0.0.1:%d", &port); err != nil {
		t.Fatalf("parse port: %v", err)
	}

	q := &QariTool{
		port:   port,
		client: srv.Client(),
	}

	if !q.IsReady(context.Background()) {
		t.Error("IsReady should return true when health returns ready")
	}
}

func TestQariTool_healthNotReady(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "loading"})
	}))
	defer srv.Close()

	var port int
	if _, err := fmt.Sscanf(srv.URL, "http://127.0.0.1:%d", &port); err != nil {
		t.Fatalf("parse port: %v", err)
	}

	q := &QariTool{
		port:   port,
		client: srv.Client(),
	}

	if q.IsReady(context.Background()) {
		t.Error("IsReady should return false when health returns 503")
	}
}

func TestQariTool_noPort(t *testing.T) {
	q := &QariTool{}

	if q.IsReady(context.Background()) {
		t.Error("IsReady should return false when port is 0")
	}

	_, err := q.RecognizeFullPage(context.Background(), "test.png")
	if err == nil {
		t.Error("RecognizeFullPage should fail when port is 0")
	}

	_, err = q.RecognizeRegions(context.Background(), "test.png", nil)
	if err == nil {
		t.Error("RecognizeRegions should fail when port is 0")
	}
}

func TestFreePort(t *testing.T) {
	port, err := freePort()
	if err != nil {
		t.Fatalf("freePort() error: %v", err)
	}
	if port <= 0 {
		t.Errorf("freePort() = %d, want > 0", port)
	}
}

func TestQariTool_fullPageOCR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ocr" && r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"text":       "extracted text",
				"model":      "test-model",
				"elapsed_ms": 1000,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	var port int
	if _, err := fmt.Sscanf(srv.URL, "http://127.0.0.1:%d", &port); err != nil {
		t.Fatalf("parse port: %v", err)
	}

	q := &QariTool{
		port:   port,
		client: srv.Client(),
	}

	// Create a temp image file
	tmpDir := t.TempDir()
	imgPath := tmpDir + "/test.png"
	if err := createTestImage(imgPath); err != nil {
		t.Fatalf("create test image: %v", err)
	}

	result, err := q.RecognizeFullPage(context.Background(), imgPath)
	if err != nil {
		t.Fatalf("RecognizeFullPage() error: %v", err)
	}
	if result.FullText != "extracted text" {
		t.Errorf("FullText = %q, want 'extracted text'", result.FullText)
	}
	if result.Model != "test-model" {
		t.Errorf("Model = %q, want 'test-model'", result.Model)
	}
	if result.TotalMs != 1000 {
		t.Errorf("TotalMs = %d, want 1000", result.TotalMs)
	}
}

//nolint:cyclop // integration test with detailed response field validation
func TestQariTool_recognizeRegions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ocr/regions" && r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{
					{"id": "r1", "text": "region 1 text", "elapsed_ms": 500},
					{"id": "r2", "text": "region 2 text", "elapsed_ms": 600},
				},
				"model":            "test-model",
				"total_elapsed_ms": 1100,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	var port int
	if _, err := fmt.Sscanf(srv.URL, "http://127.0.0.1:%d", &port); err != nil {
		t.Fatalf("parse port: %v", err)
	}

	q := &QariTool{
		port:   port,
		client: srv.Client(),
	}

	tmpDir := t.TempDir()
	imgPath := tmpDir + "/test.png"
	if err := createTestImage(imgPath); err != nil {
		t.Fatalf("create test image: %v", err)
	}

	regions := []RegionInput{
		{ID: "r1", BBox: [4]int{10, 20, 100, 80}},
		{ID: "r2", BBox: [4]int{10, 100, 100, 200}},
	}

	result, err := q.RecognizeRegions(context.Background(), imgPath, regions)
	if err != nil {
		t.Fatalf("RecognizeRegions() error: %v", err)
	}
	if len(result.Regions) != 2 {
		t.Fatalf("Regions len = %d, want 2", len(result.Regions))
	}
	if result.Regions[0].ID != "r1" || result.Regions[0].Text != "region 1 text" {
		t.Errorf("Region[0] = %+v", result.Regions[0])
	}
	if result.Regions[1].ID != "r2" || result.Regions[1].Text != "region 2 text" {
		t.Errorf("Region[1] = %+v", result.Regions[1])
	}
	if result.TotalMs != 1100 {
		t.Errorf("TotalMs = %d, want 1100", result.TotalMs)
	}
}

// createTestImage creates a minimal PNG file for testing.
func createTestImage(path string) error {
	// Minimal valid PNG: 1x1 pixel white image
	data := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, // IDAT chunk
		0x54, 0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00,
		0x00, 0x00, 0x02, 0x00, 0x01, 0xE2, 0x21, 0xBC,
		0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, // IEND chunk
		0x44, 0xAE, 0x42, 0x60, 0x82,
	}
	return os.WriteFile(path, data, 0600)
}
