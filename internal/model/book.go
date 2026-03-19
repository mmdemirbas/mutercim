package model

// Book holds book-level metadata from the workspace config.
type Book struct {
	Title       string   `yaml:"title" json:"title"`
	Author      string   `yaml:"author" json:"author"`
	SourceLang  string   `yaml:"source_lang,omitempty" mapstructure:"source_lang" json:"source_lang,omitempty"` // deprecated: use SourceLangs
	TargetLang  string   `yaml:"target_lang,omitempty" mapstructure:"target_lang" json:"target_lang,omitempty"` // deprecated: use TargetLangs
	SourceLangs []string `yaml:"source_langs,omitempty" mapstructure:"source_langs" json:"source_langs,omitempty"`
	TargetLangs []string `yaml:"target_langs,omitempty" mapstructure:"target_langs" json:"target_langs,omitempty"`
}

// PrimarySourceLang returns the first (primary) source language.
func (b *Book) PrimarySourceLang() string {
	if len(b.SourceLangs) > 0 {
		return b.SourceLangs[0]
	}
	if b.SourceLang != "" {
		return b.SourceLang
	}
	return "ar"
}

// PrimaryTargetLang returns the first (primary) target language.
func (b *Book) PrimaryTargetLang() string {
	if len(b.TargetLangs) > 0 {
		return b.TargetLangs[0]
	}
	if b.TargetLang != "" {
		return b.TargetLang
	}
	return "tr"
}
