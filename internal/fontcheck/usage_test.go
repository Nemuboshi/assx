package fontcheck

import (
	"strings"
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

func usageDocument(text string) ass.Document {
	return ass.Parse("[V4+ Styles]\nFormat: Name, Fontname, Bold, Italic\nStyle: Default,Go,0,0\n[Events]\nFormat: Style, Text\nDialogue: Default," + text + "\n")
}

func TestExtractUsagesRetainsByteSpansAndUnknownState(t *testing.T) {
	doc := usageDocument("é{\\fnMissing}中{\\rAbsent}X")
	profile, _ := renderer.Standard(renderer.Libass)
	usages := ExtractUsages(doc, profile)
	if len(usages) != 3 {
		t.Fatalf("usages = %#v", usages)
	}
	for i, want := range []string{"é", "中", "X"} {
		u := usages[i]
		if u.Text != want || doc.Text[u.Span.Start:u.Span.End] != want || u.Profile != profile {
			t.Fatalf("usage %d = %#v", i, u)
		}
	}
	if !usages[0].Known || usages[0].Request.Family != "Go" || !usages[1].Known || usages[1].Request.Family != "Missing" || usages[2].Known {
		t.Fatalf("request certainty = %#v", usages)
	}
	if usages[1].Column != strings.Index(doc.Dialogues[0].Text, "中")+1 {
		t.Fatalf("column = %d", usages[1].Column)
	}
}

func TestAnalyzeDoesNotReportUnknownOrDrawingText(t *testing.T) {
	catalog, err := NewCatalog(Source{Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	profile, _ := renderer.Standard(renderer.Libass)
	for _, text := range []string{`{\p1}m 0 0 l 10 10`, `{\rAbsent}X`, `{\t(\fnMissing)}X`} {
		findings, err := Analyze(usageDocument(text), catalog, profile)
		if err != nil || len(findings) != 0 {
			t.Fatalf("%q: %#v, %v", text, findings, err)
		}
	}
}
