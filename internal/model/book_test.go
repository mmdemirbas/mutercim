package model

import "testing"

func TestPrimarySourceLang(t *testing.T) {
	tests := []struct {
		name string
		book Book
		want string
	}{
		{"from SourceLangs", Book{SourceLangs: []string{"ar", "fa"}}, "ar"},
		{"default", Book{}, "ar"},
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
		{"default", Book{}, "tr"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.book.PrimaryTargetLang(); got != tt.want {
				t.Errorf("PrimaryTargetLang() = %q, want %q", got, tt.want)
			}
		})
	}
}
