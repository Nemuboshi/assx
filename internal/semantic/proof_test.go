package semantic

import (
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

func TestCompareResolvedProofPerPinnedRenderer(t *testing.T) {
	profiles := []renderer.Profile{
		mustProofProfile(t, renderer.Libass),
		mustProofProfile(t, renderer.XYVSFilter),
		mustProofMod(t),
	}
	before := `{\fs20\fs20}A`
	after := `{\fs20}A`
	for _, profile := range profiles {
		t.Run(profile.Kind().String(), func(t *testing.T) {
			span := resolvedTagSpan(t, profile, before, "fs", 1)
			comparison := CompareResolvedProof(
				ass.ParseConcreteDialogue(before), ass.ParseConcreteDialogue(after),
				profile, EvaluationOptions{}, []ass.ConcreteSpan{span},
			)
			if !comparison.Equivalent {
				t.Fatalf("redundant assignment proof failed: %s", comparison.Reason)
			}
			if len(comparison.Before.Steps) == 0 || len(comparison.After.Steps) == 0 {
				t.Fatal("proof omitted original or edited interpretations")
			}
			for _, step := range comparison.Before.Steps {
				if step.Kind == "tag" && step.Citation == "" {
					t.Fatalf("proof omitted pinned source citation: %#v", step)
				}
			}
		})
	}
}

func TestCompareResolvedProofRejectsChangedFirstWinsOwner(t *testing.T) {
	profile := mustProofProfile(t, renderer.Libass)
	before := `{\an7\an8}A`
	span := resolvedTagSpan(t, profile, before, "an", 0)
	comparison := CompareResolvedProof(
		ass.ParseConcreteDialogue(before), ass.ParseConcreteDialogue(`{\an8}A`),
		profile, EvaluationOptions{}, []ass.ConcreteSpan{span},
	)
	if comparison.Equivalent {
		t.Fatal("proof accepted removal of the first-wins owner")
	}
}

func TestCompareResolvedProofRejectsUnknownCommand(t *testing.T) {
	profile := mustProofProfile(t, renderer.Libass)
	before := `{\fs20\fs20\mystery}A`
	span := resolvedTagSpan(t, profile, before, "fs", 1)
	comparison := CompareResolvedProof(
		ass.ParseConcreteDialogue(before), ass.ParseConcreteDialogue(`{\fs20\mystery}A`),
		profile, EvaluationOptions{}, []ass.ConcreteSpan{span},
	)
	if comparison.Equivalent || comparison.Reason == "" {
		t.Fatal("proof accepted an edit in a dialogue with an unknown command")
	}
}

func TestCompareResolvedProofRejectsRendererPrefixCollision(t *testing.T) {
	profiles := []renderer.Profile{
		mustProofProfile(t, renderer.Libass),
		mustProofProfile(t, renderer.XYVSFilter),
		mustProofMod(t),
	}
	before := `{\fsvp6}A`
	for _, profile := range profiles {
		t.Run(profile.Kind().String(), func(t *testing.T) {
			name := "fs"
			if profile.Kind() == renderer.VSFilterMod {
				name = "fsvp"
			}
			span := resolvedTagSpan(t, profile, before, name, 0)
			comparison := CompareResolvedProof(
				ass.ParseConcreteDialogue(before), ass.ParseConcreteDialogue("A"),
				profile, EvaluationOptions{}, []ass.ConcreteSpan{span},
			)
			if comparison.Equivalent {
				t.Fatal("proof accepted removal of a renderer-prefix collision")
			}
		})
	}
}

func mustProofProfile(t *testing.T, kind renderer.Kind) renderer.Profile {
	t.Helper()
	profile, err := renderer.Standard(kind)
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func mustProofMod(t *testing.T) renderer.Profile {
	t.Helper()
	profile, err := renderer.New(renderer.VSFilterMod, renderer.Build{
		Mod: renderer.FeatureEnabled, Lua: renderer.FeatureDisabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func resolvedTagSpan(t *testing.T, profile renderer.Profile, source, name string, occurrence int) ass.ConcreteSpan {
	t.Helper()
	var spans []ass.ConcreteSpan
	profile.WalkDialogue(ass.ParseConcreteDialogue(source), func(result renderer.Result, _ bool) bool {
		if result.Name == name {
			spans = append(spans, result.Source)
		}
		return true
	})
	if occurrence < 0 || occurrence >= len(spans) {
		t.Fatalf("%s occurrence %d not found in %q: %#v", name, occurrence, source, spans)
	}
	return spans[occurrence]
}
