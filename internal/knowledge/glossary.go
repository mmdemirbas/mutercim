package knowledge

import (
	"fmt"
	"strings"
)

// BuildGlossary creates a formatted glossary string for prompt injection.
func (k *Knowledge) BuildGlossary() string {
	var sections []string

	if s := k.buildHonorificsSection(); s != "" {
		sections = append(sections, s)
	}
	if s := k.buildCompanionsSection(); s != "" {
		sections = append(sections, s)
	}
	if s := k.buildSourcesSection(); s != "" {
		sections = append(sections, s)
	}
	if s := k.buildTerminologySection(); s != "" {
		sections = append(sections, s)
	}

	return strings.Join(sections, "\n\n")
}

// HonorificsSection returns the honorifics section for prompt injection.
func (k *Knowledge) HonorificsSection() string {
	return k.buildHonorificsSection()
}

// CompanionsSection returns the companions section for prompt injection.
func (k *Knowledge) CompanionsSection() string {
	return k.buildCompanionsSection()
}

// SourcesSection returns the sources section for prompt injection.
func (k *Knowledge) SourcesSection() string {
	return k.buildSourcesSection()
}

// TerminologySection returns the terminology section for prompt injection.
func (k *Knowledge) TerminologySection() string {
	return k.buildTerminologySection()
}

func (k *Knowledge) buildHonorificsSection() string {
	if len(k.Honorifics) == 0 {
		return ""
	}
	var lines []string
	for _, h := range k.Honorifics {
		lines = append(lines, fmt.Sprintf("- %s → %s", h.Arabic, h.Turkish))
	}
	return strings.Join(lines, "\n")
}

func (k *Knowledge) buildCompanionsSection() string {
	if len(k.Companions) == 0 {
		return ""
	}
	var lines []string
	for _, c := range k.Companions {
		lines = append(lines, fmt.Sprintf("- %s → %s", c.Arabic, c.Turkish))
	}
	return strings.Join(lines, "\n")
}

func (k *Knowledge) buildSourcesSection() string {
	if len(k.Sources) == 0 {
		return ""
	}
	var lines []string
	for _, s := range k.Sources {
		lines = append(lines, fmt.Sprintf("- %s = %s (%s)", s.Code, s.NameTr, s.NameAr))
	}
	return strings.Join(lines, "\n")
}

func (k *Knowledge) buildTerminologySection() string {
	if len(k.Terminology) == 0 {
		return ""
	}
	var lines []string
	for _, t := range k.Terminology {
		lines = append(lines, fmt.Sprintf("- %s → %s", t.Arabic, t.Turkish))
	}
	return strings.Join(lines, "\n")
}
