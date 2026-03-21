package workspace

import "testing"

func TestSanitizeTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"normal title", "My Book", "My Book"},
		{"arabic title", "كتاب الأنفاس", "كتاب الأنفاس"},
		{"turkish title", "Hadîs-i Şerîf", "Hadîs-i Şerîf"},
		{"chinese title", "论语", "论语"},
		{"os prohibited slash", "vol/1", "vol-1"},
		{"os prohibited backslash", `vol\1`, "vol-1"},
		{"os prohibited colon", "book: chapter", "book- chapter"},
		{"os prohibited star", "book*", "book"},
		{"os prohibited question", "what?", "what"},
		{"os prohibited quotes", `"title"`, "title"},
		{"os prohibited angle brackets", "<title>", "title"},
		{"os prohibited pipe", "a|b", "a-b"},
		{"os prohibited null", "a\x00b", "a-b"},
		{"multiple prohibited chars", "a/b\\c:d", "a-b-c-d"},
		{"collapse multiple dashes", "a---b", "a-b"},
		{"prohibited chars collapse", "a//b", "a-b"},
		{"trim spaces", "  title  ", "title"},
		{"trim dots", "..title..", "title"},
		{"trim spaces and dots", " .title. ", "title"},
		{"empty string", "", "book"},
		{"only spaces", "   ", "book"},
		{"only dots", "...", "book"},
		{"only prohibited chars", "/:*?", "book"},
		{"mixed unicode and prohibited", "كتاب/الأول", "كتاب-الأول"},
		{"long ASCII title truncated to 80 runes", "ABCDEFGHIJ ABCDEFGHIJ ABCDEFGHIJ ABCDEFGHIJ ABCDEFGHIJ ABCDEFGHIJ ABCDEFGHIJ ABCDEFGHIJ ABCDEFGHIJ", "ABCDEFGHIJ ABCDEFGHIJ ABCDEFGHIJ ABCDEFGHIJ ABCDEFGHIJ ABCDEFGHIJ ABCDEFGHIJ ABC"},
		{"long Arabic title truncated to 80 runes", "كتاب الأحكام الكبير في الفقه الإسلامي المقارن والسنة النبوية الشريفة والعقائد والتوحيد والأصول والفروع", "كتاب الأحكام الكبير في الفقه الإسلامي المقارن والسنة النبوية الشريفة والعقائد وا"},
		{"exactly 80 runes not truncated", "ABCDEFGHIJKLMNOPQRSTUVWXYZ ABCDEFGHIJKLMNOPQRSTUVWXYZ ABCDEFGHIJKLMNOPQRSTUVWXYZ", "ABCDEFGHIJKLMNOPQRSTUVWXYZ ABCDEFGHIJKLMNOPQRSTUVWXYZ ABCDEFGHIJKLMNOPQRSTUVWXYZ"},
		{"truncation trims trailing dash", "ABCDEFGHIJ-ABCDEFGHIJ-ABCDEFGHIJ-ABCDEFGHIJ-ABCDEFGHIJ-ABCDEFGHIJ-ABCDEFGHIJ-AB-CDEFGHIJ", "ABCDEFGHIJ-ABCDEFGHIJ-ABCDEFGHIJ-ABCDEFGHIJ-ABCDEFGHIJ-ABCDEFGHIJ-ABCDEFGHIJ-AB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeTitle(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
