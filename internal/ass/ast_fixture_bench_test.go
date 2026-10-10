package ass

import (
	"testing"

	"assx/internal/perftest"
)

var benchmarkDialogueTrees []DialogueText

// Measures AST construction alone; fixture loading and document parsing are
// outside the timer. Each operation parses all dialogues in the workload.
func BenchmarkParseDialogueTextFixtures(b *testing.B) {
	for _, test := range perftest.Cases(b) {
		doc := Parse(test.Text)
		texts := make([]string, len(doc.Dialogues))
		var bytes int
		for i, dialogue := range doc.Dialogues {
			texts[i] = dialogue.Text
			bytes += len(dialogue.Text)
		}
		b.Run(test.Name, func(b *testing.B) {
			trees := make([]DialogueText, len(texts))
			b.SetBytes(int64(bytes))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for j, text := range texts {
					trees[j] = ParseDialogueText(text)
				}
			}
			benchmarkDialogueTrees = trees
		})
	}
}
