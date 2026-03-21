package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mmdemirbas/mutercim/internal/config"
	"github.com/mmdemirbas/mutercim/internal/display"
	"github.com/mmdemirbas/mutercim/internal/model"

	"github.com/mmdemirbas/mutercim/internal/workspace"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show processing progress, validation warnings, and flagged issues",
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	ws, err := workspace.Discover(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "No workspace found. Run 'mutercim init' first.")
		return nil
	}

	configPath := cfgFile
	if configPath == "" {
		configPath = ws.ConfigPath()
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	applyOutputDir(ws, cfg)

	// Discover inputs
	inputs := discoverInputs(ws)
	if len(inputs) == 0 && len(cfg.Inputs) > 0 {
		for _, inp := range cfg.Inputs {
			inputs = append(inputs, filepath.Base(strings.TrimSuffix(inp.Path, filepath.Ext(inp.Path))))
		}
	}

	// Count total images
	totalImages := 0
	for _, stem := range inputs {
		totalImages += countFiles(filepath.Join(ws.PagesDir(), stem))
	}

	// Build input name from config
	inputName := ""
	if len(cfg.Inputs) > 0 {
		inputName = filepath.Base(cfg.Inputs[0].Path)
		if len(cfg.Inputs) > 1 {
			inputName += fmt.Sprintf(" (+%d more)", len(cfg.Inputs)-1)
		}
	}

	// Build phase rows
	rows := buildPhaseRows(ws, inputs, totalImages, cfg.Book.TargetLangs)

	// Collect warnings (errors are in the log now)
	var warnings []string

	// Run structural validation on read pages
	warnings = append(warnings, collectValidationWarnings(ws, inputs)...)

	// Log file info
	logPath := "log/mutercim.log"
	var logSize int64
	if info, err := os.Stat(ws.LogPath()); err == nil {
		logSize = info.Size()
	}

	// Build config summary fields
	layoutTool := cfg.Read.LayoutTool
	if layoutTool == "" {
		layoutTool = "ai-only"
	}

	var readModels []string
	for _, m := range cfg.Read.Models {
		readModels = append(readModels, m.Provider+"/"+m.Model)
	}
	var transModels []string
	for _, m := range cfg.Translate.Models {
		transModels = append(transModels, m.Provider+"/"+m.Model)
	}

	data := display.StatusData{
		BookTitle:   cfg.Book.Title,
		InputName:   inputName,
		InputPages:  totalImages,
		PageRange:   "",
		SourceLangs: cfg.Book.SourceLangs,
		TargetLangs: cfg.Book.TargetLangs,
		LayoutTool:  layoutTool,
		ReadModels:  readModels,
		TransModels: transModels,
		Phases:      rows,
		Warnings:    warnings,
		Errors:      nil,
		LogPath:     logPath,
		LogSize:     logSize,
	}

	colors := display.NewStatusColors(os.Stdout)
	display.RenderStatus(os.Stdout, data, colors)
	return nil
}

