package lint

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"assx/internal/ass/renderer"
)

func TestSafeFixRequestedTargetScope(t *testing.T) {
	libass, _ := renderer.Standard(renderer.Libass)
	xy, _ := renderer.Standard(renderer.XYVSFilter)
	mod, _ := renderer.New(renderer.VSFilterMod, renderer.Build{Mod: renderer.FeatureEnabled, Lua: renderer.FeatureDisabled})
	doc := parseFontOverrideText(`{\fs21\fs21}A`)
	cases := []AnalysisTargets{
		{Primary: libass}, {Primary: xy}, {Primary: mod},
		{Primary: libass, Compatibility: []renderer.Profile{xy, libass, xy}},
		{Primary: xy, Compatibility: []renderer.Profile{mod, libass, mod}},
	}
	for _, targets := range cases {
		t.Run(ProfileScope(targets.Primary), func(t *testing.T) {
			ds := AnalyzeDocumentForTargets(doc, targets)
			safe, _, _ := CountFixes(ds)
			if safe == 0 {
				t.Fatal("no scoped fix")
			}
			fixed, count, err := ApplyFixesForTargets(doc.Text, ds, false, targets)
			if err != nil || count == 0 || !strings.Contains(fixed, `{\fs21}A`) {
				t.Fatalf("apply: count=%d err=%v", count, err)
			}
			wider := AnalysisTargets{Primary: libass, Compatibility: []renderer.Profile{xy, mod}}
			if len(targets.Profiles()) < len(wider.Profiles()) {
				if _, _, err := ApplyFixesForTargets(doc.Text, ds, false, wider); err == nil {
					t.Fatal("subset proof authorized wider targets")
				}
			}
			if targets.Primary == mod {
				if _, _, err := ApplyFixes(doc.Text, ds, false); err == nil {
					t.Fatal("Mod proof authorized default scope")
				}
			}
		})
	}
}

func TestSafeFixRejectsRehashedScopeTampering(t *testing.T) {
	doc := parseFontOverrideText(`{\fs21\fs21}A`)
	targets := DefaultAnalysisTargets()
	for _, mutation := range []struct {
		name   string
		change func(*FixProof)
	}{
		{"version", func(p *FixProof) { p.Targets[0].Version = "stale" }},
		{"capability", func(p *FixProof) { p.Targets[0].Capabilities.Lua = "disabled" }},
		{"target subset", func(p *FixProof) { p.Targets = p.Targets[:1] }},
		{"edited hash", func(p *FixProof) { p.EditedSHA256 = hashSource("other") }},
		{"interpretations", func(p *FixProof) { p.Interpretations = nil }},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			ds := AnalyzeDocumentForTargets(doc, targets)
			var proof *FixProof
			for _, d := range ds {
				if d.FixProof != nil {
					proof = d.FixProof
					break
				}
			}
			if proof == nil {
				t.Fatal("no proof")
			}
			mutation.change(proof)
			proof.ID = ""
			id, err := hashJSON(proof)
			if err != nil {
				t.Fatal(err)
			}
			proof.ID = id
			for i := range ds {
				if ds[i].FixProofRef != "" {
					ds[i].FixProofRef = id
				}
			}
			if _, _, err := ApplyFixesForTargets(doc.Text, ds, false, targets); err == nil {
				t.Fatal("rehashed forgery authorized a fix")
			}
		})
	}
}

func TestSafeFixModCapabilityMismatchAndUncertainty(t *testing.T) {
	known, _ := renderer.New(renderer.VSFilterMod, renderer.Build{Mod: renderer.FeatureEnabled, Lua: renderer.FeatureDisabled})
	doc := parseFontOverrideText(`{\fs21\fs21}A`)
	ds := AnalyzeDocumentForTargets(doc, AnalysisTargets{Primary: known})
	for _, build := range []renderer.Build{
		{}, {Mod: renderer.FeatureEnabled},
		{Mod: renderer.FeatureEnabled, Lua: renderer.FeatureEnabled},
		{Mod: renderer.FeatureDisabled, Lua: renderer.FeatureDisabled},
	} {
		mod, err := renderer.New(renderer.VSFilterMod, build)
		if err != nil {
			t.Fatal(err)
		}
		targets := AnalysisTargets{Primary: mod}
		if _, _, err := ApplyFixesForTargets(doc.Text, ds, false, targets); err == nil {
			t.Fatalf("proof accepted different capability %v", build)
		}
		current := AnalyzeDocumentForTargets(doc, targets)
		if build.Mod == renderer.FeatureUnknown || build.Lua != renderer.FeatureDisabled {
			safe, _, _ := CountFixes(current)
			if safe != 0 {
				t.Fatalf("uncertain build granted %d fixes", safe)
			}
			fixed, count, err := ApplyFixesForTargets(doc.Text, current, false, targets)
			if err != nil || count != 0 || fixed != doc.Text {
				t.Fatal("uncertain build changed source")
			}
		}
	}
}

func TestTargetNormalizationIsDeterministic(t *testing.T) {
	base := DefaultAnalysisTargets()
	modUnknown, _ := renderer.Standard(renderer.VSFilterMod)
	modKnown, _ := renderer.New(renderer.VSFilterMod, renderer.Build{Mod: renderer.FeatureEnabled, Lua: renderer.FeatureDisabled})
	a := AnalysisTargets{Primary: base.Primary, Compatibility: []renderer.Profile{modKnown, base.Primary, modUnknown, base.Compatibility[0], modKnown}}
	b := AnalysisTargets{Primary: base.Primary, Compatibility: []renderer.Profile{base.Compatibility[0], modUnknown, modKnown}}
	if !slices.Equal(a.Profiles(), b.Profiles()) || len(a.Profiles()) != 4 {
		t.Fatalf("normalization merged capabilities or retained duplicates: %v", a.Profiles())
	}
}

func TestSafeFixRejectsChangedSourceWithRequestedPolicy(t *testing.T) {
	libass, _ := renderer.Standard(renderer.Libass)
	targets := AnalysisTargets{Primary: libass}
	doc := parseFontOverrideText(`{\fs21\fs21}A`)
	ds := AnalyzeDocumentForTargets(doc, targets)
	if _, _, err := ApplyFixesForTargets(doc.Text+"changed", ds, false, targets); err == nil {
		t.Fatal("stale proof authorized changed source")
	}
}

func TestSafeFixScopeDoesNotTrustDiagnosticRenderer(t *testing.T) {
	libass, _ := renderer.Standard(renderer.Libass)
	doc := parseFontOverrideText(`{\fs21\fs21}A`)
	targets := AnalysisTargets{Primary: libass}
	ds := AnalyzeDocumentForTargets(doc, targets)
	// Exercise the public JSON boundary, including grouped proof references.
	data, err := json.Marshal(ds)
	if err != nil {
		t.Fatal(err)
	}
	var submitted []Diagnostic
	if err := json.Unmarshal(data, &submitted); err != nil {
		t.Fatal(err)
	}
	for i := range submitted {
		submitted[i].Renderer = "libass / xy-vsfilter / vsfiltermod"
	}
	if _, _, err := ApplyFixesForTargets(doc.Text, submitted, false, DefaultAnalysisTargets()); err == nil {
		t.Fatal("diagnostic scope expanded proof authorization")
	}
	if _, count, err := ApplyFixesForTargets(doc.Text, submitted, false, targets); err != nil || count == 0 {
		t.Fatalf("valid caller scope rejected: %v", err)
	}
}
