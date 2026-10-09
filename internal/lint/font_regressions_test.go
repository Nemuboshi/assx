package lint

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/goregular"
)

func TestFontFamiliesKeepRegionalSuffixesAndPunctuation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "regular.ttf"), goregular.TTF, 0o600); err != nil {
		t.Fatal(err)
	}
	checker, err := NewFontChecker(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range []string{"Go JP", "Go SC", "Go TC", "Go-JP", "Go.JP", "G-o"} {
		doc := parseStyleOverrideText("Name, Fontname", "Default,"+family, "A")
		diagnostics, err := AnalyzeFonts(doc, checker)
		if err != nil || len(diagnostics) != 1 || diagnostics[0].ID != IssueFontMissing {
			t.Fatalf("family %q: %#v / %v", family, diagnostics, err)
		}
	}
	for _, text := range []string{`{\fn 0}A`, "{\\fn\t0}A"} {
		doc := parseStyleOverrideText("Name, Fontname", "Default,Go", text)
		if diagnostics, err := AnalyzeFonts(doc, checker); err != nil || len(diagnostics) != 1 || diagnostics[0].ID != IssueFontMissing {
			t.Fatalf("literal zero family %q: %#v / %v", text, diagnostics, err)
		}
	}
	for _, family := range []string{"Go", "GO", "go"} {
		doc := parseStyleOverrideText("Name, Fontname", "Default,"+family, "A")
		if diagnostics, err := AnalyzeFonts(doc, checker); err != nil || len(diagnostics) != 0 {
			t.Fatalf("case variant %q: %#v / %v", family, diagnostics, err)
		}
	}
}

func TestFontChecksFollowOverrideResets(t *testing.T) {
	for _, test := range []struct {
		name, definitions, text, selected string
	}{
		{"zero family", "Style: Default,Go,0,0\n", `{\fnMissing\fn0}`, "regular.ttf"},
		{"parenthesized zero family", "Style: Default,Go,0,0\n", `{\fnMissing\fn( 0 )}`, "regular.ttf"},
		{"empty parenthesized bold", "Style: Default,Go,-1,0\n", `{\b( )}`, "bold.ttf"},
		{"empty parenthesized italic", "Style: Default,Go,0,-1\n", `{\i( )}`, "italic.ttf"},
		{"bare family", "Style: Default,Go,0,0\n", `{\fnMissing\fn}`, "regular.ttf"},
		{"bare bold", "Style: Default,Go,0,0\n", `{\b1\b}`, "regular.ttf"},
		{"invalid bold restores style", "Style: Default,Go,0,0\n", `{\b1\b2}`, "regular.ttf"},
		{"bare italic", "Style: Default,Go,0,0\n", `{\i1\i}`, "regular.ttf"},
		{"invalid italic restores style", "Style: Default,Go,0,0\n", `{\i1\i2}`, "regular.ttf"},
		{"bare drawing", "Style: Default,Go,0,0\n", `{\p1}m 0 0 l 10 10{\p}`, "regular.ttf"},
		{"named reset bold differs between renderers", "Style: Default,Go,0,0\nStyle: Other,Go,-1,0\n", `{\rOther\b0\b}`, ""},
		{"named reset italic differs between renderers", "Style: Default,Go,0,0\nStyle: Other,Go,0,-1\n", `{\rOther\i0\i}`, ""},
		{"named reset common bold", "Style: Default,Go,-1,0\nStyle: Other,Go,-1,0\n", `{\rOther\b0\b}`, "bold.ttf"},
		{"named reset common italic", "Style: Default,Go,0,-1\nStyle: Other,Go,0,-1\n", `{\rOther\i0\i}`, "italic.ttf"},
		{"base reset", "Style: Default,Go,0,0\nStyle: Other,Missing,-1,-1\n", `{\rOther\r}`, "regular.ttf"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			fonts := map[string][]byte{"regular.ttf": goregular.TTF, "bold.ttf": gobold.TTF, "italic.ttf": goitalic.TTF}
			for name, data := range fonts {
				if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			checker, err := NewFontChecker(dir)
			if err != nil {
				t.Fatal(err)
			}
			// A wrong face selection must fail instead of looking identical in the test fonts.
			for name := range fonts {
				if test.selected != "" && name != test.selected {
					if err := os.Remove(filepath.Join(dir, name)); err != nil {
						t.Fatal(err)
					}
				}
			}
			doc := parseStyleDefinitionsText("Name, Fontname, Bold, Italic", test.definitions, "Default", test.text+string(rune(0x10ffff)))
			diagnostics, err := AnalyzeFonts(doc, checker)
			if test.selected == "" {
				if err != nil || len(diagnostics) != 0 {
					t.Fatalf("renderer-dependent font state was reported as certain: %#v / %v", diagnostics, err)
				}
				return
			}
			if err != nil || len(diagnostics) != 1 || diagnostics[0].ID != IssueMissingGlyphs {
				t.Fatalf("text %q: %#v / %v", test.text, diagnostics, err)
			}
		})
	}
}
