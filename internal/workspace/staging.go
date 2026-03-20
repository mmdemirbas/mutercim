package workspace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ListStagedFiles returns the names of all YAML files in the staging area.
func (w *Workspace) ListStagedFiles() ([]string, error) {
	entries, err := os.ReadDir(w.MemoryDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read staged dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, e.Name())
		}
	}
	return files, nil
}

// PromoteStagedFile copies a staged file into the persistent knowledge directory.
// If replace is true, it overwrites any existing file. Otherwise it's a simple copy.
func (w *Workspace) PromoteStagedFile(filename string, replace bool) error {
	srcPath := filepath.Join(w.MemoryDir(), filename)
	dstPath := filepath.Join(w.KnowledgeDir(), filename)

	if _, err := os.Stat(srcPath); err != nil {
		return fmt.Errorf("staged file %q not found: %w", filename, err)
	}

	if !replace {
		if _, err := os.Stat(dstPath); err == nil {
			return fmt.Errorf("knowledge file %q already exists (use --replace to overwrite)", filename)
		}
	}

	if err := os.MkdirAll(w.KnowledgeDir(), 0755); err != nil {
		return fmt.Errorf("create knowledge dir: %w", err)
	}

	return copyFile(srcPath, dstPath)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmpPath := dst + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err := out.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, dst)
}
