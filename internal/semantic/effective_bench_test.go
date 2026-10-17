package semantic

import (
	"testing"

	"assx/internal/ass"
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
