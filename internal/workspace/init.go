package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InitOptions configures workspace initialization.
type InitOptions struct {
	Dir         string
	Title       string
	Author      string
	SourceLangs string // comma-separated, e.g. "ar" or "ar,fa"
	TargetLangs string // comma-separated, e.g. "tr" or "tr,en"
}

// Init creates a new workspace directory structure and config file.
func Init(opts InitOptions) (*Workspace, error) {
	root := opts.Dir
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}

	// Check if workspace already exists
	configPath := filepath.Join(root, "mutercim.yaml")
	if _, err := os.Stat(configPath); err == nil {
		return nil, fmt.Errorf("workspace already initialized (mutercim.yaml exists in %s)", root)
	}

	// Set defaults
	if opts.SourceLangs == "" {
		opts.SourceLangs = "ar"
	}
	if opts.TargetLangs == "" {
		opts.TargetLangs = "tr"
	}

	sourceLangs := splitLangs(opts.SourceLangs)
	targetLangs := splitLangs(opts.TargetLangs)

	// Create directory structure
	dirs := []string{
		"input",
		"knowledge",
		"log",
		"memory",
		"pages",
		"read",
		"solve",
		"translate",
		"write",
	}

	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			return nil, fmt.Errorf("create directory %s: %w", d, err)
		}
	}

	// Write config file
	config := generateConfig(opts, sourceLangs, targetLangs)
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	// Write empty progress.json
	progressPath := filepath.Join(root, "progress.json")
	if err := os.WriteFile(progressPath, []byte("{}\n"), 0644); err != nil {
		return nil, fmt.Errorf("write progress: %w", err)
	}

	return &Workspace{Root: root}, nil
}

func generateConfig(opts InitOptions, sourceLangs, targetLangs []string) string {
	title := opts.Title
	if title == "" {
		title = "Untitled Book"
	}
	author := opts.Author
	if author == "" {
		author = "Unknown Author"
	}

	return fmt.Sprintf(`book:
  title: %q
  author: %q
  source_langs: [%s]
  target_langs: [%s]

# Input files or directories (relative to workspace root)
# Simple:       inputs: [./input/book.pdf]
# Per-input pages:
#   inputs:
#     - path: ./input/vol1.pdf
#       pages: "1-50"
#     - path: ./input/vol2.pdf
#       pages: "10-20"
inputs: [./input]

# Page range to process: "1-50", "1,5,10-20", "all"
# pages: all

dpi: 300

# Sections define the book's internal layout structure.
# Pages not covered by any section default to type: auto.
# If no sections are defined at all, entire book is type: auto.
#
# Section types:
#   skip              - Don't process these pages
#   prose             - Continuous paragraphs (introductions, prefaces)
#   scholarly_entries  - Numbered entries + footnotes with source codes (hadith, athar)
#   reference_table   - Key-value reference data (abbreviation keys → auto-staged)
#   toc               - Table of contents
#   index             - Alphabetical index
#   auto              - AI detects layout (default for unconfigured pages)
sections: []

# Model configuration
read:
  provider: gemini
  model: gemini-2.0-flash
  concurrency: 1

translate:
  provider: gemini
  model: gemini-2.0-flash
  context_window: 2

write:
  formats: [md, pdf]
  expand_sources: true
  latex_docker_image: mutercim/xelatex:latest

# Knowledge paths (relative to workspace root)
knowledge:
  dir: ./knowledge

# Processing behavior
retry:
  max_attempts: 3
  backoff_seconds: 2

rate_limit:
  requests_per_minute: 14
`, title, author, strings.Join(sourceLangs, ", "), strings.Join(targetLangs, ", "))
}

// splitLangs splits a comma-separated language string into a slice.
func splitLangs(s string) []string {
	var langs []string
	for _, l := range strings.Split(s, ",") {
		l = strings.TrimSpace(l)
		if l != "" {
			langs = append(langs, l)
		}
	}
	return langs
}
