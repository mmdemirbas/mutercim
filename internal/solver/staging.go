package solver

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mmdemirbas/mutercim/internal/model"
	"gopkg.in/yaml.v3"
)

// memorySources is the YAML format for auto-extracted source entries.
type memorySources struct {
	Entries []memorySource `yaml:"entries"`
}

type memorySource struct {
	Code   string `yaml:"code"`
	NameAr string `yaml:"name_ar"`
	NameTr string `yaml:"name_tr"`
}

// ExtractToMemory extracts source-like entries from a reference_table page
// and writes them to the memory directory. Only writes if the page has entries
// that look like abbreviation key-value pairs.
func ExtractToMemory(page *model.ReadPage, memoryDir string) error {
	if page.SectionType != "reference_table" {
		return nil
	}
	if len(page.Entries) == 0 {
		return nil
	}

	// Extract entries as source candidates
	var sources []memorySource
	for _, e := range page.Entries {
		if e.ArabicText == "" {
			continue
		}
		sources = append(sources, memorySource{
			Code:   e.ArabicText,
			NameAr: e.ArabicText,
		})
	}

	if len(sources) == 0 {
		return nil
	}

	mem := memorySources{Entries: sources}
	data, err := yaml.Marshal(mem)
	if err != nil {
		return fmt.Errorf("marshal memory sources: %w", err)
	}

	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}

	filename := fmt.Sprintf("sources_%03d.yaml", page.PageNumber)
	tmpPath := filepath.Join(memoryDir, filename+".tmp")
	finalPath := filepath.Join(memoryDir, filename)

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write memory file: %w", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("rename memory file: %w", err)
	}

	return nil
}
