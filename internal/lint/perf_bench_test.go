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
	for _, diagnostic := range primaryFindings(initial) {
		if diagnostic.ID != IssueNoEffect || diagnostic.Tag != "b" || diagnostic.Column != 6 {
			continue
		}
		if diagnostic.Line == lateFailure.Dialogues[0].Line && diagnostic.FixSafety == SafeFix && diagnostic.FixProof != nil {
			firstProof = true
		}
		if diagnostic.Line == lastLine {
			lateCandidate = true
			lateProof = lateProof || diagnostic.FixSafety != "" || diagnostic.FixProof != nil || diagnostic.FixProofRef != "" || len(diagnostic.Edits) != 0
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
	prefix, suffix, found := strings.CutLast(text, before)
	if !found {
		panic("benchmark dialogue marker not found")
	}
	return prefix + after + suffix
}

func TestBenchmarkLateProofFailure(t *testing.T) {
	const dialogues = 3
	doc := ass.Parse(benchmarkLateProofFailure(dialogues))
	if len(doc.Dialogues) != dialogues {
		t.Fatalf("fixture has %d dialogues, want %d", len(doc.Dialogues), dialogues)
	}
	findings := AnalyzeDocument(doc)
	primary := primaryFindings(findings)
	for i, dialogue := range doc.Dialogues {
		unknown := strings.Contains(dialogue.Text, `\mystery`)
		if unknown != (i == dialogues-1) {
			t.Fatalf("unknown tag is not isolated to the final dialogue: %q", dialogue.Text)
		}
		var noEffects int
		var finalCandidate bool
		for _, diagnostic := range primary {
			if diagnostic.Line != dialogue.Line {
				continue
			}
			if unknown && (diagnostic.FixSafety != "" || diagnostic.FixProof != nil || diagnostic.FixProofRef != "" || len(diagnostic.Edits) != 0) {
				t.Fatalf("unresolved final dialogue offers a fix: %#v", diagnostic)
			}
			if diagnostic.ID != IssueNoEffect {
				continue
			}
			noEffects++
			if diagnostic.Tag == "b" && diagnostic.Column == 6 {
				finalCandidate = true
			} else if diagnostic.Tag != "fs" || (diagnostic.Column != 9 && diagnostic.Column != 14) {
				t.Fatalf("unexpected no-effect candidate: %#v", diagnostic)
			}
			if !unknown && (diagnostic.FixSafety != SafeFix || diagnostic.FixProof == nil || len(diagnostic.Edits) != 1) {
				t.Fatalf("resolved prefix edit lacks proof: %#v", diagnostic)
			}
		}
		if noEffects != 3 || !finalCandidate {
			t.Fatalf("line %d has %d no-effect candidates, repeated bold found=%v", dialogue.Line, noEffects, finalCandidate)
		}
	}
	fixed, count, err := ApplyFixes(doc.Text, findings, false)
	want := strings.Replace(doc.Text, `\b1\b1\fs20\fs20`, `\b1`, dialogues-1)
	if err != nil || count != 3*(dialogues-1) || fixed != want {
		t.Fatalf("safe prefix application: count=%d err=%v, source matches=%v", count, err, fixed == want)
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
