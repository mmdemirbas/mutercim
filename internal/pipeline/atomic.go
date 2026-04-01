package pipeline

import "os"

// atomicWriteFile writes data to a file atomically via write-to-tmp + rename.
// On Windows, os.Rename fails if the destination exists, so we remove it first.
func atomicWriteFile(path string, data []byte) error {
	tmpPath := path + ".tmp"
	defer func() { _ = os.Remove(tmpPath) }() // clean up on failure; no-op after successful rename
	if err := os.WriteFile(tmpPath, data, 0600); err != nil { //nolint:gosec // G703: path comes from internal workspace logic, not user HTTP input
		return err
	}
	// Remove destination first for Windows compatibility (no-op if doesn't exist)
	_ = os.Remove(path)
	return os.Rename(tmpPath, path)
}
