package pipeline

// DiscoverSubdirs returns sorted names of subdirectories in dir.
// It is an exported wrapper around the internal discoverSubdirs for use by other packages.
func DiscoverSubdirs(dir string) ([]string, error) {
	return discoverSubdirs(dir)
}
