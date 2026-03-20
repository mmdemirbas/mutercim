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

// KnowledgeDir returns the knowledge directory path.
func (w *Workspace) KnowledgeDir() string {
	return filepath.Join(w.Root, "knowledge")
}

// LogDir returns the log directory path.
func (w *Workspace) LogDir() string {
	return filepath.Join(w.Root, "log")
}

// LogPath returns the path to the log file.
func (w *Workspace) LogPath() string {
	return filepath.Join(w.Root, "log", "mutercim.log")
}

// MemoryDir returns the auto-extracted knowledge directory path.
func (w *Workspace) MemoryDir() string {
	return filepath.Join(w.Root, "memory")
}

// PagesDir returns the page images directory path.
func (w *Workspace) PagesDir() string {
	return filepath.Join(w.Root, "pages")
}

// ReadDir returns the OCR extraction directory path.
func (w *Workspace) ReadDir() string {
	return filepath.Join(w.Root, "read")
}

// SolveDir returns the enriched data directory path.
func (w *Workspace) SolveDir() string {
	return filepath.Join(w.Root, "solve")
}

// TranslateDir returns the translated data directory path.
func (w *Workspace) TranslateDir() string {
	return filepath.Join(w.Root, "translate")
}

// WriteDir returns the final output directory path.
func (w *Workspace) WriteDir() string {
	return filepath.Join(w.Root, "write")
}

// ProgressPath returns the path to the progress file.
func (w *Workspace) ProgressPath() string {
	return filepath.Join(w.Root, "progress.json")
}
