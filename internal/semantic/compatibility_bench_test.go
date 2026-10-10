package semantic

import (
	"assx/internal/ass"
	"assx/internal/perftest"
	"testing"
)

var benchmarkCompatibilityFindings []CompatibilityFinding

func BenchmarkCompareDialogue(b *testing.B) {
	for _, fixture := range perftest.Cases(b) {
		document := ass.Parse(fixture.Text)
		trees := make([]ass.ConcreteDialogue, len(document.Dialogues))
		var size int
		for i, dialogue := range document.Dialogues {
			trees[i] = ass.ParseConcreteDialogue(dialogue.Text)
			size += len(dialogue.Text)
		}
		b.Run(fixture.Name, func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				for _, tree := range trees {
					benchmarkCompatibilityFindings = CompareDialogue(tree, nil, EvaluationOptions{})
				}
			}
		})
	}
}
