package main

import (
	"strings"
	"testing"
	"time"

	"assx/internal/report"
	"assx/internal/report/plain"
)

func TestPlainPresenterPreservesLegacyBytes(t *testing.T) {
	diagnostics := []report.Diagnostic{
		{Condition: "value", Severity: "warning", Title: "Second", Line: 4, Column: 9, Detail: "detail"},
		{Condition: "arity", Severity: "error", Title: "First", Line: 2, Column: 3, Tag: "pos"},
	}
	var out strings.Builder
	if err := plain.Render(&out, "sample.ass", diagnostics, 12*time.Millisecond, "Fixes available: 0 safe, 0 unsafe, 2 not auto-fixable."); err != nil {
		t.Fatal(err)
	}
	want := "ERROR[arity] sample.ass:2:3\n    First\n    tag: \\pos\n\nWARNING[value] sample.ass:4:9\n    Second\n    detail\n\nChecked sample.ass in 12ms. Summary: 2 diagnostics (1 errors, 1 warnings, 0 suggestions).\nFixes available: 0 safe, 0 unsafe, 2 not auto-fixable.\n"
	if out.String() != want {
		t.Fatalf("plain output changed:\n%q", out.String())
	}
}
