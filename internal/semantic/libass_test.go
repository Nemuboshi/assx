package semantic

import (
	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

func evaluateLibass(tree ass.ConcreteDialogue, options EvaluationOptions) Evaluation {
	profile, _ := renderer.Standard(renderer.Libass)
	return EvaluateResolved(tree, profile, options)
}

func evaluateLibassNoEffects(tree ass.ConcreteDialogue) []NoEffect {
	return evaluateLibass(tree, EvaluationOptions{}).NoEffects
}
