package lint

import (
	"testing"

	"assx/internal/ass/renderer"
	"assx/internal/ass/spec"
)

func TestObservationStatusAndCondition(t *testing.T) {
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
		status := observationStatus(test.result)
		if status != test.status {
			t.Errorf("%s: status = %q, want %q", test.name, status, test.status)
		}
		if condition := observationCondition(test.result); condition != test.condition {
			t.Errorf("%s: condition = %q, want %q", test.name, condition, test.condition)
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
	doc := renderer.Parse(source)
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
	doc := renderer.Parse(source)
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
	doc := renderer.Parse(source)
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

// A custom event Format moves the libass Text span away from the VSFilter
// span. The two libass and VSFilter streams are not comparable, while the
// primary and VSFilter streams share their Text bytes.
func TestComparedKaraokeSplitsProjectedTexts(t *testing.T) {
	const source = "[Script Info]\nScriptType: v4.00+\n" +
		"[Events]\n" +
		"Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Text, Effect\n" +
		"Dialogue: 0,0:00:00.00,0:00:00.10,Default,,0,0,0,{\\k100}Hi,{\\k5}A{\\k10}B\n"
	doc := renderer.Parse(source)
	libass, vsfilter, mod := scopedProfiles(t)
	targets := AnalysisTargets{Primary: mod, Compatibility: []renderer.Profile{libass, vsfilter}}
	findings := analyzeDocumentForTargetsUnproved(doc, targets, nil)
	vsfilterObserved, libassObserved, notComparable := false, false, false
	for _, finding := range findings {
		if finding.Condition == ConditionLayout {
			for _, observation := range finding.Renderers {
				if observation.Renderer == libass.Kind().String() && observation.Status == StatusNotComparable {
					notComparable = true
				}
			}
		}
		if finding.Condition != IssueKaraoke {
			continue
		}
		for _, observation := range finding.Renderers {
			if observation.Renderer == vsfilter.Kind().String() && observation.Role == RoleCompared && observation.Status == StatusRejected {
				vsfilterObserved = true
			}
			if observation.Renderer == libass.Kind().String() {
				libassObserved = true
			}
		}
	}
	if !vsfilterObserved {
		t.Fatalf("compared karaoke finding lost the VSFilter observation: %#v", findings)
	}
	if libassObserved {
		t.Fatalf("compared libass tags on a different Text span: %#v", findings)
	}
	if !notComparable {
		t.Fatalf("different Text spans were not marked not comparable: %#v", findings)
	}
}

func TestSameResolution(t *testing.T) {
	base := renderer.Result{
		Closed: true, Status: renderer.Matched, Signature: renderer.SignatureVerified, HasPolicy: true,
		Name: "q", Form: renderer.Bare, Policy: spec.TagSpec{Value: spec.IntegerValue, Min: 0, Max: 3},
		Args: []renderer.Argument{{Raw: "2"}},
	}
	if !sameResolution(base, base) {
		t.Fatal("equal resolutions differ")
	}
	for _, changed := range []renderer.Result{
		{Closed: true, Status: renderer.Matched, Signature: renderer.SignatureVerified, HasPolicy: true,
			Name: "q", Form: renderer.Bare, Policy: spec.TagSpec{Value: spec.IntegerValue, Min: 0, Max: 5},
			Args: []renderer.Argument{{Raw: "2"}}},
		{Closed: true, Status: renderer.Matched, Signature: renderer.SignatureVerified, HasPolicy: true,
			Name: "q", Form: renderer.Bare, Policy: spec.TagSpec{Value: spec.IntegerValue, Min: 0, Max: 3},
			Args: []renderer.Argument{{Raw: "3"}}},
		{Closed: true, Status: renderer.UnknownName, HasPolicy: true, Name: "q", Form: renderer.Bare,
			Policy: spec.TagSpec{Value: spec.IntegerValue, Min: 0, Max: 3}, Args: []renderer.Argument{{Raw: "2"}}},
	} {
		if sameResolution(base, changed) {
			t.Fatalf("different resolution reported equal: %#v", changed)
		}
	}
}

// Under the default targets libass is primary and xy-VSFilter is compared.
// A custom Format moves their Text spans apart, so no compared tag
// observation is valid for that record.
func TestDefaultTargetsDoNotCompareDifferentText(t *testing.T) {
	const source = "[Script Info]\nScriptType: v4.00+\n" +
		"[Events]\n" +
		"Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Text, Effect\n" +
		"Dialogue: 0,0:00:00.00,0:00:02.00,Default,,0,0,0,{\\k20}Hi,{\\k10}X\n"
	doc := renderer.Parse(source)
	findings := analyzeDocumentForTargetsUnproved(doc, DefaultAnalysisTargets(), nil)
	notComparable := false
	for _, finding := range findings {
		if finding.Condition == ConditionLayout {
			for _, observation := range finding.Renderers {
				if observation.Role == RoleCompared && observation.Status == StatusNotComparable {
					notComparable = true
				}
			}
		}
		if finding.Condition != IssueKaraoke {
			continue
		}
		for _, observation := range finding.Renderers {
			if observation.Role == RoleCompared {
				t.Fatalf("compared karaoke observation on different Text spans: %#v", finding)
			}
		}
	}
	if !notComparable {
		t.Fatalf("missing not-comparable layout observation: %#v", findings)
	}
}
