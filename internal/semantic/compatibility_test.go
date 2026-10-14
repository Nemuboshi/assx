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
		{`{\pos(1,2,3)\pos(4,5)}A`, "state", 12, CompatibilityUnresolved, renderer.Libass, renderer.VSFilterMod, "pos", "pos"},
		{`{\pos(1,,2)\pos(4,5)}A`, "arguments", 1, CompatibilityEquivalent, renderer.Libass, renderer.XYVSFilter, "pos", "pos"},
		{`{\pos(1,,2)\pos(4,5)}A`, "ownership", 11, CompatibilityEquivalent, renderer.Libass, renderer.XYVSFilter, "pos", "pos"},
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
	// Tracked state may only reach equivalence through the same verified gate
	// as application. A profile pair that cannot prove a certain transition
	// must not gain state equivalence, and equivalence must never hide a known
	// difference. This preserves the partial-engine guard while allowing a
	// verified equal observation to stop re-reporting per tag.
	for _, source := range []string{`{\fs20}A`, `{\alpha&H80&}A`, `{\t(0,500,\fs30)}A`, `{\rMissing}A`, `{\clip(1,2,3)}A`, `{\k10}A`, `{\p1}m 0 0 l 1 1`, `{\fs20\mystery}A`} {
		findings := CompareDialogue(ass.ParseConcreteDialogue(source), compatibilityProfiles(t), EvaluationOptions{})
		applications := make(map[[3]any]CompatibilityStatus)
		for _, f := range findings {
			key := [3]any{f.Source.Start, f.Source.End, profilePair(f)}
			if f.Dimension == "application" {
				applications[key] = f.Status
			}
		}
		for _, f := range findings {
			if f.Dimension != "state" {
				continue
			}
			if f.Status == CompatibilityEquivalent && applications[[3]any{f.Source.Start, f.Source.End, profilePair(f)}] != CompatibilityEquivalent {
				t.Fatalf("%s: state equivalence without application proof: %#v", source, f)
			}
			if f.Status == CompatibilityEquivalent && knownStateDifference(f.Left.Event, f.Right.Event) {
				t.Fatalf("%s: state equivalence hides a known difference: %#v", source, f)
			}
		}
	}
	unknown := mustMod(t, renderer.FeatureUnknown)
	findings := CompareDialogue(ass.ParseConcreteDialogue(`{\fsvp6\pos(1,2,3)}A`), []renderer.Profile{mustProfile(t, renderer.Libass), unknown}, EvaluationOptions{})
	for _, f := range findings {
		if f.Dimension == "dispatch" && f.Left.Resolution.Head == "fsvp6" && f.Status != CompatibilityUnresolved {
			t.Fatalf("unknown build acquired a proof: %#v", f)
		}
		if f.Dimension == "state" && f.Status == CompatibilityEquivalent {
			t.Fatalf("unknown build invented state equivalence: %#v", f)
		}
	}
}

// profilePair keys an observation to its unordered profile pair.
func profilePair(f CompatibilityFinding) [2]string {
	left, right := f.Left.Profile.Kind().String(), f.Right.Profile.Kind().String()
	if left > right {
		left, right = right, left
	}
	return [2]string{left, right}
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
