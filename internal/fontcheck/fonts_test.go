package fontcheck

import (
	"assx/internal/ass/renderer"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"assx/internal/ass"
	"golang.org/x/image/font/gofont/goregular"
)

func TestAnalyzeFontsReportsMissingFamilyAndGlyph(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "font-check.ass"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(fixture), "MISSING_GLYPH", string(rune(0x10ffff)))
	doc := ass.Parse(text)

	fontDir := filepath.Join(t.TempDir(), "nested", "fonts")
	if err := os.MkdirAll(fontDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fontDir, "regular.ttf"), goregular.TTF, 0o600); err != nil {
		t.Fatal(err)
	}
	checker, err := newTestCatalog(filepath.Dir(fontDir))
	if err != nil {
		t.Fatal(err)
	}

	diagnostics, err := Analyze(doc, checker, renderer.Profile{})
	if err != nil {
		t.Fatal(err)
	}
	counts := map[Kind]int{}
	for _, diagnostic := range diagnostics {
		counts[diagnostic.Kind]++
	}
	if counts[FamilyAbsent] != 2 || counts[MissingCharacters] != 1 || len(diagnostics) != 3 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestFontDirectoryDoesNotFallBackToSystemFonts(t *testing.T) {
	checker, err := newTestCatalog(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, found, err := checker.missingRunes(fontContext{family: "Go"}, "A")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("empty explicit font directory unexpectedly found a font")
	}
}

func TestAnalyzeFontsUsesRendererStyleLookup(t *testing.T) {
	fontDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(fontDir, "regular.ttf"), goregular.TTF, 0o600); err != nil {
		t.Fatal(err)
	}
	checker, err := newTestCatalog(fontDir)
	if err != nil {
		t.Fatal(err)
	}

	doc := ass.Parse(`[V4+ Styles]
Format: Name, Fontname, Fontsize, Bold, Italic
Style: Main,Go,20,0,0
Style: main,Definitely Missing,20,0,0
Style: Default,Go,20,0,0
[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:00.00,0:00:01.00,Main,,0,0,0,,A
Dialogue: 0,0:00:01.00,0:00:02.00,main,,0,0,0,,B
Dialogue: 0,0:00:02.00,0:00:03.00,default,,0,0,0,,C
`)
	diagnostics, err := Analyze(doc, checker, renderer.Profile{})
	if err != nil {
		t.Fatal(err)
	}

	var missing []Finding
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == FamilyAbsent {
			missing = append(missing, diagnostic)
		}
	}
	if len(missing) != 1 || !strings.Contains(missing[0].Usages[0].Request.Family, "Definitely Missing") {
		t.Fatalf("font diagnostics = %#v", diagnostics)
	}
}

func newTestCatalog(dir string) (*Catalog, error) { return NewCatalog(Source{Directory: dir}) }
