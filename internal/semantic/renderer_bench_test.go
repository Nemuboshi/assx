package semantic

import (
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/perftest"
)

var benchmarkRendererEvaluation Evaluation

// Parses the neutral CST once and measures only renderer resolution and
// effective-state evaluation. Compare with BenchmarkEvaluateDialogue for the
// frozen default view; both use the same state-transition engine.
func BenchmarkEvaluateResolved(b *testing.B) {
	for _, fixture := range perftest.Cases(b) {
		doc := ass.Parse(fixture.Text)
		trees := make([]ass.ConcreteDialogue, len(doc.Dialogues))
		var bytes int
		for i, dialogue := range doc.Dialogues {
			trees[i] = ass.ParseConcreteDialogue(dialogue.Text)
			bytes += len(dialogue.Text)
		}
		for _, kind := range []renderer.Kind{renderer.Libass, renderer.XYVSFilter} {
			profile, err := renderer.Standard(kind)
			if err != nil {
				b.Fatal(err)
			}
			b.Run(fixture.Name+"/"+kind.String(), func(b *testing.B) {
				b.SetBytes(int64(bytes))
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					for _, tree := range trees {
						benchmarkRendererEvaluation = EvaluateResolved(tree, profile, EvaluationOptions{})
					}
				}
			})
		}
	}
}
