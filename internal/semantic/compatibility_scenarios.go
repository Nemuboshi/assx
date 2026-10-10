package semantic

import (
	"math"
	"slices"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/ass/spec"
)

// Candidate selection discovers relevant comparisons, not evidence of their
// applicability. Each side's domain is checked before certifying its outcome.
func candidateScenarios(observation *Interpretation) []string {
	r, e := observation.Resolution, observation.Event
	var scenarios []string
	if r.EmptyComponents {
		scenarios = append(scenarios, "empty-components")
	}
	if r.Name == "fe" {
		scenarios = append(scenarios, "effect-note")
	}
	if r.Name == "r" {
		scenarios = append(scenarios, "value-map")
		if len(r.Args) > 0 {
			scenarios = append(scenarios, "arg-form")
		}
	}
	if len(r.Args) == 0 || len(r.Args) == 1 && strings.TrimSpace(r.Args[0].Raw) == "" {
		scenarios = append(scenarios, "empty-arg")
	}
	if e.Policy == spec.FirstWins && !e.Applied && len(observation.Owners) > 0 {
		scenarios = append(scenarios, "repeat")
	}
	if !r.HasPolicy {
		return scenarios
	}
	ir := ass.DecodeTagWithSpec(e.Tag, r.Policy, true)
	value := ir.Argument(0)
	if (value.Status == ass.ValueValid || value.Status == ass.ValueAmbiguous) && value.Consumed == len(value.Raw) {
		if value.Number < 0 && r.Name != "fs" && r.Name != "move" {
			scenarios = append(scenarios, "negative")
		}
		switch r.Name {
		case "a":
			if value.Integer == 4 || value.Integer == 8 {
				scenarios = append(scenarios, "value-map")
			}
		case "fn":
			if strings.TrimSpace(value.Raw) == "0" {
				scenarios = append(scenarios, "value-map")
			}
		case "blur":
			if value.Number > 100 {
				scenarios = append(scenarios, "out-of-range")
			}
		case "be":
			if value.Number > 127 {
				scenarios = append(scenarios, "out-of-range")
			}
		case "b":
			if value.Integer != 0 && value.Integer != 1 && value.Integer < 100 {
				scenarios = append(scenarios, "out-of-range")
			}
		case "i", "u", "s":
			if value.Integer != 0 && value.Integer != 1 {
				scenarios = append(scenarios, "out-of-range")
			}
		case "q":
			if value.Integer < 0 || value.Integer > 3 {
				scenarios = append(scenarios, "out-of-range")
			}
		case "an":
			if value.Integer < 1 || value.Integer > 9 {
				scenarios = append(scenarios, "out-of-range")
			}
		}
	}
	if r.Name == "fs" && RelativeFontSize(e.Tag) && (value.Status == ass.ValueValid || value.Status == ass.ValueAmbiguous) && value.Consumed == len(value.Raw) && math.Trunc(value.Number) != value.Number {
		scenarios = append(scenarios, "arg-form")
	}
	if r.Name == "fsc" && len(r.Args) > 0 {
		scenarios = append(scenarios, "arg-form")
	}
	if (r.Name == "clip" || r.Name == "iclip") && len(r.Args) == 4 {
		for i := range r.Args {
			v := ir.Argument(i)
			if (v.Status == ass.ValueValid || v.Status == ass.ValueAmbiguous) && v.Consumed == len(v.Raw) && math.Trunc(v.Number) != math.Trunc(v.Number+0.5) {
				scenarios = append(scenarios, "coord-round")
				break
			}
		}
	}
	if r.Name == "move" && len(r.Args) == 6 {
		t1, t2 := ir.Argument(4), ir.Argument(5)
		if t1.Status == ass.ValueValid && t2.Status == ass.ValueValid && t1.Consumed == len(t1.Raw) && t2.Consumed == len(t2.Raw) && t1.Number > t2.Number {
			scenarios = append(scenarios, "out-of-range")
		}
	}
	slices.Sort(scenarios)
	return slices.Compact(scenarios)
}

