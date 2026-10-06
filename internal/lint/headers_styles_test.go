package lint

import (
	"strings"
	"testing"

	"assx/internal/ass"
)

func TestStylePrecisionFindingsAndFixClasses(t *testing.T) {
	text := "[V4+ Styles]\nFormat: Name, MarginL, Fontsize, Italic, ScaleX\nStyle: Default, 12.75, 20.123456789, 0.0, 100.00000000000001\n"
	doc := ass.Parse(text)
	diagnostics := AnalyzeStyles(doc)
	if len(diagnostics) != 4 {
		t.Fatalf("got %d style findings, want 4: %#v", len(diagnostics), diagnostics)
	}
	if diagnostics[0].ID != IssueStyleInteger || diagnostics[0].Field != "Default.marginl" || diagnostics[0].FixSafety != SafeFix {
		t.Fatalf("integer precision diagnostic = %#v", diagnostics[0])
	}
	if diagnostics[1].ID != IssueStyleFloat || diagnostics[1].Field != "Default.fontsize" || diagnostics[1].FixSafety != UnsafeFix {
		t.Fatalf("float precision diagnostic = %#v", diagnostics[1])
	}
	if diagnostics[2].ID != IssueStyleInteger || diagnostics[2].Field != "Default.italic" {
		t.Fatalf("italic precision diagnostic = %#v", diagnostics[2])
	}
	if diagnostics[3].ID != IssueStyleFloat || diagnostics[3].Field != "Default.scalex" {
		t.Fatalf("scale precision diagnostic = %#v", diagnostics[3])
	}
	safeOnly, safeCount, err := ApplyFixes(text, diagnostics, false)
	if err != nil {
		t.Fatal(err)
	}
	if safeCount != 2 || !strings.Contains(safeOnly, "Default, 12, 20.123456789, 0, 100.00000000000001") {
		t.Fatalf("safe-only result (%d): %s", safeCount, safeOnly)
	}
	all, allCount, err := ApplyFixes(text, diagnostics, true)
	if err != nil {
		t.Fatal(err)
	}
	if allCount != 4 || !strings.Contains(all, "Default, 12, 20.123457, 0, 100") {
		t.Fatalf("all-fixes result (%d): %s", allCount, all)
	}
}

func TestStyleFloatPrecisionIgnoresOrdinaryDecimals(t *testing.T) {
	text := "[V4+ Styles]\nFormat: Name, Fontsize, Spacing\nStyle: Default, 30.25, -1.5\n"
	if got := len(AnalyzeStyles(ass.Parse(text))); got != 0 {
		t.Fatalf("ordinary decimal values produced %d findings", got)
	}
}

func TestMatrixAndLayoutHeaderBoundaries(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{"601 at 576p", "[Script Info]\nPlayResY: 576\nYCbCr Matrix: TV.601\nLayoutResX: 640\nLayoutResY: 576\n", 0},
		{"601 above 576p", "[Script Info]\nPlayResY: 720\nYCbCr Matrix: PC.601\nLayoutResX: 1280\nLayoutResY: 720\n", 1},
		{"explicit none", "[Script Info]\nPlayResY: 1080\nYCbCr Matrix: None\nLayoutResX: 1920\nLayoutResY: 1080\n", 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := len(AnalyzeHeaders(ass.Parse(test.text))); got != test.want {
				t.Fatalf("got %d findings, want %d", got, test.want)
			}
		})
	}
}

func TestCountFixesSeparatesApplicableFromUnfixable(t *testing.T) {
	diagnostics := []Diagnostic{
		{FixSafety: SafeFix, Edits: []TextEdit{{Start: 0, End: 0}}},
		{FixSafety: UnsafeFix, Edits: []TextEdit{{Start: 0, End: 0}}},
		{FixSafety: UnsafeFix},
		{},
	}
	if safe, unsafe, unfixable := CountFixes(diagnostics); safe != 1 || unsafe != 1 || unfixable != 2 {
		t.Fatalf("counts = (%d, %d, %d)", safe, unsafe, unfixable)
	}
}
