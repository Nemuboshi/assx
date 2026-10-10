package renderer

import (
	"assx/internal/ass"
	"testing"
)

func TestParenthesizedEmptyComponentConsumption(t *testing.T) {
	for _, kind := range []Kind{Libass, XYVSFilter, VSFilterMod} {
		p, _ := New(kind, Build{})
		if kind == VSFilterMod {
			p, _ = New(kind, Build{Mod: FeatureEnabled})
		}
		source := `{\pos(1,,2)}`
		var result Result
		p.WalkDialogue(ass.ParseConcreteDialogue(source), func(r Result, _ bool) bool { result = r; return true })
		if len(result.Args) != 2 || result.Signature != SignatureVerified || !result.EmptyComponents {
			t.Fatalf("%s: %#v", kind, result)
		}
		for _, arg := range result.Args {
			if source[arg.Span.Start:arg.Span.End] != arg.Raw {
				t.Fatal("normalization lost original source")
			}
		}
	}
}

func TestBehaviorEvidenceExplicitVerification(t *testing.T) {
	libass, _ := Standard(Libass)
	xy, _ := Standard(XYVSFilter)
	mod, _ := New(VSFilterMod, Build{Mod: FeatureEnabled})
	if libass.Behavior("blur", "out-of-range").Outcome != "clamp-high" || !xy.Behavior("blur", "out-of-range").Verified {
		t.Fatal("lost verified scenario")
	}
	if xy.Behavior("be", "out-of-range").Verified || mod.Behavior("fsp", "empty-arg").Verified {
		t.Fatal("inferred renderer cell became verified")
	}
	for _, feature := range []Feature{FeatureDisabled, FeatureUnknown} {
		profile, _ := New(VSFilterMod, Build{Mod: feature})
		if profile.Behavior("fsc", "arg-form").Verified {
			t.Fatalf("feature %d borrowed enabled behavior", feature)
		}
	}
	if !mod.Behavior("fsc", "arg-form").Verified {
		t.Fatal("enabled behavior lost verification")
	}
	if e := libass.Behavior("fs", "non-numeric"); e.Verified || e.Outcome != "" {
		t.Fatal("editorial scenario default became executable evidence")
	}
}

func TestDocumentLayoutsAreIndependent(t *testing.T) {
	source := "[Script Info]\r\nScriptType: v4.00+\r\n[Events]\r\nFormat: Start, End, Layer, Style, Actor, MarginL, MarginR, MarginV, Effect, Text\r\nDialogue: 0:00:00.00,0:00:01.00,0,Default,,0,0,0,,{\\fs20}A\r\n"
	tree := ass.ParseConcreteDocument(source)
	libass, _ := Standard(Libass)
	xy, _ := Standard(XYVSFilter)
	a, b := libass.ResolveDocumentLayouts(tree), xy.ResolveDocumentLayouts(tree)
	if len(a) != 1 || len(b) != 1 || !a[0].Known || !b[0].Known {
		t.Fatalf("layouts: %#v %#v", a, b)
	}
	if a[0].Columns[0] != "start" || b[0].Columns[0] != "layer" {
		t.Fatal("Format policy leaked between profiles")
	}
	for _, row := range append(a, b...) {
		for _, cell := range row.Cells {
			if source[cell.Span.Start:cell.Span.End] != cell.Raw {
				t.Fatal("lost document span")
			}
		}
	}
	if tree.Source != source {
		t.Fatal("mutated neutral document")
	}
}
