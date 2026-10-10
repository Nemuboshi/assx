package renderer

import (
	"reflect"
	"strings"
	"sync"
	"testing"

	"assx/internal/ass"
)

func first(t *testing.T, raw string) (ass.ConcreteExpression, string) {
	t.Helper()
	source := "{" + raw + "}"
	tree := ass.ParseConcreteDialogue(source)
	for _, node := range tree.Nodes {
		if node.Block == nil {
			continue
		}
		for _, item := range node.Block.Items {
			if item.Expression != nil {
				return *item.Expression, source
			}
		}
	}
	t.Fatalf("no candidate: %q", raw)
	return ass.ConcreteExpression{}, ""
}

func profiles(t *testing.T) [3]Profile {
	t.Helper()
	lib, err := Standard(Libass)
	if err != nil {
		t.Fatal(err)
	}
	xy, err := Standard(XYVSFilter)
	if err != nil {
		t.Fatal(err)
	}
	mod, err := New(VSFilterMod, Build{Mod: FeatureEnabled, Lua: FeatureEnabled})
	if err != nil {
		t.Fatal(err)
	}
	return [3]Profile{lib, xy, mod}
}

func TestActualDispatchOrderAndShadowing(t *testing.T) {
	p := profiles(t)
	cases := []struct {
		raw    string
		names  [3]string
		suffix [3]string
		shadow [3]string
	}{
		{raw: "\\fsvp6", names: [3]string{"fs", "fs", "fsvp"}, suffix: [3]string{"vp6", "vp6", "6"}, shadow: [3]string{"fsvp", "fsvp", ""}},
		{raw: "\\frs10", names: [3]string{"fr", "fr", "frs"}, suffix: [3]string{"s10", "s10", "10"}, shadow: [3]string{"frs", "frs", ""}},
		{raw: "\\blend(add)", names: [3]string{"b", "b", "blend"}, suffix: [3]string{"lend", "lend", ""}, shadow: [3]string{"blend", "blend", ""}},
		{raw: "\\fs20junk", names: [3]string{"fs", "fs", "fs"}, suffix: [3]string{"20junk", "20junk", "20junk"}},
		{raw: "\\pos(1,2)", names: [3]string{"pos", "pos", "pos"}},
		{raw: "\\pos(1,2,3)", names: [3]string{"pos", "pos", "pos"}},
		{raw: "\\FAKE123", names: [3]string{"", "", ""}},
		{raw: "\\fad(1,2)", names: [3]string{"fad", "fad", "fad"}},
	}
	for _, tt := range cases {
		t.Run(tt.raw, func(t *testing.T) {
			e, source := first(t, tt.raw)
			for i, profile := range p {
				r := profile.Resolve(e, source)
				if r.Name != tt.names[i] || r.Suffix != tt.suffix[i] || r.Shadowed != tt.shadow[i] {
					t.Errorf("%s: got name=%q suffix=%q shadow=%q; want %q %q %q",
						profile.Kind(), r.Name, r.Suffix, r.Shadowed, tt.names[i], tt.suffix[i], tt.shadow[i])
				}
				if r.Source != e.Span || r.Raw != e.Raw {
					t.Errorf("%s: source changed: %+v", profile.Kind(), r)
				}
			}
		})
	}
}

func TestSignatureEvidenceIsRendererScoped(t *testing.T) {
	p := profiles(t)
	for _, test := range []struct {
		source string
		want   [3]SignatureStatus
	}{
		{source: "\\pos(1,2)", want: [3]SignatureStatus{SignatureVerified, SignatureVerified, SignatureVerified}},
		{source: "\\pos(1,2,3)", want: [3]SignatureStatus{SignatureRejected, SignatureRejected, SignatureVerified}},
		{source: "\\blend(add)", want: [3]SignatureStatus{SignatureUnknown, SignatureUnknown, SignatureVerified}},
		{source: "\\fsvp6", want: [3]SignatureStatus{SignatureVerified, SignatureVerified, SignatureInferred}},
	} {
		e, raw := first(t, test.source)
		for i, profile := range p {
			r := profile.Resolve(e, raw)
			if r.Signature != test.want[i] {
				t.Errorf("%s %s signature=%v, want %v", profile.Kind(), test.source, r.Signature, test.want[i])
			}
		}
	}
	for _, profile := range p[:2] {
		e, raw := first(t, "\\pos(1,2,3)")
		if profile.Resolve(e, raw).Signature != SignatureRejected {
			t.Fatal("mod-specific position arity leaked to traditional profiles")
		}
	}
}

