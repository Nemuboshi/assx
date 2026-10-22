package main

import (
	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/fontcheck"
	"golang.org/x/image/font/gofont/goregular"
	"os"
	"path/filepath"
	"testing"
)

func TestRendererScopedFontAnalysisUsesSharedState(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "font-check.ass"))
	if err != nil {
		t.Fatal(err)
	}
	fontDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(fontDir, "Go-Regular.ttf"), goregular.TTF, 0o600); err != nil {
		t.Fatal(err)
	}
	checker, err := fontcheck.NewCatalog(fontcheck.Source{Directory: fontDir})
	if err != nil {
		t.Fatal(err)
	}
	profile, err := renderer.Standard(renderer.XYVSFilter)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := legacyFontDiagnostics(ass.Parse(string(content)), checker, renderer.Profile{})
	if err != nil {
		t.Fatal(err)
	}
	scoped, err := legacyFontDiagnostics(ass.Parse(string(content)), checker, profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped) != len(legacy) {
		t.Fatalf("font consumers diverged unexpectedly: legacy=%#v scoped=%#v", legacy, scoped)
	}
	for _, d := range scoped {
		if d.FixSafety != "" || len(d.Edits) != 0 {
			t.Fatalf("font observation received an unsupported edit: %#v", d)
		}
	}
}