// buildPhaseRows creates the status table rows by counting files on disk.
func buildPhaseRows(ws *workspace.Workspace, inputs []string, totalImages int, targetLangs []string) []display.ProgressRow {
	var rows []display.ProgressRow

	// Count pages (image files per input stem)
	pagesCompleted := 0
	for _, stem := range inputs {
		pagesCompleted += countFiles(filepath.Join(ws.PagesDir(), stem))
	}
	pagesTotal := totalImages
	if pagesTotal == 0 {
		pagesTotal = pagesCompleted
	}
	rows = append(rows, display.ProgressRow{
		Phase: display.PhasePages, Completed: pagesCompleted, Total: pagesTotal,
		Done: pagesCompleted > 0 && pagesCompleted >= pagesTotal,
	})

	// Count read JSON files per input stem
	readCompleted := 0
	for _, stem := range inputs {
		readCompleted += countJSONFiles(filepath.Join(ws.ReadDir(), stem))
	}
	readTotal := totalImages
	rows = append(rows, display.ProgressRow{
		Phase: display.PhaseRead, Completed: readCompleted,
		Total: readTotal,
		Done:  readTotal > 0 && readCompleted >= readTotal,
	})

	// Count solve JSON files per input stem
	solveCompleted := 0
	for _, stem := range inputs {
		solveCompleted += countJSONFiles(filepath.Join(ws.SolveDir(), stem))
	}
	solveTotal := readCompleted
	rows = append(rows, display.ProgressRow{
		Phase: display.PhaseSolve, Completed: solveCompleted,
		Total: solveTotal,
		Done:  solveTotal > 0 && solveCompleted >= solveTotal,
	})

	// Translate rows per target language
	for _, lang := range targetLangs {
		transCompleted := 0
		for _, stem := range inputs {
			transCompleted += countJSONFiles(filepath.Join(ws.TranslateDir(), lang, stem))
		}
		transTotal := solveCompleted
		rows = append(rows, display.ProgressRow{
			Phase: display.PhaseTranslate, Completed: transCompleted,
			Total: transTotal, Lang: lang,
			Done: transTotal > 0 && transCompleted >= transTotal,
		})
	}

	// Write rows per target language
	for _, lang := range targetLangs {
		writeCompleted := 0
		writeDir := filepath.Join(ws.WriteDir(), lang)
		if dirHasFiles(writeDir) {
			writeCompleted = 1
		}
		writeTotal := 1
		// Only show write as having a total if translate produced output
		transCompleted := 0
		for _, stem := range inputs {
			transCompleted += countJSONFiles(filepath.Join(ws.TranslateDir(), lang, stem))
		}
		if transCompleted == 0 {
			writeTotal = 0
		}
		rows = append(rows, display.ProgressRow{
			Phase: display.PhaseWrite, Completed: writeCompleted,
			Total: writeTotal, Lang: lang,
			Done: writeCompleted > 0,
		})
	}

	return rows
}

// countJSONFiles counts .json files in a directory.
func countJSONFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			count++
		}
	}
	return count
}

// dirHasFiles returns true if the directory exists and contains at least one non-directory entry.
func dirHasFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			return true
		}
	}
	return false
}

// discoverInputs finds input stems by scanning workspace subdirectories.
func discoverInputs(ws *workspace.Workspace) []string {
	seen := make(map[string]bool)
	for _, dir := range []string{ws.PagesDir(), ws.ReadDir(), ws.SolveDir()} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				seen[e.Name()] = true
			}
		}
	}
	// Also check translated dir (has per-lang subdirs)
	entries, err := os.ReadDir(ws.TranslateDir())
	if err == nil {
		for _, langDir := range entries {
			if !langDir.IsDir() {
				continue
			}
			subEntries, err := os.ReadDir(filepath.Join(ws.TranslateDir(), langDir.Name()))
			if err != nil {
				continue
			}
			for _, e := range subEntries {
				if e.IsDir() {
					seen[e.Name()] = true
				}
			}
		}
	}

	stems := make([]string, 0, len(seen))
	for name := range seen {
		stems = append(stems, name)
	}
	sort.Strings(stems)
	return stems
}

// countFiles counts non-directory entries in a directory.
func countFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	return count
}

// collectValidationWarnings runs structural validation on read pages and returns warnings.
func collectValidationWarnings(ws *workspace.Workspace, inputs []string) []string {
	var warnings []string
	for _, stem := range inputs {
		readDir := filepath.Join(ws.ReadDir(), stem)
		pages, err := loadRegionPages(readDir)
		if err != nil || len(pages) == 0 {
			continue
		}

		// Per-page validation: check for empty text in non-separator regions
		for _, page := range pages {
			for _, r := range page.Regions {
				if r.Type != model.RegionTypeSeparator && r.Type != model.RegionTypeImage && r.Text == "" {
					warnings = append(warnings, fmt.Sprintf("%s page %d: region %s (%s) has empty text", stem, page.PageNumber, r.ID, r.Type))
				}
			}
			// Check reading order references
			regionIDs := make(map[string]bool)
			for _, r := range page.Regions {
				regionIDs[r.ID] = true
			}
			for _, id := range page.ReadingOrder {
				if !regionIDs[id] {
					warnings = append(warnings, fmt.Sprintf("%s page %d: reading_order references unknown region %s", stem, page.PageNumber, id))
				}
			}
		}
	}
	return warnings
}

// loadRegionPages loads RegionPage JSON files from a directory, sorted by page number.
func loadRegionPages(dir string) ([]*model.RegionPage, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var pages []*model.RegionPage
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		var num int
		if _, err := fmt.Sscanf(e.Name(), "%03d.json", &num); err != nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var page model.RegionPage
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
