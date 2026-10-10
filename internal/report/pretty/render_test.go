package pretty

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"assx/internal/lint"
	"assx/internal/report"
)

func TestFrameCropsByDisplayCellsAndExpandsTabs(t *testing.T) {
	line := "前\t" + strings.Repeat("a", 120) + "終"
	shown, marker, _, _ := frame(line, len("前\t"), 1, 40)
	if cells := ansi.StringWidth(shown); cells > 42 {
		t.Fatalf("cropped frame is too long: %d cells: %q", cells, shown)
	}
	if marker < 0 || marker > 24 {
		t.Fatalf("marker position = %d", marker)
	}
}

func TestFrameUsesGraphemeCellWidths(t *testing.T) {
	family := "👩‍👩‍👧‍👦"
	line := "界é" + family + "\tX"
	at := strings.Index(line, family)
	shown, marker, width, _ := frame(line, at, len(family), 120)
	if marker != ansi.StringWidth(expand(line[:at])) || width != 2 || ansi.StringWidth(shown) != ansi.StringWidth(expand(line)) {
		t.Fatalf("frame = %q, marker %d, width %d", shown, marker, width)
	}
}

func TestRenderHandlesCRAndCRLFSourceLines(t *testing.T) {
	view := report.Summary{Total: 1, Remaining: report.FixCounts{Unfixable: 1}, Groups: []report.Group{{
		File: "sample.ass", Diagnostics: []report.Diagnostic{{Line: 2, Column: 1, SourceColumn: 1, SourceWidth: 1, ID: "ASS001", Severity: "error", Title: "Finding"}},
	}}}
	var out strings.Builder
	if err := Render(&out, view, "first\rsecond\r\nthird", false, 80); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "  2 | second\n") {
		t.Fatalf("CR/CRLF source line was not split correctly: %q", out.String())
	}
}

func TestCodeFrameMarkerAlignsAfterFourDigitLineNumber(t *testing.T) {
	view := report.Summary{Total: 1, Groups: []report.Group{{File: "sample.ass", Diagnostics: []report.Diagnostic{{
		Line: 4667, Column: 3, SourceColumn: 3, SourceWidth: 1, ID: "ASS001", Severity: "error", Title: "Finding",
	}}}}}
	source := strings.Repeat("\n", 4666) + "012345"
	var out strings.Builder
	if err := Render(&out, view, source, false, 80); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "  4667 | 012345\n       |   ^\n") {
		t.Fatalf("four-digit source frame is misaligned:\n%s", out.String())
	}
}

func TestFrameAdaptsAtCommonTerminalWidths(t *testing.T) {
	line := strings.Repeat("x", 240)
	for _, width := range []int{40, 80, 120} {
		shown, marker, _, _ := frame(line, 130, 1, width-8)
		if got := ansi.StringWidth(shown); got > width-6 {
			t.Errorf("width %d frame uses %d cells", width, got)
		}
		if marker < 0 || marker > width {
			t.Errorf("width %d marker = %d", width, marker)
		}
	}
}

func BenchmarkRenderReport(b *testing.B) {
	source := strings.Repeat("Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\\\bord1\\\\shad2\\\\fs20}A long subtitle line for report rendering.\n", 200)
	diagnostics := make([]report.Diagnostic, 0, 50)
	for line := 1; line <= 50; line++ {
		diagnostics = append(diagnostics, report.Diagnostic{Line: line, Column: 75, SourceColumn: 75, SourceWidth: 6, ID: "ASS006", Severity: "suggestion", Title: "Override has no effect", Detail: "This override has no effect on the resulting render state.", Outcome: report.FixSafe, Fix: "Remove the redundant override."})
	}
	view := report.Summary{Total: len(diagnostics), Remaining: report.FixCounts{Safe: len(diagnostics)}, Groups: []report.Group{{File: "sample.ass", Diagnostics: diagnostics}}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var out strings.Builder
		if err := Render(&out, view, source, false, 80); err != nil {
			b.Fatal(err)
		}
	}
}