func TestCapabilitiesNeverDefaultToEnabled(t *testing.T) {
	unknown, err := Standard(VSFilterMod)
	if err != nil {
		t.Fatal(err)
	}
	enabled, err := New(VSFilterMod, Build{Mod: FeatureEnabled, Lua: FeatureEnabled})
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := New(VSFilterMod, Build{Mod: FeatureDisabled, Lua: FeatureDisabled})
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"\\fsvp6", "\\frs10", "\\blend(add)"} {
		e, source := first(t, raw)
		got := unknown.Resolve(e, source)
		if got.Status != Conditional || got.Name == "" || got.Signature != SignatureUnknown {
			t.Errorf("%s unconfigured build unexpectedly resolved: %+v", raw, got)
		}
		if enabled.Resolve(e, source).Status != Matched {
			t.Errorf("%s enabled build not matched", raw)
		}
		off := disabled.Resolve(e, source)
		if off.Name == got.Name {
			t.Errorf("%s disabled mod branch still selected: %+v", raw, off)
		}
	}
	if _, err := New(VSFilterMod, Build{Mod: FeatureDisabled, Lua: FeatureEnabled}); err == nil {
		t.Fatal("accepted incompatible build capabilities")
	}
	e, source := first(t, "\\lua(test)")
	for _, build := range []Build{{Mod: FeatureEnabled, Lua: FeatureUnknown}, {Mod: FeatureUnknown, Lua: FeatureUnknown}} {
		profile, err := New(VSFilterMod, build)
		if err != nil {
			t.Fatal(err)
		}
		if profile.Resolve(e, source).Status != Conditional {
			t.Errorf("Lua build %+v must remain conditional", build)
		}
	}
}

func TestNoOpNormalizationIsNotAValidCommandWithSuffix(t *testing.T) {
	mod := profiles(t)[2]
	e, source := first(t, "\\posjunk(1,2)")
	r := mod.Resolve(e, source)
	if r.Name != "pos" || r.Status != Ignored || r.Signature != SignatureUnknown {
		t.Errorf("literal-only normalization should not accept junk suffix: %+v", r)
	}
	e, source = first(t, "\\blend(add)")
	if got := mod.Resolve(e, source); got.Status != Matched || got.Name != "blend" {
		t.Errorf("pinned blend branch is reachable: %+v", got)
	}
}

func TestNestedTransformUsesSameCoordinatesAndDispatch(t *testing.T) {
	raw := "{\\t(0,100,\\pos(1,2,3)\\fsvp6)}hello"
	tree := ass.ParseConcreteDialogue(raw)
	for _, p := range profiles(t) {
		var names []string
		var nested []bool
		p.WalkDialogue(tree, func(r Result, inTransition bool) bool {
			names = append(names, r.Name)
			nested = append(nested, inTransition)
			if r.Source.Start < 0 || r.Source.End > len(raw) || raw[r.Source.Start:r.Source.End] != r.Raw {
				t.Errorf("source coordinate mismatch: %+v", r)
			}
			return true
		})
		if len(names) != 3 || names[0] != "t" || names[1] != "pos" || !nested[1] || !nested[2] {
			t.Errorf("%s traversal=%v inTransform=%v", p.Kind(), names, nested)
		}
	}
}

func TestNestedTransformsWalkIteratively(t *testing.T) {
	source := "{\\t(0,100,\\t(0,50,\\pos(1,2,3)))}"
	tree := ass.ParseConcreteDialogue(source)
	for _, profile := range profiles(t) {
		var names []string
		profile.WalkDialogue(tree, func(r Result, nested bool) bool {
			names = append(names, r.Name)
			if len(names) > 1 && !nested {
				t.Errorf("nested event not marked: %+v", r)
			}
			return true
		})
		if !reflect.DeepEqual(names, []string{"t", "t", "pos"}) {
			t.Errorf("%s nested traversal: %v", profile.Kind(), names)
		}
	}
}

func TestProfilesHaveValueSemanticsAndDetachedResults(t *testing.T) {
	p := profiles(t)
	e, source := first(t, "\\pos(1,2)")
	r := p[0].Resolve(e, source)
	if !r.HasPolicy || len(r.Policy.Counts) == 0 {
		t.Fatal("missing resolved policy")
	}
	r.Policy.Counts[0] = 999
	if got := p[0].Resolve(e, source); got.Policy.Counts[0] == 999 {
		t.Fatal("shared policy changed")
	}
	sigs := p[0].Signatures("pos")
	sigs[0].Count = 999
	if got := p[0].Signatures("pos"); got[0].Count == 999 {
		t.Fatal("shared signature changed")
	}
	for _, profile := range p {
		if len(profile.Version()) != 40 || strings.TrimSpace(profile.Kind().String()) == "" {
			t.Fatal("bad pinned profile identity")
		}
	}
	var wg sync.WaitGroup
	for n := 0; n < 24; n++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if got := p[i%3].Resolve(e, source); got.Name != "pos" {
					t.Errorf("unstable concurrent dispatch: %+v", got)
				}
			}
		}(n)
	}
	wg.Wait()
}

func TestMalformedSyntaxNeverBecomesProven(t *testing.T) {
	p := profiles(t)
	for _, raw := range []string{"\\\\pos(1,2)", "\\pos(1,2", "\\UNKNOWN123"} {
		e, source := first(t, raw)
		for _, profile := range p {
			r := profile.Resolve(e, source)
			if r.Signature == SignatureVerified {
				t.Errorf("%s accepted %s with %+v", profile.Kind(), raw, r)
			}
		}
	}
}

func BenchmarkProfileDispatch(b *testing.B) {
	profile, _ := New(VSFilterMod, Build{Mod: FeatureEnabled, Lua: FeatureEnabled})
	source := "{\\fsvp6\\pos(1,2,3)\\blend(add)}"
	tree := ass.ParseConcreteDialogue(source)
	e := *tree.Nodes[0].Block.Items[0].Expression
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = profile.Resolve(e, source)
	}
}
