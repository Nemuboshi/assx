package semantic

import (
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

func TestEvaluatePreparedSetSharesProgram(t *testing.T) {
	tree := ass.ParseConcreteDialogue(`{\fs20\fs20}Hi`)
	libass, err := renderer.Standard(renderer.Libass)
	if err != nil {
		t.Fatal(err)
	}
	vsfilter, err := renderer.Standard(renderer.XYVSFilter)
	if err != nil {
		t.Fatal(err)
	}
	preparations := NewDialoguePreparations()
	first := preparations.Prepare(libass, tree)
	second := preparations.Prepare(vsfilter, tree)
	if first == second || first.program != second.program {
		t.Fatal("profiles did not share the neutral program")
	}
	evaluations := EvaluatePreparedSet([]*PreparedDialogue{first, second}, []EvaluationOptions{{}, {}})
	if len(evaluations) != 2 {
		t.Fatalf("set evaluation = %v", len(evaluations))
	}
	if len(evaluations[0].NoEffects) != len(evaluations[1].NoEffects) {
		t.Fatalf("states disagree: %d vs %d", len(evaluations[0].NoEffects), len(evaluations[1].NoEffects))
	}
}
