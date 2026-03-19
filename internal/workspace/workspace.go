package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// Workspace represents a book workspace directory.
type Workspace struct {
	Root string
}

// Discover finds the workspace root by looking for mutercim.yaml
// starting from the given directory and walking up.
func Discover(startDir string) (*Workspace, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	for {
		configPath := filepath.Join(dir, "mutercim.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return &Workspace{Root: dir}, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, fmt.Errorf("no mutercim.yaml found (searched from %s to filesystem root)", startDir)
}

// ConfigPath returns the path to the workspace config file.
func (w *Workspace) ConfigPath() string {
	return filepath.Join(w.Root, "mutercim.yaml")
}

// InputDir returns the input directory path.
func (w *Workspace) InputDir() string {
	return filepath.Join(w.Root, "input")
}

// OutputDir returns the output directory path.
func (w *Workspace) OutputDir() string {
	return filepath.Join(w.Root, "output")
}

// CacheDir returns the cache directory path.
func (w *Workspace) CacheDir() string {
	return filepath.Join(w.Root, "cache")
}

// KnowledgeDir returns the knowledge directory path.
func (w *Workspace) KnowledgeDir() string {
	return filepath.Join(w.Root, "knowledge")
}

// ProgressPath returns the path to the progress file.
func (w *Workspace) ProgressPath() string {
	return filepath.Join(w.Root, "progress.json")
}

// ReportsDir returns the reports directory path.
func (w *Workspace) ReportsDir() string {
	return filepath.Join(w.Root, "reports")
}

// StagedDir returns the staged knowledge directory path.
func (w *Workspace) StagedDir() string {
	return filepath.Join(w.Root, "cache", "staged")
}

// ReadDir returns the read cache directory path.
func (w *Workspace) ReadDir() string {
	return filepath.Join(w.Root, "cache", "read")
}

// SolvedDir returns the solved cache directory path.
func (w *Workspace) SolvedDir() string {
	return filepath.Join(w.Root, "cache", "solved")
}

// TranslatedDir returns the translated cache directory path.
func (w *Workspace) TranslatedDir() string {
	return filepath.Join(w.Root, "cache", "translated")
}

// ImagesDir returns the images cache directory path.
func (w *Workspace) ImagesDir() string {
	return filepath.Join(w.Root, "cache", "images")
}
