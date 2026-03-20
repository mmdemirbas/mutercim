package knowledge

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load loads knowledge from all three layers and merges them.
// Later layers override earlier ones on key conflicts.
func Load(workspaceKnowledgeDir, memoryDir string) (*Knowledge, error) {
	k := &Knowledge{}

	// Layer 1: Embedded defaults
	if err := loadFromFS(k, embeddedFS, "defaults"); err != nil {
		return nil, fmt.Errorf("load embedded knowledge: %w", err)
	}

	// Layer 2: Workspace knowledge directory
	if workspaceKnowledgeDir != "" {
		if err := loadFromDir(k, workspaceKnowledgeDir); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("load workspace knowledge: %w", err)
		}
	}

	// Layer 3: Memory (auto-extracted by solve phase)
	if memoryDir != "" {
		if err := loadFromDir(k, memoryDir); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("load memory knowledge: %w", err)
		}
	}

	return k, nil
}

func loadFromFS(k *Knowledge, fsys fs.FS, root string) error {
	dirEntries, err := fs.ReadDir(fsys, root)
	if err != nil {
		return nil // directory might not exist
	}
	for _, de := range dirEntries {
		if de.IsDir() || !isYAMLFile(de.Name()) {
			continue
		}
		data, err := fs.ReadFile(fsys, filepath.Join(root, de.Name()))
		if err != nil {
			continue
		}
		if err := mergeRawEntries(k, data); err != nil {
			return fmt.Errorf("parse %s: %w", de.Name(), err)
		}
	}
	return nil
}

func loadFromDir(k *Knowledge, dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return loadFromFS(k, os.DirFS(dir), ".")
}

func isYAMLFile(name string) bool {
	return strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")
}

// rawFile represents the on-disk YAML format.
type rawFile struct {
	Entries []map[string]interface{} `yaml:"entries"`
}

func mergeRawEntries(k *Knowledge, data []byte) error {
	var f rawFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return err
	}
	for _, raw := range f.Entries {
		entry, err := parseEntry(raw)
		if err != nil {
			return err
		}
		mergeEntry(k, entry)
	}
	return nil
}

func parseEntry(raw map[string]interface{}) (Entry, error) {
	entry := Entry{
		Forms: make(map[string][]string),
	}
	for key, val := range raw {
		if key == "note" {
			s, ok := val.(string)
			if !ok {
				return Entry{}, fmt.Errorf("note must be a string")
			}
			entry.Note = s
			continue
		}
		// Everything else is treated as a language code
		forms, err := normalizeValue(val)
		if err != nil {
			return Entry{}, fmt.Errorf("language %q: %w", key, err)
		}
		entry.Forms[key] = forms
	}
	return entry, nil
}

func normalizeValue(val interface{}) ([]string, error) {
	switch v := val.(type) {
	case string:
		return []string{v}, nil
	case []interface{}:
		var result []string
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("list items must be strings")
			}
			result = append(result, s)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("value must be string or list of strings")
	}
}

// mergeEntry adds or overrides an entry in the knowledge base.
// The merge key is the canonical form (first element) of the alphabetically
// first language code.
func mergeEntry(k *Knowledge, entry Entry) {
	key := mergeKey(entry)
	for i, existing := range k.Entries {
		if mergeKey(existing) == key {
			k.Entries[i] = entry
			return
		}
	}
	k.Entries = append(k.Entries, entry)
}

func mergeKey(e Entry) string {
	var langs []string
	for lang := range e.Forms {
		langs = append(langs, lang)
	}
	sort.Strings(langs)
	if len(langs) == 0 {
		return ""
	}
	forms := e.Forms[langs[0]]
	if len(forms) == 0 {
		return ""
	}
	return langs[0] + ":" + forms[0]
}
