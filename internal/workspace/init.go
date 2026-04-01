package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InitOptions configures workspace initialization.
type InitOptions struct {
	Dir         string
	SourceLangs string // comma-separated, e.g. "ar" or "ar,fa"
	TargetLangs string // comma-separated, e.g. "tr" or "tr,en"
}

// Init creates a new workspace directory structure and config file.
func Init(opts InitOptions) (*Workspace, error) {
	root := opts.Dir
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}

	// Check if workspace already exists
	configPath := filepath.Join(root, "mutercim.yaml")
	if _, err := os.Stat(configPath); err == nil {
		return nil, fmt.Errorf("workspace already initialized (mutercim.yaml exists in %s)", root)
	}

	// Set defaults
	if opts.SourceLangs == "" {
		opts.SourceLangs = "ar"
	}
	if opts.TargetLangs == "" {
		opts.TargetLangs = "tr"
	}

	sourceLangs := splitLangs(opts.SourceLangs)
	targetLangs := splitLangs(opts.TargetLangs)

	// Create directory structure
	dirs := []string{
		"input",
		"knowledge",
	}

	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0750); err != nil {
			return nil, fmt.Errorf("create directory %s: %w", d, err)
		}
	}

	// Write config file
	config := generateConfig(sourceLangs, targetLangs)
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	// Scaffold example glossary file
	glossaryPath := filepath.Join(root, "knowledge", "glossary.yaml")
	if err := os.WriteFile(glossaryPath, []byte(glossaryScaffold), 0600); err != nil {
		return nil, fmt.Errorf("write glossary scaffold: %w", err)
	}

	return &Workspace{Root: root}, nil
}

func generateConfig(sourceLangs, targetLangs []string) string {
	return fmt.Sprintf(`# Input files or directories (relative to workspace root)
inputs:
  - path: ./input
    languages: [%s]

cut:
  dpi: 300

layout:
  tool: doclayout-yolo

read:
  models:
    - provider: gemini
      model: gemini-2.0-flash
  concurrency: 1

translate:
  languages: [%s]
  models:
    - provider: gemini
      model: gemini-2.0-flash
  context_window: 2

write:
  formats: [md, pdf]
  expand_sources: true

knowledge: [./knowledge]
`, strings.Join(sourceLangs, ", "), strings.Join(targetLangs, ", "))
}

const glossaryScaffold = `# Glossary entries for translation knowledge.
# Each entry uses ISO 639-1 language codes as keys.
# Values can be a single string or a list (first is canonical, rest are variants).
# The "note" field is optional guidance for the AI.
#
# For a comprehensive Arabic/Turkish/English glossary covering Islamic scholarly
# terminology, honorifics, companion names, and places, see:
#   config/glossary.yaml in the mutercim repository
# Copy it here and customize for your book.

entries:
  # === Format examples ===

  # Simple: one form per language
  # - ar: "أبو هريرة"
  #   tr: "Ebû Hüreyre"
  #   en: "Abu Hurayra"
  #   note: "Prominent companion, narrator of most hadiths"

  # Variants: abbreviations, alternate spellings
  # - ar: ["صلى الله عليه وسلم", "ﷺ", "صلعم"]
  #   tr: ["sallallâhu aleyhi ve sellem", "s.a.v."]
  #   en: ["peace be upon him", "PBUH"]
  #   note: "Salawat. Must appear after every mention of the Prophet."

  # Minimal: two languages only
  # - ar: "فقه"
  #   tr: "fıkıh"
`

// splitLangs splits a comma-separated language string into a slice.
func splitLangs(s string) []string {
	var langs []string
	for _, l := range strings.Split(s, ",") {
		l = strings.TrimSpace(l)
		if l != "" {
			langs = append(langs, l)
		}
	}
	return langs
}
