package renderer

import (
	"context"
	"fmt"
	"os/exec"
)

// ConvertMarkdownToDocx converts a Markdown file to DOCX using pandoc.
func ConvertMarkdownToDocx(ctx context.Context, markdownPath, docxPath string) error {
	cmd := exec.CommandContext(ctx, "pandoc",
		markdownPath,
		"-o", docxPath,
		"--from=markdown",
		"--to=docx",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pandoc: %w: %s", err, output)
	}
	return nil
}

// CheckPandoc returns an error if pandoc is not available.
func CheckPandoc() error {
	if _, err := exec.LookPath("pandoc"); err != nil {
		return fmt.Errorf("pandoc not found in PATH (required for DOCX output)")
	}
	return nil
}
