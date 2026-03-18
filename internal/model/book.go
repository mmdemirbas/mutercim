package model

// Book holds book-level metadata from the workspace config.
type Book struct {
	Title      string `yaml:"title" json:"title"`
	Author     string `yaml:"author" json:"author"`
	SourceLang string `yaml:"source_lang" json:"source_lang"`
	TargetLang string `yaml:"target_lang" json:"target_lang"`
}
