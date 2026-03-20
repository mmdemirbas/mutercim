package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// PhaseResult contains the outcome counts from running a pipeline phase.
type PhaseResult struct {
	Completed int
	Failed    int
	Skipped   int
}

// IsEmpty returns true if no pages were completed or skipped (nothing useful produced).
func (r PhaseResult) IsEmpty() bool {
	return r.Completed == 0 && r.Skipped == 0
}

// pageFile represents a JSON page file with its parsed page number.
type pageFile struct {
	pageNum int
	path    string
}

// listPageFiles lists page JSON files in a directory, sorted by page number.
func listPageFiles(dir string) ([]pageFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var pages []pageFile
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		var num int
		if _, err := fmt.Sscanf(e.Name(), "%03d.json", &num); err != nil {
			continue
		}
		pages = append(pages, pageFile{pageNum: num, path: filepath.Join(dir, e.Name())})
	}

	sort.Slice(pages, func(i, j int) bool { return pages[i].pageNum < pages[j].pageNum })
	return pages, nil
}

// filterPages filters page files to only include those in the wanted list.
func filterPages(pages []pageFile, wanted []int) []pageFile {
	set := make(map[int]bool)
	for _, p := range wanted {
		set[p] = true
	}
	var filtered []pageFile
	for _, pf := range pages {
		if set[pf.pageNum] {
			filtered = append(filtered, pf)
		}
	}
	return filtered
}

// discoverSubdirs returns sorted names of subdirectories in dir.
func discoverSubdirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var stems []string
	for _, e := range entries {
		if e.IsDir() {
			stems = append(stems, e.Name())
		}
	}
	sort.Strings(stems)
	return stems, nil
}

// fileStem returns the filename without extension.
// e.g. "./input/Anfas1.pdf" -> "Anfas1"
func fileStem(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if ext == "" {
		return base
	}
	return base[:len(base)-len(ext)]
}

// fileExists returns true if the path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// containsInt returns true if s contains v.
func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
