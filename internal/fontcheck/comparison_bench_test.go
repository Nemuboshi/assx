package fontcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"golang.org/x/image/font/gofont/goregular"
)

func BenchmarkFontCatalogComparison(b *testing.B) {
	dir := b.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "font.ttf"), goregular.TTF, 0o600); err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := NewCatalog(Source{Directory: dir}); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkFontAnalysisComparison(b *testing.B) {
	dir := b.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "font.ttf"), goregular.TTF, 0o600); err != nil {
		b.Fatal(err)
	}
	c, err := NewCatalog(Source{Directory: dir})
	if err != nil {
		b.Fatal(err)
	}
	data, err := os.ReadFile("../../testdata/font-check.ass")
	if err != nil {
		b.Fatal(err)
	}
	doc := ass.Parse(strings.ReplaceAll(string(data), "MISSING_GLYPH", string(rune(0x10ffff))))
	for b.Loop() {
		if _, err := Analyze(doc, c, renderer.Profile{}); err != nil {
			b.Fatal(err)
		}
	}
}
