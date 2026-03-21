package pipeline

import "os"

// atomicWriteFile writes data to a file atomically via write-to-tmp + rename.
// On Windows, os.Rename fails if the destination exists, so we remove it first.
func atomicWriteFile(path string, data []byte) error {
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	// Remove destination first for Windows compatibility (no-op if doesn't exist)
	os.Remove(path)
	return os.Rename(tmpPath, path)
}
