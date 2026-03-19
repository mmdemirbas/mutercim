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
	if s := k.buildPeopleSection(); s != "" {
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

// PeopleSection returns the people section for prompt injection.
func (k *Knowledge) PeopleSection() string {
	return k.buildPeopleSection()
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

func (k *Knowledge) buildPeopleSection() string {
	if len(k.People) == 0 {
		return ""
	}
	var lines []string
	for _, p := range k.People {
		lines = append(lines, fmt.Sprintf("- %s → %s", p.Arabic, p.Turkish))
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
