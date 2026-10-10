package lint

import (
	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"reflect"
	"strings"
	"testing"
)

func TestCompatibilityDiagnosticsSeparateValidityAndFixSafety(t *testing.T) {
	mod, err := renderer.New(renderer.VSFilterMod, renderer.Build{Mod: renderer.FeatureEnabled})
	if err != nil {
		t.Fatal(err)
	}
	libass, _ := renderer.Standard(renderer.Libass)
	dialogue := ass.Dialogue{Text: `{\pos(1,2,3)\pos(4,5)\fsvp6\blend(add)}A`, Line: 7}
	before := Analyze(dialogue)
	findings := AnalyzeCompatibility(dialogue, []renderer.Profile{libass, mod})
	var signature, dispatch, ownership bool
	for _, d := range findings {
		if d.ID == IssueArgumentCount || d.FixSafety != "" || len(d.Edits) != 0 {
			t.Fatalf("compatibility leaked validity or edits: %#v", d)
		}
		if d.Line != 7 || d.Column < 2 {
			t.Fatalf("wrong source position: %#v", d)
		}
		if d.ID == IssueRendererDiff {
			if !strings.Contains(d.Detail, "libass") || !strings.Contains(d.Detail, "VSFilterMod") {
				t.Fatalf("missing one side: %#v", d)
			}
			switch d.Field {
			case "signature":
				signature = true
			case "dispatch":
				dispatch = true
			case "ownership":
				ownership = true
			}
		}
	}
	if !signature || !dispatch || !ownership {
		t.Fatal("deduplication hid distinct failures")
	}
	if !reflect.DeepEqual(before, Analyze(dialogue)) {
		t.Fatal("comparison changed default diagnostics")
	}
	strict := false
	for _, d := range before {
		if d.ID == IssueArgumentCount {
			strict = true
		}
	}
	if !strict {
		t.Fatal("default accepted Mod position arity")
	}
	if !reflect.DeepEqual(findings, AnalyzeCompatibility(dialogue, []renderer.Profile{libass, mod, libass})) {
		t.Fatal("duplicate target changed diagnostics")
	}
}

func TestDocumentCompatibilityDiagnosticsPhysicalLocations(t *testing.T) {
	source := "[Script Info]\rScriptType: v4.00+\r[Events]\rFormat: Text\rDialogue: {\\fs20}A\r"
	doc := ass.Parse(source)
	findings := AnalyzeDocumentCompatibility(doc, nil)
	if len(findings) == 0 {
		t.Fatal("legacy parser's dropped event hid document incompatibility")
	}
	for _, d := range findings {
		if d.Line != 5 {
			t.Fatalf("wrong physical line: %#v", d)
		}
		if d.FixSafety != "" || len(d.Edits) != 0 {
			t.Fatal("document comparison introduced a fix")
		}
	}
}

func TestCompatibilityReportsIgnoredAndUnverifiedSeparately(t *testing.T) {
	ignored := AnalyzeCompatibility(ass.Dialogue{Text: `{\mystery}A`, Line: 1}, nil)
	if len(ignored) != 1 || !strings.Contains(ignored[0].Detail, "ignored-or-unsupported") {
		t.Fatalf("unknown command not explicitly ignored: %#v", ignored)
	}
	unresolved := AnalyzeCompatibility(ass.Dialogue{Text: `{\be200}A`, Line: 1}, nil)
	for _, d := range unresolved {
		if d.Field == "behavior/out-of-range" {
			if d.ID != IssueRendererUnresolved || !strings.Contains(d.Detail, "unresolved") {
				t.Fatal("unverified evidence claimed compatibility")
			}
			return
		}
	}
	t.Fatal("missing unverified scenario")
}
