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

func TestLibassUsesASCIIWhitespaceForEmptyComponents(t *testing.T) {
	libass, err := Standard(Libass)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		component string
		wantArgs  int
		wantSig   SignatureStatus
		wantEmpty bool
	}{
		{"empty", "", 2, SignatureVerified, true},
		{"ascii spaces", " ", 2, SignatureVerified, true},
		{"ascii tab", "\t", 2, SignatureVerified, true},
		{"letter t", "t", 3, SignatureRejected, false},
		{"nonbreaking space", "\u00a0", 3, SignatureRejected, false},
		{"em space", "\u2003", 3, SignatureRejected, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := "\\pos(1," + tc.component + ",2)"
			expr, source := first(t, raw)
			got := libass.Resolve(expr, source)
			if len(got.Args) != tc.wantArgs || got.Signature != tc.wantSig || got.EmptyComponents != tc.wantEmpty {
				t.Fatalf("resolved args=%#v signature=%v empty=%t; want %d args, %v, empty=%t",
					got.Args, got.Signature, got.EmptyComponents, tc.wantArgs, tc.wantSig, tc.wantEmpty)
			}
			for _, arg := range got.Args {
				if source[arg.Span.Start:arg.Span.End] != arg.Raw {
					t.Fatalf("argument source span changed: %#v", arg)
				}
			}
		})
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

func TestBuildScopedSignaturesAndRejection(t *testing.T) {
	profiles := make(map[Feature]Profile)
	for _, state := range []Feature{FeatureUnknown, FeatureDisabled, FeatureEnabled} {
		profile, err := New(VSFilterMod, Build{Mod: state})
		if err != nil {
			t.Fatal(err)
		}
		profiles[state] = profile
	}
	for _, tc := range []struct {
		raw                        string
		unknown, disabled, enabled SignatureStatus
	}{
		{"\\pos(1,2)", SignatureVerified, SignatureVerified, SignatureVerified},
		{"\\pos(1,2,3)", SignatureUnknown, SignatureRejected, SignatureVerified},
		{"\\pos(1,2,3,4)", SignatureUnknown, SignatureRejected, SignatureRejected},
		{"\\fsc", SignatureVerified, SignatureVerified, SignatureVerified},
		{"\\fsc42", SignatureUnknown, SignatureUnknown, SignatureVerified},
		{"\\fsc(42)", SignatureUnknown, SignatureUnknown, SignatureVerified},
		{"\\fsc(42,24)", SignatureUnknown, SignatureUnknown, SignatureUnknown},
		{"\\r", SignatureInferred, SignatureInferred, SignatureInferred},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			expr, source := first(t, tc.raw)
			for _, state := range []struct {
				feature Feature
				want    SignatureStatus
			}{
				{FeatureUnknown, tc.unknown},
				{FeatureDisabled, tc.disabled},
				{FeatureEnabled, tc.enabled},
			} {
				got := profiles[state.feature].Resolve(expr, source)
				if got.Status != Matched || got.Signature != state.want {
					t.Errorf("build=%d got status=%v signature=%v; want Matched, %v",
						state.feature, got.Status, got.Signature, state.want)
				}
			}
		})
	}
}

