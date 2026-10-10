package lint

import (
	"strings"
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
	lateFailure := ass.Parse(benchmarkLateProofFailure(1000))
	initial := AnalyzeDocument(lateFailure)
	firstProof, lateCandidate, lateProof := false, false, false
	lastLine := lateFailure.Dialogues[len(lateFailure.Dialogues)-1].Line
	for _, diagnostic := range initial {
		if diagnostic.ID != IssueNoEffect {
			continue
		}
		if diagnostic.Line == lateFailure.Dialogues[0].Line && diagnostic.FixSafety == SafeFix && diagnostic.FixProof != nil {
			firstProof = true
		}
		if diagnostic.Line == lastLine {
			lateCandidate = true
			lateProof = lateProof || diagnostic.FixSafety == SafeFix && diagnostic.FixProof != nil
		}
	}
	if !firstProof || !lateCandidate || lateProof {
		b.Fatal("benchmark did not isolate the late unresolved SafeFix candidate")
	}
	b.Run("late_candidate_failure_1000", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchmarkDiagnostics = AnalyzeDocument(lateFailure)
		}
	})
}

func benchmarkLateProofFailure(dialogues int) string {
	text := benchmarkASSDocument(dialogues)
	before := `\t(0,500,\fscx100)}Benchmark subtitle`
	after := `\t(0,500,\fscx100)\mystery}Benchmark subtitle`
	start := strings.LastIndex(text, before)
	if start < 0 {
		panic("benchmark dialogue marker not found")
	}
	return text[:start] + after + text[start+len(before):]
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
