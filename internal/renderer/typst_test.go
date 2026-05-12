package renderer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mmdemirbas/mutercim/internal/lang/ar"
	"github.com/mmdemirbas/mutercim/internal/model"
)

func TestTypstRenderer_Extension(t *testing.T) {
	if got := (&TypstRenderer{}).Extension(); got != ".typ" {
		t.Errorf("Extension = %q, want \".typ\"", got)
	}
}

func TestTypstEscape(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"plain text", "plain text"},
		{"hash#in#middle", `hash\#in\#middle`},
		{"$dollar$", `\$dollar\$`},
		{`back\slash`, `back\\slash`},
		{`<angle>`, `\<angle>`},
		// Arabic content passes through untouched.
		{"أَصبَحتُ بِصُحْبَةٍ", "أَصبَحتُ بِصُحْبَةٍ"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := typstEscape(tt.in); got != tt.want {
				t.Errorf("typstEscape(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestTypstRenderer_RenderPage_EmitsHeadingAndEntries(t *testing.T) {
	page := &model.TranslatedRegionPage{
		PageNumber: 7,
		Regions: []model.TranslatedRegion{
			{ID: "h", Type: model.RegionTypeHeader, TranslatedText: "Chapter Seven"},
			{ID: "e1", Type: model.RegionTypeEntry, TranslatedText: "Entry one body."},
			{ID: "s", Type: model.RegionTypeSeparator},
			{ID: "f1", Type: model.RegionTypeFootnote, TranslatedText: "Footnote one."},
		},
		ReadingOrder: []string{"h", "e1", "s", "f1"},
	}
	out := (&TypstRenderer{Lang: "tr"}).RenderPage(page)
	if !strings.Contains(out, "= Chapter Seven") {
		t.Errorf("missing header in output:\n%s", out)
	}
	if !strings.Contains(out, "Entry one body.") {
		t.Errorf("missing entry text in output:\n%s", out)
	}
	if !strings.Contains(out, "#line(length: 100%") {
		t.Errorf("missing separator line in output:\n%s", out)
	}
	if !strings.Contains(out, "#footnote[Footnote one.]") {
		t.Errorf("missing footnote in output:\n%s", out)
	}
}

func TestTypstRenderer_RenderBook_RTLForArabic(t *testing.T) {
	// Arabic profile must be registered for the renderer to pick up
	// the dir=rtl hint. Test setup mirrors how cmd/mutercim/main.go
	// registers profiles at startup.
	ar.Register()

	pages := []*model.TranslatedRegionPage{
		{
			PageNumber: 1,
			Regions: []model.TranslatedRegion{
				{ID: "e1", Type: model.RegionTypeEntry, TranslatedText: "أَصبَحتُ"},
			},
			ReadingOrder: []string{"e1"},
		},
	}
	out := (&TypstRenderer{Lang: "ar"}).RenderBook(pages)
	if !strings.Contains(out, "Shaikh Hamdullah Mushaf") {
		t.Errorf("Arabic output should reference the Shaikh Hamdullah Mushaf font:\n%s", out)
	}
	if !strings.Contains(out, "#text(dir: rtl)") {
		t.Errorf("Arabic output should wrap body in dir: rtl:\n%s", out)
	}
}

func TestTypstRenderer_RenderBook_LTRForNonArabic(t *testing.T) {
	pages := []*model.TranslatedRegionPage{
		{
			PageNumber: 1,
			Regions: []model.TranslatedRegion{
				{ID: "e1", Type: model.RegionTypeEntry, TranslatedText: "Birinci kayıt."},
			},
			ReadingOrder: []string{"e1"},
		},
	}
	out := (&TypstRenderer{Lang: "tr"}).RenderBook(pages)
	if strings.Contains(out, "#text(dir: rtl)") {
		t.Errorf("Turkish output must NOT wrap in dir: rtl:\n%s", out)
	}
	if !strings.Contains(out, "Alegreya") {
		t.Errorf("Latin output should reference Alegreya font:\n%s", out)
	}
}

// TestTypstRenderer_RealCompile is an integration test: the generated
// .typ must compile cleanly via system `typst`. Skips when typst is
// not installed.
func TestTypstRenderer_RealCompile(t *testing.T) {
	if _, err := exec.LookPath("typst"); err != nil {
		t.Skip("typst not installed")
	}
	ar.Register()

	pages := []*model.TranslatedRegionPage{
		{
			PageNumber: 1,
			Regions: []model.TranslatedRegion{
				{ID: "h", Type: model.RegionTypeHeader, TranslatedText: "الأَنفاس"},
				{ID: "e1", Type: model.RegionTypeEntry, TranslatedText: "أَصبَحتُ بِصُحْبَةٍ أَبَداً"},
				{ID: "s", Type: model.RegionTypeSeparator},
				{ID: "f1", Type: model.RegionTypeFootnote, TranslatedText: "هذه التَّعليقات تُشيرُ"},
			},
			ReadingOrder: []string{"h", "e1", "s", "f1"},
		},
	}
	content := (&TypstRenderer{Lang: "ar"}).RenderBook(pages)

	tmp := t.TempDir()
	typPath := filepath.Join(tmp, "book.typ")
	if err := os.WriteFile(typPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write typ: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	if err := CompileTypstPDF(ctx, typPath, ""); err != nil {
		t.Fatalf("CompileTypstPDF: %v", err)
	}
	pdfPath := strings.TrimSuffix(typPath, ".typ") + ".pdf"
	info, err := os.Stat(pdfPath)
	if err != nil {
		t.Fatalf("expected PDF at %s: %v", pdfPath, err)
	}
	if info.Size() < 1024 {
		t.Errorf("PDF size %d bytes is suspiciously small", info.Size())
	}
}