func TestSignatureMetadataCannotPromoteConditionalGates(t *testing.T) {
	for _, tc := range []struct {
		tag   string
		arity int
		form  Form
	}{
		{"pos", 3, Paren},
		{"fsc", 1, Both},
		{"blend", 1, Paren},
		{"frs", 1, Both},
		{"fsvp", 1, Both},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			for _, state := range []struct {
				build        Feature
				availability SignatureAvailability
			}{
				{FeatureUnknown, SignatureConditional},
				{FeatureDisabled, SignatureUnavailable},
				{FeatureEnabled, SignatureAvailable},
			} {
				profile, err := New(VSFilterMod, Build{Mod: state.build})
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, sig := range profile.Signatures(tc.tag) {
					if sig.Count != tc.arity || sig.Form != tc.form {
						continue
					}
					found = true
					if !sig.Requires.Mod || sig.Requires.Lua {
						t.Errorf("build=%d unexpected guards: %+v", state.build, sig)
					}
					if sig.Availability != state.availability {
						t.Errorf("build=%d availability=%v want %v", state.build, sig.Availability, state.availability)
					}
					if sig.Citation == "" {
						t.Fatal("missing source citation")
					}
					if state.build != FeatureEnabled && sig.Evidence != SignatureUnknown {
						t.Errorf("build=%d advertised evidence from unavailable branch: %+v", state.build, sig)
					}
					if state.build == FeatureEnabled && tc.tag != "fsvp" && sig.Evidence != SignatureVerified {
						t.Errorf("enabled source-verified signature lost evidence: %+v", sig)
					}
					if state.build == FeatureEnabled && tc.tag == "fsvp" && sig.Evidence != SignatureInferred {
						t.Errorf("inferred evidence upgraded without proof: %+v", sig)
					}
				}
				if !found {
					t.Fatalf("missing signature %s arity=%d", tc.tag, tc.arity)
				}
			}
		})
	}
	for _, state := range []Feature{FeatureUnknown, FeatureDisabled, FeatureEnabled} {
		profile, _ := New(VSFilterMod, Build{Mod: state})
		for _, core := range []struct {
			tag   string
			arity int
		}{{"pos", 2}, {"fsc", 0}} {
			found := false
			for _, sig := range profile.Signatures(core.tag) {
				if sig.Count != core.arity {
					continue
				}
				found = true
				if sig.Availability != SignatureAvailable || sig.Evidence != SignatureVerified || sig.Requires.Mod {
					t.Errorf("core %s arity=%d unavailable under mod=%d: %+v", core.tag, core.arity, state, sig)
				}
			}
			if !found {
				t.Errorf("missing core %s arity %d", core.tag, core.arity)
			}
		}
	}
}

func TestCombinedBuildRequirements(t *testing.T) {
	for _, tc := range []struct {
		mod, lua Feature
		want     SignatureAvailability
	}{
		{FeatureEnabled, FeatureEnabled, SignatureAvailable},
		{FeatureEnabled, FeatureUnknown, SignatureConditional},
		{FeatureUnknown, FeatureEnabled, SignatureConditional},
		{FeatureUnknown, FeatureUnknown, SignatureConditional},
		{FeatureDisabled, FeatureUnknown, SignatureUnavailable},
		{FeatureUnknown, FeatureDisabled, SignatureUnavailable},
		{FeatureEnabled, FeatureDisabled, SignatureUnavailable},
	} {
		profile, err := New(VSFilterMod, Build{Mod: tc.mod, Lua: tc.lua})
		if err != nil {
			t.Fatal(err)
		}
		if got := profile.signatureAvailability(requiresMod | requiresLua); got != tc.want {
			t.Errorf("_VSMOD=%v _LUA=%v availability=%v want %v", tc.mod, tc.lua, got, tc.want)
		}
	}
}

func TestBareZeroArityIsNotImplicitForAllCommands(t *testing.T) {
	for _, raw := range []string{"\\fsc", "\\r", "\\pos"} {
		e, source := first(t, raw)
		for _, profile := range profiles(t) {
			got := profile.Resolve(e, source)
			switch raw {
			case "\\fsc":
				if got.Signature != SignatureVerified {
					t.Errorf("%s %s should verify zero arity: %+v", profile.Kind(), raw, got)
				}
			case "\\r":
				want := SignatureVerified
				if profile.Kind() == VSFilterMod {
					want = SignatureInferred
				}
				if got.Signature != want {
					t.Errorf("%s %s got=%v want=%v", profile.Kind(), raw, got.Signature, want)
				}
			case "\\pos":
				if got.Signature == SignatureVerified || got.Signature == SignatureInferred {
					t.Errorf("%s missing required args incorrectly accepted: %+v", profile.Kind(), got)
				}
			}
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
