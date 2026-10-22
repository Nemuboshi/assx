package fontcheck

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"assx/internal/ass/renderer"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
)

func TestAnalyzeRejectsIncompleteCatalog(t *testing.T) {
	isolateCache(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.ttf"), []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	catalog, err := NewCatalog(Source{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	findings, err := Analyze(usageDocument("A"), catalog, renderer.Profile{})
	if err == nil || len(findings) != 0 {
		t.Fatalf("incomplete catalog produced findings: %#v, %v", findings, err)
	}
}

func TestGlyphLookupErrorIsNotMissingCoverage(t *testing.T) {
	isolateCache(t)
	data := append([]byte(nil), goregular.TTF...)
	changed := false
	for i := range int(binary.BigEndian.Uint16(data[4:])) {
		table := data[12+i*16:]
		if string(table[:4]) != "cmap" {
			continue
		}
		cmap := data[int(binary.BigEndian.Uint32(table[8:])):]
		for j := range int(binary.BigEndian.Uint16(cmap[2:])) {
			sub := cmap[int(binary.BigEndian.Uint32(cmap[4+j*8+4:])):]
			if binary.BigEndian.Uint16(sub) != 4 {
				continue
			}
			segments := int(binary.BigEndian.Uint16(sub[6:])) / 2
			for k := range segments {
				binary.BigEndian.PutUint16(sub[16+6*segments+2*k:], 0xfffe)
			}
			changed = true
		}
	}
	if !changed {
		t.Fatal("fixture has no format-4 cmap")
	}
	font, err := sfnt.Parse(data)
	if err != nil {
		t.Fatalf("fixture must parse before glyph lookup: %v", err)
	}
	if _, err := font.GlyphIndex(nil, 'A'); err == nil {
		t.Fatal("fixture must fail at glyph lookup")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.ttf"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	catalog, err := NewCatalog(Source{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	findings, err := Analyze(usageDocument("A"), catalog, renderer.Profile{})
	if err == nil || len(findings) != 0 {
		t.Fatalf("glyph error became coverage claim: %#v, %v", findings, err)
	}
}

func TestCatalogInvalidatesChangedSymlinkTarget(t *testing.T) {
	isolateCache(t)
	target := filepath.Join(t.TempDir(), "font.ttf")
	if err := os.WriteFile(target, goregular.TTF, 0o600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Symlink(target, filepath.Join(dir, "link.ttf")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	first, err := NewCatalog(Source{Directory: dir})
	if err != nil || len(first.families["go"]) == 0 {
		t.Fatalf("first scan: %#v, %v", first, err)
	}
	if err := os.WriteFile(target, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := NewCatalog(Source{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Issues) != 1 || len(second.families["go"]) != 0 {
		t.Fatalf("stale target metadata: %#v", second)
	}
}
