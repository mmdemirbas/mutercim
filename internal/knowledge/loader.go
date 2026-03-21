package knowledge

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load loads knowledge from the given paths and memory directory, merging them.
// Each path can be a directory (all .yaml/.yml files are loaded) or a single YAML file.
// Later entries override earlier ones on key conflicts.
// Schema mismatches in individual files are logged as warnings and skipped.
func Load(knowledgePaths []string, memoryDir string) (*Knowledge, error) {
	k := &Knowledge{}

	// Layer 1: Knowledge paths (files and directories)
	for _, p := range knowledgePaths {
		if p == "" {
			continue
		}
		if err := loadPath(k, p); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("load knowledge %q: %w", p, err)
		}
	}

	// Layer 2: Memory (auto-extracted by solve phase)
	if memoryDir != "" {
		if err := loadPath(k, memoryDir); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("load memory knowledge: %w", err)
		}
	}

	return k, nil
}

// loadPath loads knowledge from a single path — either a directory or a YAML file.
func loadPath(k *Knowledge, p string) error {
	info, err := os.Stat(p)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return loadFromDir(k, p)
	}
	if isYAMLFile(info.Name()) {
		return loadFile(k, p)
	}
	return nil
}

func loadFromDir(k *Knowledge, dir string) error {
	return loadFromFS(k, os.DirFS(dir), ".", dir)
}

func loadFromFS(k *Knowledge, fsys fs.FS, root, displayRoot string) error {
	dirEntries, err := fs.ReadDir(fsys, root)
	if err != nil {
		return nil // directory might not exist
	}
	for _, de := range dirEntries {
		if de.IsDir() || !isYAMLFile(de.Name()) {
			continue
		}
		data, err := fs.ReadFile(fsys, path.Join(root, de.Name()))
		if err != nil {
			slog.Warn("failed to read knowledge file", "file", de.Name(), "dir", displayRoot, "error", err)
			continue
		}
		if err := mergeRawEntries(k, data); err != nil {
			slog.Warn("skipping knowledge file with invalid schema", "file", de.Name(), "dir", displayRoot, "error", err)
			continue
		}
	}
	return nil
}

func loadFile(k *Knowledge, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	if err := mergeRawEntries(k, data); err != nil {
		slog.Warn("skipping knowledge file with invalid schema", "file", filePath, "error", err)
		return nil
	}
	return nil
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
			for lang, forms := range entry.Forms {
				k.Entries[i].Forms[lang] = forms
			}
			if entry.Note != "" {
				k.Entries[i].Note = entry.Note
			}
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
