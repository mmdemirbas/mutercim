package model

// Book holds book-level metadata from the workspace config.
type Book struct {
	Title       string   `yaml:"title" json:"title"`
	SourceLangs []string `yaml:"source_langs" mapstructure:"source_langs" json:"source_langs"`
	TargetLangs []string `yaml:"target_langs" mapstructure:"target_langs" json:"target_langs"`
}

// PrimarySourceLang returns the first (primary) source language.
func (b *Book) PrimarySourceLang() string {
	if len(b.SourceLangs) > 0 {
		return b.SourceLangs[0]
	}
	return "ar"
}

// PrimaryTargetLang returns the first (primary) target language.
func (b *Book) PrimaryTargetLang() string {
	if len(b.TargetLangs) > 0 {
		return b.TargetLangs[0]
	}
	return "tr"
}
