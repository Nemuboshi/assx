package semantic

import (
	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"testing"
)

func TestCompatibilityCoreBehaviorBuildGuards(t *testing.T) {
	for _, feature := range []renderer.Feature{renderer.FeatureEnabled, renderer.FeatureDisabled, renderer.FeatureUnknown} {
		profiles := []renderer.Profile{mustProfile(t, renderer.Libass), mustMod(t, feature)}
		findings := CompareDialogue(ass.ParseConcreteDialogue(`{\fsc(100)}A`), profiles, EvaluationOptions{})
		found := false
		for _, finding := range findings {
			if finding.Dimension == "behavior/arg-form" {
				found = true
				want := CompatibilityUnresolved
				if feature == renderer.FeatureEnabled {
					want = CompatibilityDivergent
				}
				if finding.Status != want {
					t.Fatalf("feature %d: got %s, want %s", feature, finding.Status, want)
				}
			}
		}
		if !found {
			t.Fatalf("feature %d: missing guarded behavior", feature)
		}
	}
}

func TestCompatibilityMechanismLabelsDoNotProveInvocationDifference(t *testing.T) {
	profiles := compatibilityProfiles(t)
	for _, source := range []string{`{\frx}A`, `{\fry}A`, `{\fax}A`, `{\fay}A`, `{\fs+10}A`, `{\fs-10}A`, `{\blur}A`, `{\fs+10junk}A`} {
		for _, finding := range CompareDialogue(ass.ParseConcreteDialogue(source), profiles, EvaluationOptions{}) {
			if len(finding.Dimension) >= 9 && finding.Dimension[:9] == "behavior/" && finding.Status == CompatibilityDivergent {
				t.Fatalf("%s: mechanism labels became a proved result difference: %#v", source, finding)
			}
		}
	}
	// An exact fractional delta really is consumed differently by the Mod
	// integer parser, even though pixel effects still depend on state/context.
	found := false
	for _, finding := range CompareDialogue(ass.ParseConcreteDialogue(`{\fs+10.5}A`), profiles, EvaluationOptions{}) {
		if finding.Dimension == "behavior/arg-form" && finding.Left.Profile.Kind() == renderer.Libass && finding.Right.Profile.Kind() == renderer.VSFilterMod {
			found = true
			if finding.Status != CompatibilityDivergent {
				t.Fatalf("lost verified fractional parsing difference: %#v", finding)
			}
		}
	}
	if !found {
		t.Fatal("missing fractional argument comparison")
	}
}

func TestCompatibilitySharesDetachedObservationsAcrossDimensions(t *testing.T) {
	findings := CompareDialogue(ass.ParseConcreteDialogue(`{\pos(1,2)}A`), nil, EvaluationOptions{})
	if len(findings) < 2 {
		t.Fatal("missing dimensions")
	}
	for _, finding := range findings[1:] {
		if finding.Left != findings[0].Left || finding.Right != findings[0].Right {
			t.Fatal("duplicated a large interpretation snapshot for each dimension")
		}
	}
	if findings := CompareDialogue(ass.ParseConcreteDialogue("Plain text"), nil, EvaluationOptions{}); len(findings) != 0 {
		t.Fatal("plain text produced override findings")
	}
}

func TestCompatibilityNestedParameterConsumptionRemainsUnresolved(t *testing.T) {
	source := `{\t(0,500,\t(0,250,\fs30))}A`
	for _, finding := range CompareDialogue(ass.ParseConcreteDialogue(source), compatibilityProfiles(t), EvaluationOptions{}) {
		if finding.Source.Start == 1 && (finding.Dimension == "arguments" || finding.Dimension == "signature") && finding.Status != CompatibilityUnresolved {
			t.Fatalf("nested parameter arity became a proof: %#v", finding)
		}
	}
}
