package lint

import (
	"testing"

	"assx/internal/ass"
	"assx/internal/perftest"
)

var benchmarkDiagnostics []Diagnostic

// Measures all lint passes with document parsing excluded.
func BenchmarkAnalyzeDocumentFixtures(b *testing.B) {
	for _, test := range perftest.Cases(b) {
		doc := ass.Parse(test.Text)
		b.Run(test.Name, func(b *testing.B) {
			b.SetBytes(int64(len(test.Text)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkDiagnostics = AnalyzeDocument(doc)
			}
		})
	}
}

// Measures end-to-end ASS parsing and linting for the same workloads.
func BenchmarkParseAndAnalyzeDocumentFixtures(b *testing.B) {
	for _, test := range perftest.Cases(b) {
		b.Run(test.Name, func(b *testing.B) {
			b.SetBytes(int64(len(test.Text)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkDiagnostics = AnalyzeDocument(ass.Parse(test.Text))
			}
		})
	}
}