func TestPrettyShowsMultipleFindingsOnOneComplexDialogueLine(t *testing.T) {
	source := `Dialogue: {\pos(1,2,3)\fsbad}Hello`
	posStart := strings.Index(source, `\pos`)
	fsStart := strings.Index(source, `\fsbad`)
	view := report.Summary{Total: 2, Remaining: report.FixCounts{Unfixable: 2}, Groups: []report.Group{{
		File: "sample.ass", Errors: 2, Diagnostics: []report.Diagnostic{
			{Line: 1, Column: 1, SourceColumn: posStart + 1, SourceWidth: len(`\pos(1,2,3)`), ID: "ASS001", Severity: lint.Error, Title: "Bad position", Detail: "Expected two coordinates."},
			{Line: 1, Column: 2, SourceColumn: fsStart + 1, SourceWidth: len(`\fsbad`), ID: "ASS002", Severity: lint.Error, Title: "Bad font size", Detail: "Expected a number."},
		},
	}}}
	var out strings.Builder
	if err := Render(&out, view, source, false, 80); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if strings.Count(got, "  1 | ") != 1 || strings.Count(got, "^~~~~~~~~~~") != 1 || strings.Count(got, "^") != 2 || strings.Index(got, "error[ASS001]") > strings.Index(got, "error[ASS002]") {
		t.Fatalf("complex line findings were omitted or out of order: %s", got)
	}
}

func TestPrettyReprintsChangedViewportAndAlignsEachCaret(t *testing.T) {
	source := strings.Repeat("0123456789", 30)
	columns := []int{35, 180}
	diagnostics := make([]report.Diagnostic, 0, len(columns))
	for i, column := range columns {
		diagnostics = append(diagnostics, report.Diagnostic{Line: 1, Column: column, SourceColumn: column, SourceWidth: 1, ID: fmt.Sprintf("ASS%03d", i), Severity: lint.Error, Title: "Finding"})
	}
	view := report.Summary{Total: len(diagnostics), Groups: []report.Group{{File: "sample.ass", Errors: len(diagnostics), Diagnostics: diagnostics}}}
	var out strings.Builder
	if err := Render(&out, view, source, false, 40); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(out.String(), "\n")
	frames := 0
	for i, line := range lines {
		if !strings.HasPrefix(line, "  1 | ") {
			continue
		}
		frames++
		if i+1 >= len(lines) {
			t.Fatalf("missing marker after %q", line)
		}
		column := columns[frames-1]
		_, wantMarker, _, _ := frame(source, column-1, 1, 34)
		markerLine := lines[i+1]
		bar := strings.Index(markerLine, "|")
		caret := strings.Index(markerLine, "^")
		if bar < 0 || caret-bar-2 != wantMarker {
			t.Fatalf("column %d marker offset = %d, want %d; frame=%q marker=%q", column, caret-bar-2, wantMarker, line, markerLine)
		}
	}
	if frames != len(columns) {
		t.Fatalf("source frames = %d, want %d:\n%s", frames, len(columns), out.String())
	}
}

func TestPrettySortsPresentationByPhysicalSourcePosition(t *testing.T) {
	columns := []int{52, 62, 71, 35}
	diagnostics := make([]report.Diagnostic, 0, len(columns))
	for _, column := range columns {
		diagnostics = append(diagnostics, report.Diagnostic{Line: 1, Column: column, SourceColumn: column, SourceWidth: 1, ID: fmt.Sprintf("ASS%03d", column), Severity: lint.Warning, Title: "Finding"})
	}
	view := report.Summary{Total: len(diagnostics), Groups: []report.Group{{File: "sample.ass", Warnings: len(diagnostics), Diagnostics: diagnostics}}}
	var out strings.Builder
	if err := Render(&out, view, strings.Repeat("x", 100), false, 120); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	last := -1
	for _, column := range []int{35, 52, 62, 71} {
		at := strings.Index(got, fmt.Sprintf("warning[ASS%03d]", column))
		if at <= last {
			t.Fatalf("presentation order not sorted by source column %d: %s", column, got)
		}
		last = at
	}
}