// compareScenarioOutcomes compares only domains with a verified semantic
// distinction. Editorial labels such as zero/restored or both-forms/assign-int
// can describe different mechanisms that produce the same invocation result.
func compareScenarioOutcomes(name, scenario string, a, b renderer.BehaviorEvidence) CompatibilityStatus {
	if !a.Verified || !b.Verified {
		return CompatibilityUnresolved
	}
	if a.Outcome == b.Outcome {
		return CompatibilityEquivalent
	}
	switch scenario {
	case "empty-components":
		if name == "pos" {
			return CompatibilityDivergent
		}
	case "coord-round":
		if name == "clip" || name == "iclip" {
			return CompatibilityDivergent
		}
	case "out-of-range":
		if name == "blur" || name == "be" || name == "move" {
			return CompatibilityDivergent
		}
	case "value-map":
		if name == "a" {
			return CompatibilityDivergent
		}
	case "effect-note":
		if name == "fe" {
			return CompatibilityDivergent
		}
	case "arg-form":
		// fs reaches this scenario only for exact fractional relative input;
		// fsc describes argument use versus restoring the Style scales.
		if name == "fs" || name == "fsc" {
			return CompatibilityDivergent
		}
	}
	return CompatibilityUnresolved
}

// Resolution status and scenario evidence deliberately remain independent. A
// conditional build cannot inherit a verified outcome of an enabled branch.
func scenarioEvidence(observation *Interpretation, scenario string, candidates []string) renderer.BehaviorEvidence {
	r, e := observation.Resolution, observation.Event
	evidence := observation.Profile.Behavior(r.Name, scenario)
	applicable := slices.Contains(candidates, scenario) && r.Signature == renderer.SignatureVerified
	switch scenario {
	case "empty-components":
		// Splitting precedes the handler's arity check, including rejection.
		applicable = r.Form == renderer.Paren && r.EmptyComponents
	case "coord-round":
		applicable = rectangleRoundingDomain(observation)
	case "repeat":
		applicable = applicable && e.Ignored && transitionCertain(e)
		for _, owner := range observation.Owners {
			applicable = applicable && owner.End <= r.Source.Start
		}
	case "arg-form":
		if r.Name == "fsc" {
			// The explicit behavior row proves that traditional handlers ignore
			// this argument, even though their signature lists only bare reset.
			applicable = len(r.Args) == 1 && ass.DecodeExactNumber(r.Args[0].Raw).Status == ass.ValueValid &&
				(r.Signature == renderer.SignatureVerified || observation.Profile.Kind() == renderer.Libass || observation.Profile.Kind() == renderer.XYVSFilter)
		}
	case "out-of-range":
		if r.Name == "move" {
			for _, arg := range r.Args {
				applicable = applicable && ass.DecodeExactNumber(arg.Raw).Status == ass.ValueValid
			}
		}
	}
	if r.Status != renderer.Matched || !r.Closed || r.ParametersUnresolved || !applicable {
		evidence.Verified = false
	}
	return evidence
}

func rectangleRoundingDomain(observation *Interpretation) bool {
	r := observation.Resolution
	if (r.Name != "clip" && r.Name != "iclip") || r.Form != renderer.Paren || r.Signature != renderer.SignatureVerified || len(r.Args) != 4 {
		return false
	}
	for _, arg := range r.Args {
		value := ass.DecodeExactNumber(arg.Raw)
		if value.Status != ass.ValueValid || math.Trunc(value.Number) < math.MinInt32 || math.Trunc(value.Number) > math.MaxInt32 {
			return false
		}
		switch observation.Profile.Kind() {
		case renderer.XYVSFilter:
			if n := math.Trunc(value.Number + 0.5); n < math.MinInt32 || n > math.MaxInt32 {
				return false
			}
		case renderer.VSFilterMod:
			// wcstol stops at an exponent; the row proves decimal truncation.
			if strings.ContainsAny(value.Raw, "eE") {
				return false
			}
		}
	}
	return true
}
