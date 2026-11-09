package semantic

import (
	"fmt"
	"reflect"
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

// A projection can select a different Text span for each renderer. The set
// must not fold the operations of one text with the program of another.
func TestEvaluatePreparedSetSplitsDifferentTexts(t *testing.T) {
	libass, err := renderer.Standard(renderer.Libass)
	if err != nil {
		t.Fatal(err)
	}
	vsfilter, err := renderer.Standard(renderer.XYVSFilter)
	if err != nil {
		t.Fatal(err)
	}
	preparations := NewDialoguePreparations()
	first := preparations.Prepare(libass, ass.ParseConcreteDialogue(`{\k100}Hi,{\k5}A{\k10}B`))
	second := preparations.Prepare(vsfilter, ass.ParseConcreteDialogue(`{\k5}A{\k10}B`))
	if first.program == second.program {
		t.Fatal("different texts shared one program")
	}
	observe := func(events *[]string) EvaluationOptions {
		return EvaluationOptions{Observer: Observer{
			Tag: func(event TagEvent, state StateView) {
				*events = append(*events, fmt.Sprintf("tag %s@%d cursor=%s", event.Tag.Name, event.Tag.Start, state.Value("karaoke_cursor").Value))
			},
			Text: func(text string, start int, state StateView) {
				*events = append(*events, fmt.Sprintf("text %q@%d cursor=%s", text, start, state.Value("karaoke_cursor").Value))
			},
		}}
	}
	var firstWant, secondWant []string
	individual := []Evaluation{
		first.Evaluate(observe(&firstWant)),
		second.Evaluate(observe(&secondWant)),
	}
	var firstEvents, secondEvents []string
	evaluations := EvaluatePreparedSet(
		[]*PreparedDialogue{first, second},
		[]EvaluationOptions{observe(&firstEvents), observe(&secondEvents)},
	)
	if len(evaluations) != len(individual) {
		t.Fatalf("set evaluations = %d, want %d", len(evaluations), len(individual))
	}
	for i := range evaluations {
		if len(evaluations[i].NoEffects) != len(individual[i].NoEffects) {
			t.Fatalf("entry %d: NoEffects = %d, want %d", i, len(evaluations[i].NoEffects), len(individual[i].NoEffects))
		}
	}
	if !reflect.DeepEqual(firstEvents, firstWant) {
		t.Fatalf("entry 0 events = %v, want %v", firstEvents, firstWant)
	}
	if !reflect.DeepEqual(secondEvents, secondWant) {
		t.Fatalf("entry 1 events = %v, want %v", secondEvents, secondWant)
	}
}
