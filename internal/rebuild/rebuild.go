package rebuild

import (
	"os"
	"path/filepath"
	"time"
)

// NeedsRebuild returns true if the output file should be regenerated.
// Returns true if the output doesn't exist or any input is newer than the output.
// Inputs can be files or directories. For directories, both the directory's own
// mtime and all files within are considered (directory mtime captures additions/deletions).
// Errors reading inputs are treated as "needs rebuild" (safe side).
func NeedsRebuild(output string, inputs ...string) bool {
	outInfo, err := os.Stat(output)
	if err != nil {
		return true // output missing → rebuild
	}
	outMtime := outInfo.ModTime()

	inputMtime, err := NewestMtime(inputs...)
	if err != nil {
		return true // can't determine input state → rebuild to be safe
	}

	return inputMtime.After(outMtime)
}

// NewestMtime returns the most recent mtime across all given paths.
// For files: uses the file's mtime directly.
// For directories: uses the maximum of the directory's own mtime and all
// files within it (recursively). The directory's own mtime captures file
// additions and deletions.
// Returns zero time and nil if no paths are provided.
// Returns an error if any path cannot be stat'd.
func NewestMtime(paths ...string) (time.Time, error) {
	var newest time.Time

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue // skip non-existent paths
			}
			return time.Time{}, err
		}

		if !info.IsDir() {
			if info.ModTime().After(newest) {
				newest = info.ModTime()
			}
			continue
		}

		// Directory: include its own mtime (catches additions/deletions)
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}

		// Walk all files within
		err = filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			fi, err := d.Info()
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if fi.ModTime().After(newest) {
				newest = fi.ModTime()
			}
			return nil
		})
		if err != nil {
			return time.Time{}, err
		}
	}

	return newest, nil
}
