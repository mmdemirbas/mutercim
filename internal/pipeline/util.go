package pipeline

import (
	"fmt"
	"math"
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
		if _, err := fmt.Sscanf(e.Name(), "%d.json", &num); err != nil {
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

// dirHasEntries returns true if the directory exists and has at least one entry.
func dirHasEntries(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}

// pageFilename returns a zero-padded JSON filename for the given page number.
// The padding width is determined by the total number of pages:
//   - totalPages < 1000  -> 3 digits (e.g. 001.json)
//   - totalPages < 10000 -> 4 digits (e.g. 0001.json)
//   - else               -> 5 digits (e.g. 00001.json)
func pageFilename(pageNum, totalPages int) string {
	width := 3
	if totalPages >= 10000 {
		width = 5
	} else if totalPages >= 1000 {
		width = 4
	}
	return fmt.Sprintf("%0*d.json", width, pageNum)
}

// maxPageNum returns the maximum page number seen across a list of page numbers.
// Returns 0 if the slice is empty.
func maxPageNum(pages []int) int {
	m := 0
	for _, p := range pages {
		if p > m {
			m = p
		}
	}
	return m
}

// estimateTotalPages returns the max of two ints, used for deciding padding width.
func estimateTotalPages(a, b int) int {
	return int(math.Max(float64(a), float64(b)))
}
