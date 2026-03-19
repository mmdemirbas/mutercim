package model

import "testing"

func TestPrimarySourceLang(t *testing.T) {
	tests := []struct {
		name string
		book Book
		want string
	}{
		{"from SourceLangs", Book{SourceLangs: []string{"ar", "fa"}}, "ar"},
		{"from deprecated SourceLang", Book{SourceLang: "fa"}, "fa"},
		{"default", Book{}, "ar"},
		{"SourceLangs takes precedence", Book{SourceLang: "fa", SourceLangs: []string{"ar"}}, "ar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.book.PrimarySourceLang(); got != tt.want {
				t.Errorf("PrimarySourceLang() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrimaryTargetLang(t *testing.T) {
	tests := []struct {
		name string
		book Book
		want string
	}{
		{"from TargetLangs", Book{TargetLangs: []string{"tr", "en"}}, "tr"},
		{"from deprecated TargetLang", Book{TargetLang: "en"}, "en"},
		{"default", Book{}, "tr"},
		{"TargetLangs takes precedence", Book{TargetLang: "en", TargetLangs: []string{"tr"}}, "tr"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.book.PrimaryTargetLang(); got != tt.want {
				t.Errorf("PrimaryTargetLang() = %q, want %q", got, tt.want)
			}
		})
	}
}
