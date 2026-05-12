package pipeline

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// readFile is a thin wrapper to keep test bodies terse.
func readFile(p string) ([]byte, error) { return os.ReadFile(p) } //nolint:gosec // G304: test-only path
// osStat ditto.
func osStat(p string) (os.FileInfo, error) { return os.Stat(p) }

// TestWriteTypstFormat verifies the "typst" write format emits a .typ
// file via the unified format dispatch. Distinct from the typst
// renderer's own tests — those verify the .typ content; this verifies
// the pipeline wiring writes it to the right path.
func TestWriteTypstFormat(t *testing.T) {
	ws, cfg := setupWriteWorkspace(t)
	cfg.Write.Formats = []string{"typst"}

	if err := Write(t.Context(), WriteOptions{
		Workspace: ws,
		Config:    cfg,
		Force:     true,
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	outPath := filepath.Join(ws.WriteDir(), "tr", "TestBook.typ")
	data, err := readFile(outPath)
	if err != nil {
		t.Fatalf("expected .typ at %s: %v", outPath, err)
	}
	if !strings.Contains(string(data), "Elif Harfi") {
		t.Errorf("output should contain translated header, got:\n%s", data)
	}
	if !strings.Contains(string(data), "Müjdelenin!") {
		t.Errorf("output should contain translated entry, got:\n%s", data)
	}
}

// TestWritePDF_TypstEngineProducesPDF verifies that write.pdf_engine =
// "typst" routes the "pdf" format through the Typst compiler. Skips
// when typst is not on PATH so CI without typst still passes.
func TestWritePDF_TypstEngineProducesPDF(t *testing.T) {
	if _, err := exec.LookPath("typst"); err != nil {
		t.Skip("typst not installed")
	}
	ws, cfg := setupWriteWorkspace(t)
	cfg.Write.Formats = []string{"pdf"}
	cfg.Write.PdfEngine = "typst"

	ctx, cancel := context.WithTimeout(t.Context(), 60*1e9) // 60s
	defer cancel()
	if err := Write(ctx, WriteOptions{
		Workspace: ws,
		Config:    cfg,
		Force:     true,
	}); err != nil {
		t.Fatalf("Write with typst engine: %v", err)
	}

	pdfPath := filepath.Join(ws.WriteDir(), "tr", "TestBook.pdf")
	info, err := osStat(pdfPath)
	if err != nil {
		t.Fatalf("expected PDF at %s: %v", pdfPath, err)
	}
	if info.Size() < 1024 {
		t.Errorf("PDF size %d bytes is suspiciously small", info.Size())
	}
	// The .typ source should also be present (for re-rendering).
	if _, err := osStat(filepath.Join(ws.WriteDir(), "tr", "TestBook.typ")); err != nil {
		t.Errorf("expected .typ source alongside PDF: %v", err)
	}
}
