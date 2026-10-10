package semantic

import (
	"reflect"
	"strings"
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

func TestCompatibilityProfileOrderInvariant(t *testing.T) {
	profiles := compatibilityProfiles(t)
	for _, source := range []string{`{\clip(1.5,,0,10,10)}A`, `{\iclip(1.5,,0,10,10)}A`, `{\clip(1.5,invalid,10,10)}A`, `{\blur(,)}A`, `{\r(,)}A`, `{\pos(1,2)\pos(3,4)}A`, `{\pos(1,2,3)\pos(4,5)}A`, `{\fs+10.5}A`, `{\fsc(100)}A`} {
		for i := range profiles {
			for j := i + 1; j < len(profiles); j++ {
				t.Run(source+"/"+profiles[i].Kind().String()+"/"+profiles[j].Kind().String(), func(t *testing.T) {
					type key struct {
						span      ass.ConcreteSpan
						dimension string
					}
					collect := func(targets []renderer.Profile) map[key]CompatibilityStatus {
						out := make(map[key]CompatibilityStatus)
						for _, f := range CompareDialogue(ass.ParseConcreteDialogue(source), targets, EvaluationOptions{}) {
							out[key{f.Source, f.Dimension}] = f.Status
						}
						return out
					}
					forward := collect([]renderer.Profile{profiles[i], profiles[j]})
					reverse := collect([]renderer.Profile{profiles[j], profiles[i]})
					if !reflect.DeepEqual(forward, reverse) {
						t.Fatalf("renderer order changed dimensions/statuses:\nforward: %v\nreverse: %v", forward, reverse)
					}
				})
			}
		}
	}
}

func TestCompatibilityRectangleBehaviorRequiresValidDomain(t *testing.T) {
	profiles := []renderer.Profile{mustProfile(t, renderer.XYVSFilter), mustProfile(t, renderer.Libass)}
	for _, source := range []string{`{\clip(1.5,invalid,10,10)}A`, `{\iclip(1.5,0,10junk,10)}A`} {
		t.Run(source, func(t *testing.T) {
			found := false
			for _, f := range CompareDialogue(ass.ParseConcreteDialogue(source), profiles, EvaluationOptions{}) {
				if f.Dimension == "behavior/coord-round" {
					found = true
					if f.Status != CompatibilityUnresolved {
						t.Fatalf("unverified rectangle acquired behavior proof: %#v", f)
					}
				}
			}
			if !found {
				t.Fatal("missing explicit unresolved coordinate behavior")
			}
		})
	}
}

func TestCompatibilityNormalizedRectangleBehavior(t *testing.T) {
	for _, source := range []string{`{\clip(1.5,,0,10,10)}A`, `{\iclip(1.5, ,0,10,10)}A`} {
		found := false
		for _, f := range CompareDialogue(ass.ParseConcreteDialogue(source), nil, EvaluationOptions{}) {
			if f.Dimension == "behavior/coord-round" {
				found = true
				if f.Status != CompatibilityDivergent {
					t.Fatalf("%s: lost valid normalized rectangle: %#v", source, f)
				}
			}
		}
		if !found {
			t.Fatalf("%s: missing rounding comparison", source)
		}
	}
}

func TestCompatibilityCoordinateConversionDomain(t *testing.T) {
	profiles := []renderer.Profile{mustProfile(t, renderer.Libass), mustMod(t, renderer.FeatureEnabled)}
	for _, source := range []string{`{\clip(1.5,1e2,10,10)}A`, `{\iclip(1.5,0,1e300,10)}A`} {
		found := false
		for _, f := range CompareDialogue(ass.ParseConcreteDialogue(source), profiles, EvaluationOptions{}) {
			if f.Dimension == "behavior/coord-round" {
				found = true
				if f.Status != CompatibilityUnresolved {
					t.Fatalf("%s: unsupported conversion inherited proof: %#v", source, f)
				}
			}
		}
		if !found {
			t.Fatalf("%s: missing unresolved conversion", source)
		}
	}
	for _, f := range CompareDialogue(ass.ParseConcreteDialogue(`{\clip(1.5,0,10,10,20)}A`), nil, EvaluationOptions{}) {
		if f.Dimension == "signature" && f.Status != CompatibilityUnresolved {
			t.Fatalf("unverified arity acquired proof: %#v", f)
		}
		if f.Dimension == "behavior/coord-round" && f.Status != CompatibilityUnresolved {
			t.Fatalf("unverified arity acquired rounding proof: %#v", f)
		}
	}
}

func TestCompatibilityParserNormalizationIndependentOfAcceptance(t *testing.T) {
	for _, f := range CompareDialogue(ass.ParseConcreteDialogue(`{\pos(1,,2,3)}A`), nil, EvaluationOptions{}) {
		if f.Dimension == "behavior/empty-components" {
			if f.Status != CompatibilityEquivalent {
				t.Fatalf("lost parser normalization proof: %#v", f)
			}
			return
		}
	}
	t.Fatal("missing parser normalization comparison")
}

func TestLibassRetainsUnicodeWhitespaceArguments(t *testing.T) {
	profile := mustProfile(t, renderer.Libass)
	for _, whitespace := range []string{"\u00a0", "\u2003"} {
		source := "{\\pos(1," + whitespace + ",2)\\pos(4,5)}A"
		t.Run(source, func(t *testing.T) {
			var after StateView
			EvaluateResolved(ass.ParseConcreteDialogue(source), profile, EvaluationOptions{
				Observer: Observer{Text: func(text string, _ int, state StateView) {
					if text == "A" {
						after = state
					}
				}},
			})
			if owner := after.Source("position"); owner != 1 {
				t.Fatalf("second valid position did not own state: owner=%d want=1", owner)
			}
		})
	}
}

func TestCompatibilityUnicodeWhitespaceKeepsRendererOwnershipSeparate(t *testing.T) {
	profiles := []renderer.Profile{mustProfile(t, renderer.Libass), mustProfile(t, renderer.XYVSFilter)}
	for _, whitespace := range []string{"\u00a0", "\u2003"} {
		source := "{\\pos(1," + whitespace + ",2)\\pos(4,5)}A"
		found := false
		for _, finding := range CompareDialogue(ass.ParseConcreteDialogue(source), profiles, EvaluationOptions{}) {
			if finding.Dimension != "ownership" || finding.Source.Start != strings.LastIndex(source, `\pos`) {
				continue
			}
			found = true
			if finding.Status != CompatibilityDivergent {
				t.Fatalf("Unicode whitespace did not preserve renderer ownership difference: %#v", finding)
			}
		}
		if !found {
			t.Fatalf("missing ownership comparison for %q", source)
		}
	}
}

func TestCompatibilityRepeatBehaviorRequiresOccupiedLatch(t *testing.T) {
	profiles := []renderer.Profile{mustMod(t, renderer.FeatureEnabled), mustProfile(t, renderer.Libass)}
	for _, f := range CompareDialogue(ass.ParseConcreteDialogue(`{\pos(1,2,3)\pos(4,5)}A`), profiles, EvaluationOptions{}) {
		if f.Source.Start == 12 && f.Dimension == "behavior/repeat" {
			if f.Status != CompatibilityUnresolved {
				t.Fatalf("unoccupied latch inherited repeat outcome: %#v", f)
			}
			return
		}
	}
	t.Fatal("missing repeat comparison")
}

func TestCompatibilityVerifiedApplicationObservations(t *testing.T) {
	for _, source := range []string{`{\fs20}A`, `{\bord2}A`, `{\pos(1,2)}A`, `{\pos(1,2)\pos(3,4)}A`, `{\pos(1,2,3)}A`} {
		t.Run(source, func(t *testing.T) {
			found := false
			for _, f := range CompareDialogue(ass.ParseConcreteDialogue(source), nil, EvaluationOptions{}) {
				if f.Dimension == "application" {
					found = true
					if f.Status != CompatibilityEquivalent {
						t.Fatalf("verified observation not comparable: %#v", f)
					}
				}
				if f.Dimension == "state" && f.Status == CompatibilityEquivalent {
					t.Fatal("tracked state became rendering equivalence")
				}
			}
			if !found {
				t.Fatal("missing application observation")
			}
		})
	}
	for _, source := range []string{`{\fs20junk}A`, `{\clip(1,2,3)}A`, `{\t(0,500,\fs30)}A`} {
		for _, f := range CompareDialogue(ass.ParseConcreteDialogue(source), nil, EvaluationOptions{}) {
			if f.Dimension == "application" && f.Source.Start == 1 && f.Status != CompatibilityUnresolved {
				t.Fatalf("%s: uncertain invocation gained application proof: %#v", source, f)
			}
		}
	}
}
