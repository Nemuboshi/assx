package lint

import (
	"fmt"
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
	for _, count := range perftest.ScalingCounts {
		benchmarkLateProofFailureStage(b, count)
	}
	benchmarkStyleEditStage(b)
}

func benchmarkLateProofFailureStage(b *testing.B, dialogues int) {
	text := benchmarkLateProofFailure(dialogues)
	failure := ass.Parse(text)
	initial := AnalyzeDocument(failure)
	firstProof, lateCandidate, lateProof := false, false, false
	lastLine := failure.Dialogues[len(failure.Dialogues)-1].Line
	var candidates int
	for _, diagnostic := range primaryFindings(initial) {
		if diagnostic.ID != IssueNoEffect {
			continue
		}
		candidates++
		if diagnostic.Tag != "b" || diagnostic.Column != 6 {
			continue
		}
		if diagnostic.Line == failure.Dialogues[0].Line && diagnostic.FixSafety == SafeFix && diagnostic.FixProof != nil {
			firstProof = true
		}
		if diagnostic.Line == lastLine {
			lateCandidate = true
			lateProof = lateProof || diagnostic.FixSafety != "" || diagnostic.FixProof != nil || diagnostic.FixProofRef != "" || len(diagnostic.Edits) != 0
		}
	}
	if !firstProof || !lateCandidate || lateProof {
		b.Fatalf("late failure stage did not isolate the unresolved candidate at %d dialogues", dialogues)
	}
	if candidates != 3*dialogues {
		b.Fatalf("late failure stage found %d candidates, want %d", candidates, 3*dialogues)
	}
	b.Run(fmt.Sprintf("late_candidate_failure_%d", dialogues), func(b *testing.B) {
		b.SetBytes(int64(len(text)))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchmarkDiagnostics = AnalyzeDocument(failure)
		}
		b.ReportMetric(float64(candidates), "cand/op")
	})
}

func benchmarkStyleEditStage(b *testing.B) {
	text := perftest.StyleEdit(b)
	doc := ass.Parse(text)
	findings := AnalyzeDocument(doc)
	var styleProofs, dialogueProofs int
	for _, diagnostic := range primaryFindings(findings) {
		if diagnostic.FixSafety != SafeFix || (diagnostic.FixProof == nil && diagnostic.FixProofRef == "") {
			continue
		}
		switch diagnostic.ID {
		case IssueStyleInteger:
			styleProofs++
		case IssueNoEffect:
			dialogueProofs++
		}
	}
	want := perftest.StyleEditDialogueCount * 3
	if styleProofs != 1 || dialogueProofs != want {
		b.Fatalf("style-edit stage proofs: style=%d dialogue=%d, want 1 and %d", styleProofs, dialogueProofs, want)
	}
	fixed, count, err := ApplyFixes(text, findings, false)
	if err != nil || count != 1+want || strings.Contains(fixed, `,0.0,0,0,1`) {
		b.Fatalf("style-edit stage fixed output: applied=%d err=%v", count, err)
	}
	b.Run(fmt.Sprintf("style_edit_proof_%d", perftest.StyleEditDialogueCount), func(b *testing.B) {
		b.SetBytes(int64(len(text)))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchmarkDiagnostics = AnalyzeDocument(doc)
		}
		b.ReportMetric(float64(1+want), "cand/op")
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
