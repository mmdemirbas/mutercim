package input

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mmdemirbas/mutercim/internal/docker"
)

// DefaultPopplerImage is the Docker image used for PDF-to-image conversion.
const DefaultPopplerImage = "mutercim/poppler:latest"

// ConvertPDFToImages converts a PDF file to PNG images using pdftoppm in Docker.
// Output files are named NNN.png in outputDir (zero-padded to match page count).
// dockerfileDir is the path to docker/poppler/ for auto-building the image.
func ConvertPDFToImages(ctx context.Context, pdfPath, outputDir string, dpi, firstPage, lastPage int, dockerfileDir string) error {
	if err := docker.EnsureImage(ctx, DefaultPopplerImage, dockerfileDir); err != nil {
		return fmt.Errorf("ensure poppler image: %w", err)
	}

	absPDF, err := filepath.Abs(pdfPath)
	if err != nil {
		return fmt.Errorf("resolve pdf path: %w", err)
	}
	absOut, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("resolve output dir: %w", err)
	}

	pdfDir := filepath.Dir(absPDF)
	pdfBase := filepath.Base(absPDF)

	args := []string{
		"run", "--rm",
		"-v", filepath.ToSlash(pdfDir) + ":/input:ro",
		"-v", filepath.ToSlash(absOut) + ":/output",
		DefaultPopplerImage,
		"pdftoppm",
		"-png", "-r", strconv.Itoa(dpi),
	}
	if firstPage > 0 {
		args = append(args, "-f", strconv.Itoa(firstPage))
	}
	if lastPage > 0 {
		args = append(args, "-l", strconv.Itoa(lastPage))
	}
	args = append(args, "/input/"+pdfBase, "/output/page")

	output, err := docker.Run(ctx, args...)
	if err != nil {
		return fmt.Errorf("pdftoppm (docker): %w: %s", err, output)
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