func TestWholeReportFitsCommonTerminalWidths(t *testing.T) {
	view := report.Summary{Total: 1234, Remaining: report.FixCounts{Safe: 111, Unsafe: 222, Unfixable: 901}, Groups: []report.Group{{
		File: strings.Repeat("very-long-directory/", 8) + "sample.ass", Errors: 123, Warnings: 456, Suggestions: 655,
		Diagnostics: []report.Diagnostic{{Line: 1, Column: 2, SourceColumn: 2, SourceWidth: 1, ID: "ASS001", Severity: lint.Error, Title: strings.Repeat("A long diagnostic title ", 4), Detail: strings.Repeat("A detailed explanation with several words. ", 4), Outcome: report.FixUnsafe, Fix: strings.Repeat("Review this change carefully. ", 4)}},
	}}}
	for _, width := range []int{40, 80, 120} {
		var out strings.Builder
		if err := Render(&out, view, "x", false, width); err != nil {
			t.Fatal(err)
		}
		for lineNumber, line := range strings.Split(out.String(), "\n") {
			if cells := ansi.StringWidth(line); cells > width {
				t.Errorf("width %d line %d uses %d cells: %q", width, lineNumber+1, cells, line)
			}
		}
	}
}

func TestPrettyReportGoldenNoColor(t *testing.T) {
	view := report.Summary{Total: 1, Remaining: report.FixCounts{Unfixable: 1}, Groups: []report.Group{{
		File: "sample.ass", Errors: 1, Diagnostics: []report.Diagnostic{{
			Line: 1, Column: 2, SourceColumn: 12, SourceWidth: 6, ID: "ASS001", Severity: lint.Error,
			Title: "Bad tag", Detail: "Invalid font size.", Outcome: report.FixNone,
		}},
	}}}
	source := `Dialogue: {\fsbad}hello`
	var out strings.Builder
	if err := Render(&out, view, source, false, 80); err != nil {
		t.Fatal(err)
	}
	want := "assx  sample.ass\n1 errors  |  0 warnings  |  0 suggestions\n\n" +
		"error[ASS001]  Bad tag  1:12\n" +
		"  1 | Dialogue: {\\fsbad}hello\n" +
		"    |            ^~~~~~\n" +
		"  Invalid font size.\n" +
		"  Fix: edit manually\n\n" +
		"1 diagnostics\n" +
		"Fixes available: 0 safe, 0 unsafe, 1 not auto-fixable.\n"
	if out.String() != want {
		t.Fatalf("report differs from golden:\n--- got ---\n%s--- want ---\n%s", out.String(), want)
	}
}

func TestRenderColorIsOptionalAndDoesNotChangeText(t *testing.T) {
	view := report.Summary{Total: 1, Remaining: report.FixCounts{Unfixable: 1}, Groups: []report.Group{{
		File: "sample.ass", Diagnostics: []report.Diagnostic{{Line: 1, Column: 1, SourceColumn: 1, ID: "ASS001", Severity: "error", Title: "Bad source", Detail: "Invalid token"}},
	}}}
	var plain, colored strings.Builder
	if err := Render(&plain, view, "x", false, 80); err != nil {
		t.Fatal(err)
	}
	if err := Render(&colored, view, "x", true, 80); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain.String(), "\x1b") || !strings.Contains(colored.String(), "\x1b[") || ansi.Strip(colored.String()) != plain.String() {
		t.Fatalf("color output mismatch: plain=%q colored=%q", plain.String(), colored.String())
	}
}

func TestRenderEscapesTerminalControlsAndSeparatesExplanation(t *testing.T) {
	view := report.Summary{Total: 1, Remaining: report.FixCounts{Unfixable: 1}, Groups: []report.Group{{
		File: "bad\x1b[31m.ass", Diagnostics: []report.Diagnostic{{
			Line: 1, Column: 1, SourceColumn: 1, ID: "ASS001", Severity: "error", Title: "Unsafe\x1b[2J",
			Description: "Long rule description", Detail: "Short reason", Fix: "manual",
		}},
	}}}
	var out strings.Builder
	if err := Render(&out, view, "Dialogue: test\r\n", false, 80); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if strings.Contains(got, "\x1b") || !strings.Contains(got, "Short reason") || strings.Contains(got, "Long rule description") {
		t.Fatalf("default report has unsafe or expanded text: %q", got)
	}
	view.Explain = true
	out.Reset()
	if err := Render(&out, view, "Dialogue: test\r\n", false, 80); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Long rule description") {
		t.Fatalf("expanded explanation missing: %q", out.String())
	}
}
