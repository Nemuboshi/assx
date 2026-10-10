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
		trees := make([]ass.DialogueText, len(doc.Dialogues))
		var bytes int
		for i, dialogue := range doc.Dialogues {
			trees[i] = dialogue.ParsedText()
			bytes += len(dialogue.Text)
		}
		b.Run(test.Name, func(b *testing.B) {
			b.SetBytes(int64(bytes))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, tree := range trees {
					benchmarkEffects = EvaluateDialogue(tree)
				}
			}
		})
	}
}
