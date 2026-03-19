package solver

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mmdemirbas/mutercim/internal/model"
	"gopkg.in/yaml.v3"
)

// stagedSources is the YAML format for auto-staged source entries.
type stagedSources struct {
	Entries []stagedSource `yaml:"entries"`
}

type stagedSource struct {
	Code   string `yaml:"code"`
	NameAr string `yaml:"name_ar"`
	NameTr string `yaml:"name_tr"`
}

// StageFromReferenceTable extracts source-like entries from a reference_table page
// and writes them to the staging area. Only stages if the page has entries that
// look like abbreviation key-value pairs.
func StageFromReferenceTable(page *model.ReadPage, stagedDir string) error {
	if page.SectionType != "reference_table" {
		return nil
	}
	if len(page.Entries) == 0 {
		return nil
	}

	// Extract entries as source candidates
	var sources []stagedSource
	for _, e := range page.Entries {
		if e.ArabicText == "" {
			continue
		}
		sources = append(sources, stagedSource{
			Code:   e.ArabicText,
			NameAr: e.ArabicText,
		})
	}

	if len(sources) == 0 {
		return nil
	}

	staged := stagedSources{Entries: sources}
	data, err := yaml.Marshal(staged)
	if err != nil {
		return fmt.Errorf("marshal staged sources: %w", err)
	}

	if err := os.MkdirAll(stagedDir, 0755); err != nil {
		return fmt.Errorf("create staged dir: %w", err)
	}

	filename := fmt.Sprintf("sources_page_%03d.yaml", page.PageNumber)
	tmpPath := filepath.Join(stagedDir, filename+".tmp")
	finalPath := filepath.Join(stagedDir, filename)

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write staged file: %w", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("rename staged file: %w", err)
	}

	return nil
}
