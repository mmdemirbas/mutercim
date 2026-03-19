package knowledge

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Load loads knowledge from all three layers and merges them.
// Later layers override earlier ones on key conflicts.
func Load(workspaceKnowledgeDir, stagedDir string) (*Knowledge, error) {
	k := &Knowledge{}

	// Layer 1: Embedded defaults
	if err := loadFromFS(k, embeddedFS, "defaults", "embedded"); err != nil {
		return nil, fmt.Errorf("load embedded knowledge: %w", err)
	}

	// Layer 2: Workspace knowledge directory
	if workspaceKnowledgeDir != "" {
		if err := loadFromDir(k, workspaceKnowledgeDir, "workspace"); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("load workspace knowledge: %w", err)
		}
	}

	// Layer 3: Staged knowledge
	if stagedDir != "" {
		if err := loadFromDir(k, stagedDir, "staged"); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("load staged knowledge: %w", err)
		}
	}

	return k, nil
}

func loadFromFS(k *Knowledge, fsys fs.FS, root, layer string) error {
	if err := loadYAML(k, fsys, filepath.Join(root, "phrases.yaml"), layer, loadHonorifics); err != nil {
		return err
	}
	if err := loadYAML(k, fsys, filepath.Join(root, "sources.yaml"), layer, loadSources); err != nil {
		return err
	}
	if err := loadYAML(k, fsys, filepath.Join(root, "people.yaml"), layer, loadPeople); err != nil {
		return err
	}
	if err := loadYAML(k, fsys, filepath.Join(root, "terms.yaml"), layer, loadTerminology); err != nil {
		return err
	}
	if err := loadYAML(k, fsys, filepath.Join(root, "places.yaml"), layer, loadPlaces); err != nil {
		return err
	}
	return nil
}

func loadFromDir(k *Knowledge, dir, layer string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return loadFromFS(k, os.DirFS(dir), ".", layer)
}

type mergeFunc func(k *Knowledge, data []byte, layer string) error

func loadYAML(k *Knowledge, fsys fs.FS, path, layer string, merge mergeFunc) error {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		// File not found is not an error — not all layers have all files
		return nil
	}
	return merge(k, data, layer)
}

func loadHonorifics(k *Knowledge, data []byte, layer string) error {
	var f entriesFile[Honorific]
	if err := yaml.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("parse honorifics: %w", err)
	}
	mergeByKey(&k.Honorifics, f.Entries, func(h Honorific) string { return h.Arabic })
	return nil
}

func loadSources(k *Knowledge, data []byte, layer string) error {
	var f entriesFile[Source]
	if err := yaml.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("parse sources: %w", err)
	}
	for i := range f.Entries {
		f.Entries[i].Layer = layer
	}
	mergeByKey(&k.Sources, f.Entries, func(s Source) string { return s.Code })
	return nil
}

func loadPeople(k *Knowledge, data []byte, layer string) error {
	var f entriesFile[Person]
	if err := yaml.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("parse people: %w", err)
	}
	mergeByKey(&k.People, f.Entries, func(p Person) string { return p.Arabic })
	return nil
}

func loadTerminology(k *Knowledge, data []byte, layer string) error {
	var f entriesFile[Term]
	if err := yaml.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("parse terminology: %w", err)
	}
	mergeByKey(&k.Terminology, f.Entries, func(t Term) string { return t.Arabic })
	return nil
}

func loadPlaces(k *Knowledge, data []byte, layer string) error {
	var f entriesFile[Place]
	if err := yaml.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("parse places: %w", err)
	}
	mergeByKey(&k.Places, f.Entries, func(p Place) string { return p.Arabic })
	return nil
}

// mergeByKey merges entries into the target slice, overriding on key conflict.
func mergeByKey[T any](target *[]T, entries []T, key func(T) string) {
	existing := make(map[string]int)
	for i, item := range *target {
		existing[key(item)] = i
	}
	for _, item := range entries {
		k := key(item)
		if idx, ok := existing[k]; ok {
			(*target)[idx] = item
		} else {
			*target = append(*target, item)
			existing[k] = len(*target) - 1
		}
	}
}
