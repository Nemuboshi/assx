package semantic

import (
	"slices"
	"strings"
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

func compatibilityProfiles(t *testing.T) []renderer.Profile {
	t.Helper()
	return []renderer.Profile{mustProfile(t, renderer.Libass), mustProfile(t, renderer.XYVSFilter), mustMod(t, renderer.FeatureEnabled)}
}

func TestCompatibilityIndependentInterpretations(t *testing.T) {
	tests := []struct {
		source, dimension   string
		start               int
		status              CompatibilityStatus
		left, right         renderer.Kind
		leftName, rightName string
	}{
		{`{\fsvp6}A`, "dispatch", 1, CompatibilityDivergent, renderer.Libass, renderer.VSFilterMod, "fs", "fsvp"},
		{`{\frs10}A`, "dispatch", 1, CompatibilityDivergent, renderer.XYVSFilter, renderer.VSFilterMod, "fr", "frs"},
		{`{\blend(add)}A`, "dispatch", 1, CompatibilityDivergent, renderer.Libass, renderer.VSFilterMod, "b", "blend"},
		{`{\pos(1,2,3)}A`, "signature", 1, CompatibilityDivergent, renderer.Libass, renderer.VSFilterMod, "pos", "pos"},
		{`{\pos(1,2,3)\pos(4,5)}A`, "ownership", 12, CompatibilityDivergent, renderer.Libass, renderer.VSFilterMod, "pos", "pos"},
		{`{\pos(1,2,3)\pos(4,5)}A`, "state", 12, CompatibilityDivergent, renderer.Libass, renderer.VSFilterMod, "pos", "pos"},
		{`{\pos(1,,2)\pos(4,5)}A`, "arguments", 1, CompatibilityDivergent, renderer.Libass, renderer.XYVSFilter, "pos", "pos"},
		{`{\pos(1,,2)\pos(4,5)}A`, "ownership", 11, CompatibilityDivergent, renderer.Libass, renderer.XYVSFilter, "pos", "pos"},
	}
	for _, tt := range tests {
		t.Run(tt.source+"/"+tt.dimension+"/"+tt.left.String(), func(t *testing.T) {
			findings := CompareDialogue(ass.ParseConcreteDialogue(tt.source), compatibilityProfiles(t), EvaluationOptions{})
			var found bool
			for _, f := range findings {
				if f.Source.Start == tt.start && f.Dimension == tt.dimension && f.Left.Profile.Kind() == tt.left && f.Right.Profile.Kind() == tt.right {
					found = true
					if f.Status != tt.status || f.Left.Resolution.Name != tt.leftName || f.Right.Resolution.Name != tt.rightName {
						t.Errorf("unexpected comparison: %#v", f)
					}
					if !strings.Contains(f.Detail, tt.left.String()) || !strings.Contains(f.Detail, tt.right.String()) {
						t.Errorf("missing both explanations: %s", f.Detail)
					}
				}
			}
			if !found {
				t.Fatalf("missing %s at %d: %#v", tt.dimension, tt.start, findings)
			}
		})
	}
}

func TestCompatibilityEvidenceNeverInventsEquivalence(t *testing.T) {
	for _, source := range []string{`{\fs20}A`, `{\alpha&H80&}A`, `{\t(0,500,\fs30)}A`, `{\rMissing}A`, `{\clip(1,2,3)}A`, `{\k10}A`, `{\p1}m 0 0 l 1 1`, `{\fs20\mystery}A`} {
		findings := CompareDialogue(ass.ParseConcreteDialogue(source), compatibilityProfiles(t), EvaluationOptions{})
		unresolved := false
		for _, f := range findings {
			if f.Status == CompatibilityUnresolved {
				unresolved = true
			}
			if f.Dimension == "state" && f.Status == CompatibilityEquivalent {
				t.Fatalf("%s: tracked state was promoted to rendering equivalence", source)
			}
		}
		if !unresolved {
			t.Fatalf("%s: missing explicit unresolved result", source)
		}
	}
	unknown := mustMod(t, renderer.FeatureUnknown)
	findings := CompareDialogue(ass.ParseConcreteDialogue(`{\fsvp6\pos(1,2,3)}A`), []renderer.Profile{mustProfile(t, renderer.Libass), unknown}, EvaluationOptions{})
	for _, f := range findings {
		if f.Dimension == "dispatch" && f.Left.Resolution.Head == "fsvp6" && f.Status != CompatibilityUnresolved {
			t.Fatalf("unknown build acquired a proof: %#v", f)
		}
	}
}

func TestCompatibilityVerifiedScenarios(t *testing.T) {
	tests := []struct {
		source, dimension string
		status            CompatibilityStatus
	}{
		{`{\blur101}A`, "behavior/out-of-range", CompatibilityDivergent},
		{`{\blur}A`, "behavior/empty-arg", CompatibilityUnresolved},
		{`{\a4}A`, "behavior/value-map", CompatibilityDivergent},
		{`{\clip(1.5,0,10,10)}A`, "behavior/coord-round", CompatibilityDivergent},
		{`{\fe1}A`, "behavior/effect-note", CompatibilityDivergent},
		{`{\move(0,0,10,10,500,100)}A`, "behavior/out-of-range", CompatibilityDivergent},
		{`{\frz}A`, "behavior/empty-arg", CompatibilityEquivalent},
		{`{\be200}A`, "behavior/out-of-range", CompatibilityUnresolved},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			findings := CompareDialogue(ass.ParseConcreteDialogue(tt.source), nil, EvaluationOptions{})
			for _, f := range findings {
				if f.Dimension == tt.dimension {
					if f.Status != tt.status {
						t.Fatalf("got %s, want %s: %#v", f.Status, tt.status, f)
					}
					if f.Status != CompatibilityUnresolved && len(f.Citations) == 0 {
						t.Fatal("proof without citation")
					}
					return
				}
			}
			t.Fatalf("missing scenario %s", tt.dimension)
		})
	}
}

func TestCompatibilityStableDetachedSourceObservations(t *testing.T) {
	source := `{\pos(4,5)}A{\pos(6,7)}B`
	profiles := compatibilityProfiles(t)
	tree := ass.ParseConcreteDialogue(source)
	findings := CompareDialogue(tree, profiles, EvaluationOptions{Observer: Observer{Tag: func(TagEvent, StateView) { t.Fatal("comparison called caller observer") }}})
	if tree.Source != source {
		t.Fatal("source changed")
	}
	for _, f := range findings {
		if f.Left.Present && source[f.Source.Start:f.Source.End] != f.Left.Resolution.Raw {
			t.Errorf("lost original span: %#v", f)
		}
		if f.Dimension == "ownership" && f.Source.Start > 1 {
			for _, o := range []*Interpretation{f.Left, f.Right} {
				owner, ok := o.Owners["position"]
				if !ok || owner.Start != 1 {
					t.Errorf("owner is a profile-local index or borrowed map: %#v", o.Owners)
				}
			}
		}
	}
	again := CompareDialogue(tree, append(profiles, profiles[0]), EvaluationOptions{})
	if len(findings) != len(again) {
		t.Fatal("duplicate target repeated findings")
	}
	// Default scope and explicitly requested traditional profiles agree.
	defaults := CompareDialogue(tree, nil, EvaluationOptions{})
	explicit := CompareDialogue(tree, profiles[:2], EvaluationOptions{})
	if !slices.EqualFunc(defaults, explicit, func(a, b CompatibilityFinding) bool {
		return a.Dimension == b.Dimension && a.Status == b.Status && a.Detail == b.Detail
	}) {
		t.Fatal("wrong default compatibility scope")
	}
}
