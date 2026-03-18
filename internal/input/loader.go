package input

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

// PageImage represents a single page image file.
type PageImage struct {
	PageNumber int
	Path       string
}

// ListImages scans a directory for page image files and returns them sorted by page number.
// Supports naming patterns from pdftoppm (page-001.png) and manual naming (page_001.png, 001.png).
func ListImages(dir string) ([]PageImage, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read images dir %s: %w", dir, err)
	}

	var images []PageImage
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
			continue
		}
		pageNum := parsePageNumber(e.Name())
		if pageNum < 0 {
			continue
		}
		images = append(images, PageImage{
			PageNumber: pageNum,
			Path:       filepath.Join(dir, e.Name()),
		})
	}
	sort.Slice(images, func(i, j int) bool {
		return images[i].PageNumber < images[j].PageNumber
	})
	return images, nil
}

var pageNumRegex = regexp.MustCompile(`\d+`)

func parsePageNumber(filename string) int {
	name := filename[:len(filename)-len(filepath.Ext(filename))]
	matches := pageNumRegex.FindAllString(name, -1)
	if len(matches) == 0 {
		return -1
	}
	// Use the last number in the filename (handles "page-001", "page_001", etc.)
	num, err := strconv.Atoi(matches[len(matches)-1])
	if err != nil {
		return -1
	}
	return num
}

// LoadImage reads an image file and returns its bytes.
func LoadImage(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read image %s: %w", path, err)
	}
	return data, nil
}
