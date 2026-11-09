package renderer

import (
	"testing"

	"assx/internal/ass"

	"assx/internal/perftest"
)

var benchmarkDialogueTrees []ass.ConcreteDialogue

// Measures AST construction alone; fixture loading and document parsing are
// outside the timer. Each operation parses all dialogues in the workload.
func BenchmarkParseConcreteDialogueFixtures(b *testing.B) {
	for _, test := range perftest.Cases(b) {
		doc := Parse(test.Text)
		texts := make([]string, len(doc.Dialogues))
		var bytes int
		for i, dialogue := range doc.Dialogues {
			texts[i] = dialogue.Text
			bytes += len(dialogue.Text)
		}
		b.Run(test.Name, func(b *testing.B) {
			trees := make([]ass.ConcreteDialogue, len(texts))
			b.SetBytes(int64(bytes))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for j, text := range texts {
					trees[j] = ass.ParseConcreteDialogue(text)
				}
			}
			benchmarkDialogueTrees = trees
		})
	}
}
