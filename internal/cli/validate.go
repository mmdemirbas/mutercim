package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/model"
	"github.com/mmdemirbas/mutercim/internal/pipeline"
	"github.com/mmdemirbas/mutercim/internal/solver"
	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Run validation checks on read/solved data (read-only)",
		Long:  "Validates structural consistency, numbering sequences, and source resolution without making API calls or modifying files.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := workspace.Discover(".")
			if err != nil {
				return fmt.Errorf("workspace: %w", err)
			}

			configPath := cfgFile
			if configPath == "" {
				configPath = ws.ConfigPath()
			}
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}

			// Determine page range
			pageSpec := cfg.Pages
			if pages != "" {
				pageSpec = pages
			}
			var pagesToProcess []int
			if pageSpec != "" && pageSpec != "all" {
				ranges, err := model.ParsePageRanges(pageSpec)
				if err != nil {
					return fmt.Errorf("parse pages: %w", err)
				}
				pagesToProcess = model.ExpandPages(ranges)
			}

			// Discover inputs from read directory
			inputs, err := pipeline.DiscoverSubdirs(ws.ReadDir())
			if err != nil || len(inputs) == 0 {
				fmt.Println("No pages found. Run read first.")
				return nil
			}

			totalWarnings := 0
			totalPages := 0

			for _, stem := range inputs {
				fmt.Printf("\n%s:\n", stem)
				warnings, pages := validateInput(ws, stem, pagesToProcess)
				totalWarnings += warnings
				totalPages += pages
			}

			fmt.Printf("\nTotal: %d pages validated, %d warnings\n", totalPages, totalWarnings)
			return nil
		},
	}
}

func validateInput(ws *workspace.Workspace, stem string, pagesToProcess []int) (int, int) {
	readDir := filepath.Join(ws.ReadDir(), stem)

	pages, err := loadReadPages(readDir, pagesToProcess)
	if err != nil {
		fmt.Printf("  error loading pages: %v\n", err)
		return 0, 0
	}

	if len(pages) == 0 {
		fmt.Println("  no pages found")
		return 0, 0
	}

	totalWarnings := 0

	// Validate each page
	for _, page := range pages {
		v := solver.Validate(page)
		if v.Status != "ok" {
			fmt.Printf("  page %d: %s\n", page.PageNumber, v.Status)
			for _, w := range v.Warnings {
				fmt.Printf("    - %s\n", w)
			}
			totalWarnings += len(v.Warnings)
		}
	}

	// Check cross-page number continuity
	var allNumbers []int
	for _, page := range pages {
		for _, e := range page.Entries {
			if e.Number != nil && !e.IsContinuation {
				allNumbers = append(allNumbers, *e.Number)
			}
		}
	}

	if len(allNumbers) > 1 {
		for i := 1; i < len(allNumbers); i++ {
			if allNumbers[i] != allNumbers[i-1]+1 {
				fmt.Printf("  cross-page gap: entry %d → %d\n", allNumbers[i-1], allNumbers[i])
				totalWarnings++
			}
		}
	}

	if totalWarnings == 0 {
		fmt.Printf("  %d pages — ok\n", len(pages))
	}

	return totalWarnings, len(pages)
}

func loadReadPages(dir string, filter []int) ([]*model.ReadPage, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	filterSet := make(map[int]bool)
	for _, p := range filter {
		filterSet[p] = true
	}

	var pages []*model.ReadPage
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		var num int
		if _, err := fmt.Sscanf(e.Name(), "page_%03d.json", &num); err != nil {
			continue
		}
		if len(filterSet) > 0 && !filterSet[num] {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var page model.ReadPage
		if err := json.Unmarshal(data, &page); err != nil {
			continue
		}
		pages = append(pages, &page)
	}

	sort.Slice(pages, func(i, j int) bool {
		return pages[i].PageNumber < pages[j].PageNumber
	})
	return pages, nil
}
