package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// Workspace represents a book workspace directory.
// Root is where mutercim.yaml, input/, and knowledge/ live.
// OutputDir is the base for all generated directories (cut/, read/, solve/,
// translate/, write/, log/, memory/). Defaults to Root.
type Workspace struct {
	Root      string
	OutputDir string
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
			return &Workspace{Root: dir, OutputDir: dir}, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, fmt.Errorf("no mutercim.yaml found (searched from %s to filesystem root)", startDir)
}

// outputBase returns the effective output base directory.
func (w *Workspace) outputBase() string {
	if w.OutputDir != "" {
		return w.OutputDir
	}
	return w.Root
}

// ConfigPath returns the path to the workspace config file.
func (w *Workspace) ConfigPath() string {
	return filepath.Join(w.Root, "mutercim.yaml")
}

// KnowledgeDir returns the knowledge directory path.
func (w *Workspace) KnowledgeDir() string {
	return filepath.Join(w.Root, "knowledge")
}

// LogPath returns the path to the log file.
func (w *Workspace) LogPath() string {
	return filepath.Join(w.outputBase(), "mutercim.log")
}

// MemoryDir returns the auto-extracted knowledge directory path.
func (w *Workspace) MemoryDir() string {
	return filepath.Join(w.outputBase(), "memory")
}

// CutDir returns the page images directory path.
func (w *Workspace) CutDir() string {
	return filepath.Join(w.outputBase(), "cut")
}

// LayoutDir returns the layout detection directory path.
func (w *Workspace) LayoutDir() string {
	return filepath.Join(w.outputBase(), "layout")
}

// ReadDir returns the OCR extraction directory path.
func (w *Workspace) ReadDir() string {
	return filepath.Join(w.outputBase(), "read")
}

// SolveDir returns the solved data directory path.
func (w *Workspace) SolveDir() string {
	return filepath.Join(w.outputBase(), "solve")
}

// TranslateDir returns the translated data directory path.
func (w *Workspace) TranslateDir() string {
	return filepath.Join(w.outputBase(), "translate")
}

// WriteDir returns the final output directory path.
func (w *Workspace) WriteDir() string {
	return filepath.Join(w.outputBase(), "write")
}
