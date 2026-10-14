package semantic

import (
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

func TestZZTProbe(t *testing.T) {
	for _, kind := range []renderer.Kind{renderer.Libass, renderer.XYVSFilter} {
		p, err := renderer.Standard(kind)
		if err != nil {
			t.Fatal(err)
		}
		for _, text := range []string{
			`\clip()`,
			`\clip(1,2,3)`,
			`\clip`,
			`\t(18,3438,\clip())`,
			`\t(18,3438,\fs9)`,
			`\t(1,2,\clip(1,2,3,4),4)`,
			`\iclip()`,
		} {
			dlg := ass.Dialogue{Text: "{" + text + "}x", Line: 1}
			tree := ass.ParseConcreteDialogue(dlg.Text)
			obs := observeInterpretation(tree, p, EvaluationOptions{})
			for span, o := range obs {
				e := o.Event
				t.Logf("kind=%v in=%q span=%d name=%q sig=%v paramsUnres=%v applied=%v ignored=%v barrier=%v unc=%v slots=%v status=%v",
					kind, text, span.Start, o.Resolution.Name, o.Resolution.Signature, o.Resolution.ParametersUnresolved,
					e.Applied, e.Ignored, e.Barrier, e.Uncertainty, e.Slots, o.Resolution.Status)
			}
			for _, f := range CompareDialogue(tree, []renderer.Profile{p}, EvaluationOptions{}) {
				_ = f
			}
		}
	}
}
