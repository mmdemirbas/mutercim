// Package lang provides per-language profile plugins.
//
// A Profile is the language-agnostic vehicle for behaviour that today is
// scattered as Arabic-specific defaults across the codebase: RTL hints
// for renderers, the Adab corpus injected into translate prompts, the
// Qari OCR dispatch, and the 2-column reading-order fixup for Arabic
// Islamic books.
//
// Profiles are looked up by ISO 639-1 source-language code from the
// input declaration in mutercim.yaml. When no profile matches, the
// EmptyProfile is returned — every callsite must work cleanly with the
// empty case, so adding new languages is purely additive.
package lang

import (
	"strings"

	"github.com/mmdemirbas/mutercim/internal/model"
)

// Profile is the per-language plugin interface. All methods have safe
// no-op defaults via EmptyProfile so callers can dispatch without
// nil-checking. New languages plug in by implementing only the methods
// that matter for their content shape.
type Profile interface {
	// Code returns the ISO 639-1 language code this profile targets.
	Code() string

	// ScriptDetect returns true when the given text is plausibly written
	// in this profile's script. Used by the dispatcher to pick the right
	// profile for a mixed-language input.
	ScriptDetect(text string) bool

	// OcrOverride returns the preferred OCR tool name for this language,
	// or empty to use the global OCR config. e.g. Arabic returns "qari".
	OcrOverride() string

	// ReadingOrderFixup post-processes a region list to repair
	// reading-order failures specific to this language. Returns the
	// corrected reading-order ID list. Returning the input slice
	// unchanged means "no fixup applies." Pure function — does not
	// mutate input regions.
	ReadingOrderFixup(regions []model.Region, currentOrder []string) []string

	// RenderingHints returns hints for the write phase. The "dir" key
	// is read as "rtl" or "ltr" by the LaTeX/Typst/docx renderers.
	// Empty map means "no overrides."
	RenderingHints() map[string]string

	// PromptCorpus returns additional prompt text injected into the
	// translate phase system prompt for this language. Empty means
	// "use only the workspace knowledge." Reserved for the future
	// Adab-as-plugin migration.
	PromptCorpus() string
}

// EmptyProfile is the language-agnostic no-op profile used for any
// source language without an explicit registered profile.
type EmptyProfile struct {
	code string
}

// Code returns the language code this empty profile represents.
func (e EmptyProfile) Code() string { return e.code }

// ScriptDetect for the empty profile always returns false — there is
// no script to match against.
func (EmptyProfile) ScriptDetect(string) bool { return false }

// OcrOverride for the empty profile is empty (use global config).
func (EmptyProfile) OcrOverride() string { return "" }

// ReadingOrderFixup for the empty profile returns the input order
// unchanged.
func (EmptyProfile) ReadingOrderFixup(_ []model.Region, currentOrder []string) []string {
	return currentOrder
}

// RenderingHints for the empty profile is nil (no overrides).
func (EmptyProfile) RenderingHints() map[string]string { return nil }

// PromptCorpus for the empty profile is empty.
func (EmptyProfile) PromptCorpus() string { return "" }

// profileRegistry maps language code → registered Profile. Populated by
// init functions in each internal/lang/<code>/ subpackage via Register.
var profileRegistry = map[string]Profile{}

// Register installs a Profile under its Code(). Idempotent — repeated
// registration of the same code is silently accepted so tests can call
// Register without coordinating across packages.
func Register(p Profile) {
	profileRegistry[p.Code()] = p
}

// Get returns the profile for the given language code, or an
// EmptyProfile when nothing is registered. Callers never receive nil.
//
// Language codes are matched case-insensitively. An empty argument
// returns the empty profile, so it is safe to call with an input that
// may not have a declared language.
func Get(code string) Profile {
	key := strings.ToLower(strings.TrimSpace(code))
	if key == "" {
		return EmptyProfile{}
	}
	if p, ok := profileRegistry[key]; ok {
		return p
	}
	return EmptyProfile{code: key}
}

// Registered returns the codes of all currently-registered profiles
// (excluding empty profiles returned from Get for unknown codes).
// Order is unspecified. Provided for status display.
func Registered() []string {
	codes := make([]string, 0, len(profileRegistry))
	for c := range profileRegistry {
		codes = append(codes, c)
	}
	return codes
}
