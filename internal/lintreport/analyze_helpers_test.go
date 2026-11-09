package lintreport

import (
	"context"
	"testing"

	"assx/internal/analysis"
	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/lint"
)

func analyzeDocument(t *testing.T, doc renderer.Document) []lint.Diagnostic {
	t.Helper()
	snapshot, err := analysis.NewSnapshot("test.ass", doc.Text, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := analysis.Analyze(context.Background(), snapshot, ass.ParseSource(doc.Text), analysis.Options{})
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := result.Diagnostics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return diagnostics
}
