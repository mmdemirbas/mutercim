package knowledge

// Honorific represents an Arabic honorific formula with its Turkish rendering.
type Honorific struct {
	Arabic        string   `yaml:"arabic" json:"arabic"`
	Abbreviations []string `yaml:"abbreviations,omitempty" json:"abbreviations,omitempty"`
	Turkish       string   `yaml:"turkish" json:"turkish"`
	Context       string   `yaml:"context,omitempty" json:"context,omitempty"`
}

// Source represents a source abbreviation mapping.
type Source struct {
	Code     string `yaml:"code" json:"code"`
	NameAr   string `yaml:"name_ar" json:"name_ar"`
	NameTr   string `yaml:"name_tr" json:"name_tr"`
	AuthorTr string `yaml:"author_tr,omitempty" json:"author_tr,omitempty"`
	Number   string `yaml:"number,omitempty" json:"number,omitempty"`
	Layer    string `yaml:"-" json:"-"` // set during loading, not persisted in YAML
}

// Companion represents a companion name mapping.
type Companion struct {
	Arabic     string `yaml:"arabic" json:"arabic"`
	Turkish    string `yaml:"turkish" json:"turkish"`
	FullNameTr string `yaml:"full_name_tr,omitempty" json:"full_name_tr,omitempty"`
}

// Term represents a terminology entry.
type Term struct {
	Arabic  string `yaml:"arabic" json:"arabic"`
	Turkish string `yaml:"turkish" json:"turkish"`
	Context string `yaml:"context,omitempty" json:"context,omitempty"`
}

// Place represents a place name mapping.
type Place struct {
	Arabic  string `yaml:"arabic" json:"arabic"`
	Turkish string `yaml:"turkish" json:"turkish"`
	Context string `yaml:"context,omitempty" json:"context,omitempty"`
}

// entriesFile is a generic wrapper for YAML files with an entries list.
type entriesFile[T any] struct {
	Entries []T `yaml:"entries"`
}

// Knowledge holds all knowledge data from all layers merged together.
type Knowledge struct {
	Honorifics  []Honorific
	Sources     []Source
	Companions  []Companion
	Terminology []Term
	Places      []Place
}

// LookupSource finds a source by its abbreviation code.
func (k *Knowledge) LookupSource(code string) (Source, bool) {
	for _, s := range k.Sources {
		if s.Code == code {
			return s, true
		}
	}
	return Source{}, false
}
