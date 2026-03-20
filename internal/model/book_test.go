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
