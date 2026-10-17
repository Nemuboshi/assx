package semantic

import (
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/perftest"
)

var benchmarkEffects []NoEffect

// Isolates the semantic effective-state engine from ASS and AST parsing.
func BenchmarkEvaluateDialogue(b *testing.B) {
	for _, test := range perftest.Cases(b) {
		doc := ass.Parse(test.Text)
		trees := make([]ass.ConcreteDialogue, len(doc.Dialogues))
		var bytes int
		for i, dialogue := range doc.Dialogues {
			trees[i] = ass.ParseConcreteDialogue(dialogue.Text)
			bytes += len(dialogue.Text)
		}
		benchmarkEffects = nil
		for _, tree := range trees {
			benchmarkEffects = append(benchmarkEffects, evaluateLibassNoEffects(tree)...)
		}
		candidates := len(benchmarkEffects)
		b.Run(test.Name, func(b *testing.B) {
			b.SetBytes(int64(bytes))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for _, tree := range trees {
					benchmarkEffects = evaluateLibassNoEffects(tree)
				}
			}
			b.ReportMetric(float64(candidates), "cand/op")
		})
	}
}

// BenchmarkCompareResolvedProof measures both authorization outcomes on real
// fixtures: an accepted comparison materializes the audit wire for the
// caller, and a rejected comparison must materialize nothing.
func BenchmarkCompareResolvedProof(b *testing.B) {
	profile, err := renderer.Standard(renderer.Libass)
	if err != nil {
		b.Fatal(err)
	}
	options := EvaluationOptions{Styles: compactCorpusStyles(), DialogueStyle: "Default"}
	for _, test := range perftest.Cases(b) {
		doc := ass.Parse(test.Text)
		if len(doc.Dialogues) > 50 {
			doc.Dialogues = doc.Dialogues[:50]
		}
		trees := make([]ass.ConcreteDialogue, len(doc.Dialogues))
		edits := make([]ass.ConcreteDialogue, len(doc.Dialogues))
		removed := make([][]ass.ConcreteSpan, len(doc.Dialogues))
		for i, dialogue := range doc.Dialogues {
			trees[i] = ass.ParseConcreteDialogue(dialogue.Text)
			for _, node := range trees[i].Nodes {
				if node.Block != nil && len(node.Block.Content) != 0 && len(removed[i]) == 0 {
					removed[i] = []ass.ConcreteSpan{node.Block.Span}
					start, end := node.Block.Span.Start, node.Block.Span.End
					edited := dialogue.Text[:start] + dialogue.Text[end:]
					edits[i] = ass.ParseConcreteDialogue(edited)
				}
			}
		}
		b.Run(test.Name+"/self", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for _, tree := range trees {
					benchmarkProofResult = CompareResolvedProof(tree, tree, profile, options, nil)
				}
			}
		})
		b.Run(test.Name+"/edited", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for i, tree := range trees {
					if len(removed[i]) == 0 {
						continue
					}
					benchmarkProofResult = CompareResolvedProof(tree, edits[i], profile, options, removed[i])
				}
			}
		})
	}
}

var benchmarkProofResult ProofComparison
