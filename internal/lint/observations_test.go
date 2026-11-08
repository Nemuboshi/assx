package lint

import (
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

func TestConditionFromResult(t *testing.T) {
	libass, err := renderer.Standard(renderer.Libass)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name      string
		result    renderer.Result
		condition string
		status    Status
	}{
		{"unterminated", renderer.Result{}, IssueRendererUnresolved, StatusUnknown},
		{"unknown name", renderer.Result{Closed: true, Status: renderer.UnknownName}, IssueUnknownTag, StatusUnknown},
		{"disabled", renderer.Result{Closed: true, Status: renderer.Disabled}, IssueRendererUnresolved, StatusUnmodeled},
		{"ignored", renderer.Result{Closed: true, Status: renderer.Ignored}, IssueRendererUnresolved, StatusIgnored},
		{"rejected signature", renderer.Result{Closed: true, Status: renderer.Matched, HasPolicy: true, Signature: renderer.SignatureRejected}, IssueArgumentCount, StatusRejected},
		{"inferred signature", renderer.Result{Closed: true, Status: renderer.Matched, HasPolicy: true, Signature: renderer.SignatureInferred}, IssueRendererUnresolved, StatusInferred},
		{"no policy", renderer.Result{Closed: true, Status: renderer.Matched}, IssueRendererUnresolved, StatusUnmodeled},
		{"accepted", renderer.Result{Closed: true, Status: renderer.Matched, HasPolicy: true, Signature: renderer.SignatureVerified}, "", StatusAccepted},
	} {
		condition, status, _ := conditionFromResult(test.result, libass, "pos")
		if condition != test.condition || status != test.status {
			t.Errorf("%s: got (%q, %q), want (%q, %q)", test.name, condition, status, test.condition, test.status)
		}
	}
}

// A compared renderer that rejects a signature the primary accepts creates
// the arity finding. The reverse primary turns the same finding into an error.
func TestComparedResolutionCreatesArityFinding(t *testing.T) {
	const source = "[Script Info]\nPlayResX: 640\nPlayResY: 360\nLayoutResX: 640\nLayoutResY: 360\nYCbCr Matrix: None\n" +
		"[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default, Arial, 20\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\pos(1,2,3)}Hi\n"
	doc := ass.Parse(source)
	mod, err := renderer.Standard(renderer.VSFilterMod)
	if err != nil {
		t.Fatal(err)
	}
	libass, err := renderer.Standard(renderer.Libass)
	if err != nil {
		t.Fatal(err)
	}
	findArity := func(findings []Diagnostic) *Diagnostic {
		for i := range findings {
			if findings[i].Condition == IssueArgumentCount {
				return &findings[i]
			}
		}
		return nil
	}
	modPrimary := analyzeDocumentForTargetsUnproved(doc, AnalysisTargets{Primary: mod, Compatibility: []renderer.Profile{libass}}, nil)
	arity := findArity(modPrimary)
	if arity == nil {
		t.Fatal("compared rejection did not create the arity finding")
	}
	if arity.Severity != Warning || len(arity.Renderers) != 2 ||
		arity.Renderers[0].Renderer != mod.Kind().String() || arity.Renderers[0].Status != StatusAccepted ||
		arity.Renderers[1].Renderer != libass.Kind().String() || arity.Renderers[1].Status != StatusRejected {
		t.Fatalf("compared arity finding = %#v", arity)
	}
	libassPrimary := analyzeDocumentForTargetsUnproved(doc, AnalysisTargets{Primary: libass, Compatibility: []renderer.Profile{mod}}, nil)
	arity = findArity(libassPrimary)
	if arity == nil || arity.Severity != Error {
		t.Fatalf("primary arity finding = %#v, want error", arity)
	}
}

// A lint-side value check still reports under a compared renderer: it runs
// for the finding's span, not for every tag in the document.
func TestComparedValueFindingKeepsEveryObservation(t *testing.T) {
	const source = "[Script Info]\nPlayResX: 640\nPlayResY: 360\nLayoutResX: 640\nLayoutResY: 360\nYCbCr Matrix: None\n" +
		"[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default, Arial, 20\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\b2}Hi\n"
	doc := ass.Parse(source)
	libass, err := renderer.Standard(renderer.Libass)
	if err != nil {
		t.Fatal(err)
	}
	vsfilter, err := renderer.Standard(renderer.XYVSFilter)
	if err != nil {
		t.Fatal(err)
	}
	findings := analyzeDocumentForTargetsUnproved(doc, AnalysisTargets{Primary: libass, Compatibility: []renderer.Profile{vsfilter}}, nil)
	for _, finding := range findings {
		if finding.Condition != IssueInvalidValue {
			continue
		}
		if len(finding.Renderers) != 2 || finding.Renderers[1].Status != StatusRejected {
			t.Fatalf("value finding lost the compared observation: %#v", finding)
		}
		return
	}
	t.Fatal("value finding missing")
}

// A compared renderer evaluates karaoke and drawing state only for dialogues
// that use them; the observation merges with the primary finding.
func TestComparedKaraokeAddsObservation(t *testing.T) {
	const source = "[Script Info]\nPlayResX: 640\nPlayResY: 360\nLayoutResX: 640\nLayoutResY: 360\nYCbCr Matrix: None\n" +
		"[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default, Arial, 20\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\k9999}Hi\n"
	doc := ass.Parse(source)
	libass, err := renderer.Standard(renderer.Libass)
	if err != nil {
		t.Fatal(err)
	}
	vsfilter, err := renderer.Standard(renderer.XYVSFilter)
	if err != nil {
		t.Fatal(err)
	}
	findings := analyzeDocumentForTargetsUnproved(doc, AnalysisTargets{Primary: libass, Compatibility: []renderer.Profile{vsfilter}}, nil)
	for _, finding := range findings {
		if finding.Condition != IssueKaraoke {
			continue
		}
		if len(finding.Renderers) != 2 || finding.Renderers[1].Status != StatusRejected {
			t.Fatalf("karaoke finding lost the compared observation: %#v", finding)
		}
		return
	}
	t.Fatal("karaoke finding missing")
}
