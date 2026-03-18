package input

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
)

// ConvertPDFToImages converts a PDF file to PNG images using pdftoppm.
// Output files are named page-NNN.png in outputDir.
func ConvertPDFToImages(ctx context.Context, pdfPath, outputDir string, dpi, firstPage, lastPage int) error {
	args := []string{"-png", "-r", strconv.Itoa(dpi)}
	if firstPage > 0 {
		args = append(args, "-f", strconv.Itoa(firstPage))
	}
	if lastPage > 0 {
		args = append(args, "-l", strconv.Itoa(lastPage))
	}
	args = append(args, pdfPath, filepath.Join(outputDir, "page"))

	cmd := exec.CommandContext(ctx, "pdftoppm", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pdftoppm: %w: %s", err, output)
	}
	return nil
}

// CheckPdftoppm returns an error if pdftoppm is not available.
func CheckPdftoppm() error {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		return fmt.Errorf("pdftoppm not found in PATH (install: brew install poppler / apt install poppler-utils)")
	}
	return nil
}
