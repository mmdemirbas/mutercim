package input

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// ConvertPDFToImages converts a PDF file to PNG images using pdftoppm.
// Output files are named NNN.png in outputDir (zero-padded to match page count).
func ConvertPDFToImages(ctx context.Context, pdfPath, outputDir string, dpi, firstPage, lastPage int) error {
	args := []string{"-png", "-r", strconv.Itoa(dpi)}
	if firstPage > 0 {
		args = append(args, "-f", strconv.Itoa(firstPage))
	}
	if lastPage > 0 {
		args = append(args, "-l", strconv.Itoa(lastPage))
	}
	// pdftoppm requires a prefix; we use a temporary one then rename
	prefix := filepath.Join(outputDir, "page")
	args = append(args, pdfPath, prefix)

	cmd := exec.CommandContext(ctx, "pdftoppm", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pdftoppm: %w: %s", err, output)
	}

	// Rename page-NNN.png → NNN.png to match the convention used by all subsequent phases
	return renamePdftoppmOutput(outputDir)
}

// renamePdftoppmOutput renames pdftoppm output files from page-NNN.ext to NNN.ext.
func renamePdftoppmOutput(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// pdftoppm produces "page-NNN.png" — strip the "page-" prefix
		if len(name) > 5 && name[:5] == "page-" {
			newName := name[5:] // "001.png"
			oldPath := filepath.Join(dir, name)
			newPath := filepath.Join(dir, newName)
			if err := os.Rename(oldPath, newPath); err != nil {
				return fmt.Errorf("rename %s → %s: %w", name, newName, err)
			}
		}
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
