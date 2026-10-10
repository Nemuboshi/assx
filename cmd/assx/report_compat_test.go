package main

import (
	"strings"
	"testing"
	"time"

	"assx/internal/lint"
	"assx/internal/report/plain"
)

func TestPlainPresenterPreservesLegacyBytes(t *testing.T) {
	diagnostics := []lint.Diagnostic{
		{ID: "ASS002", Severity: lint.Warning, Title: "Second", Line: 4, Column: 9, Detail: "detail"},
		{ID: "ASS001", Severity: lint.Error, Title: "First", Line: 2, Column: 3, Tag: "pos"},
	}
	var out strings.Builder
	if err := plain.Render(&out, "sample.ass", diagnostics, 12*time.Millisecond, "Fixes available: 0 safe, 0 unsafe, 2 not auto-fixable."); err != nil {
		t.Fatal(err)
	}
	want := "ERROR[ASS001] sample.ass:2:3\n    First\n    tag: \\pos\n\nWARNING[ASS002] sample.ass:4:9\n    Second\n    detail\n\nChecked sample.ass in 12ms. Summary: 2 diagnostics (1 errors, 1 warnings, 0 suggestions).\nFixes available: 0 safe, 0 unsafe, 2 not auto-fixable.\n"
	if out.String() != want {
		t.Fatalf("plain output changed:\n%q", out.String())
	}
}
