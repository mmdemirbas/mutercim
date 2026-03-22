package renderer

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mmdemirbas/mutercim/internal/docker"
)

// DefaultPandocImage is the Docker image used for DOCX conversion.
const DefaultPandocImage = "mutercim/pandoc:latest"

// ConvertMarkdownToDocx converts a Markdown file to DOCX using pandoc in Docker.
// dockerfileDir is the path to docker/pandoc/ for auto-building the image.
func ConvertMarkdownToDocx(ctx context.Context, markdownPath, docxPath, dockerfileDir string) error {
	if err := docker.EnsureImage(ctx, DefaultPandocImage, dockerfileDir); err != nil {
		return fmt.Errorf("ensure pandoc image: %w", err)
	}

	absDir, err := filepath.Abs(filepath.Dir(markdownPath))
	if err != nil {
		return fmt.Errorf("resolve markdown dir: %w", err)
	}
	mdBase := filepath.Base(markdownPath)
	docxBase := filepath.Base(docxPath)

	output, err := docker.Run(ctx,
		"run", "--rm",
		"-v", absDir+":/data",
		DefaultPandocImage,
		"/data/"+mdBase,
		"-o", "/data/"+docxBase,
		"--from=markdown",
		"--to=docx",
	)
	if err != nil {
		return fmt.Errorf("pandoc (docker): %w: %s", err, output)
	}
	return nil
}
