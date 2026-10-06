package lint

import (
	"strings"
	"testing"

	"assx/internal/ass"
)

func TestDefaultASSStyleFormatChecksIntegerFields(t *testing.T) {
	text := "[V4+ Styles]\nStyle: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,0,2,20.5,30,40,1\n"
	diagnostics := AnalyzeStyles(ass.Parse(text))
	if len(diagnostics) != 1 || diagnostics[0].ID != IssueStyleInteger || diagnostics[0].Field != "Default.marginl" {
		t.Fatalf("default format diagnostics = %#v", diagnostics)
	}
	fixed, count, err := ApplyFixes(text, diagnostics, false)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || !strings.Contains(fixed, ",2,20,30,40,1") {
		t.Fatalf("integer style fix = (%d, %q)", count, fixed)
	}
}
