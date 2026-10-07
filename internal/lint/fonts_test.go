package lint

import (
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
	checker, err := NewFontChecker(filepath.Dir(fontDir))
	if err != nil {
		t.Fatal(err)
	}

	diagnostics, err := AnalyzeFonts(doc, checker)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, diagnostic := range diagnostics {
		counts[diagnostic.ID]++
		if diagnostic.Severity != Suggestion {
			t.Errorf("font finding %s severity = %q, want %q", diagnostic.ID, diagnostic.Severity, Suggestion)
		}
		if diagnostic.FixSafety != "" || len(diagnostic.Edits) != 0 {
			t.Errorf("font finding %s has automatic fix metadata: %#v", diagnostic.ID, diagnostic)
		}
	}
	if counts[IssueFontMissing] != 2 || counts[IssueMissingGlyphs] != 1 || len(diagnostics) != 3 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if safe, unsafe, unfixable := CountFixes(diagnostics); safe != 0 || unsafe != 0 || unfixable != 3 {
		t.Fatalf("fix counts = (%d, %d, %d), want (0, 0, 3)", safe, unsafe, unfixable)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.ID == IssueMissingGlyphs && !strings.Contains(diagnostic.Detail, "characters") {
			t.Fatalf("missing-glyph detail = %q", diagnostic.Detail)
		}
	}
}

func TestFontDirectoryDoesNotFallBackToSystemFonts(t *testing.T) {
	checker, err := NewFontChecker(t.TempDir())
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
	checker, err := NewFontChecker(fontDir)
	if err != nil {
		t.Fatal(err)
	}

	doc := ass.Parse(`[V4+ Styles]
Format: Name, Fontname, Fontsize, Bold, Italic
Style: Main,Go,20,0,0
Style: main,Definitely Missing,20,0,0
Style: Default,Go,20,0,0
[Events]
Format: Layer, Start, End, Style, Text
Dialogue: 0,0,1,Main,A
Dialogue: 0,0,1,main,B
Dialogue: 0,0,1,default,C
`)
	diagnostics, err := AnalyzeFonts(doc, checker)
	if err != nil {
		t.Fatal(err)
	}

	var missing []Diagnostic
	for _, diagnostic := range diagnostics {
		if diagnostic.ID == IssueFontMissing {
			missing = append(missing, diagnostic)
		}
	}
	if len(missing) != 1 || !strings.Contains(missing[0].Detail, "Definitely Missing") {
		t.Fatalf("font diagnostics = %#v", diagnostics)
	}
}
