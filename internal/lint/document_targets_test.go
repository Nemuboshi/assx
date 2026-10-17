package lint

import (
	"strings"
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

func TestPrimaryVSFilterUsesFixedDocumentSlots(t *testing.T) {
	xy, _ := renderer.Standard(renderer.XYVSFilter)
	libass, _ := renderer.Standard(renderer.Libass)
	source := "[Script Info]\nScriptType: v4.00+\nPlayResX: 640\nPlayResY: 480\nLayoutResX: 640\nLayoutResY: 480\nYCbCr Matrix: None\n[Events]\nFormat: Start, Layer, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\fs21\\fs21}A\n"
	doc := ass.Parse(source)
	for _, profile := range []renderer.Profile{xy, libass} {
		findings := AnalyzeDocumentForTargets(doc, AnalysisTargets{Primary: profile})
		invalid := false
		for _, d := range findings {
			invalid = invalid || d.Severity == Error
		}
		if invalid != (profile == libass) {
			t.Fatalf("%s primary used the wrong event layout", profile.Kind())
		}
	}
	missingText := ass.Parse(strings.Replace(source, ", Text\n", ", Unknown\n", 1))
	if len(missingText.Dialogues) != 0 {
		t.Fatal("libass view unexpectedly retained missing Text")
	}
	if len(xy.DocumentView(missingText).Dialogues) != 1 {
		t.Fatal("fixed-position primary lost the Text field")
	}
}

func TestSafeFixProofUsesEachTargetsStyleLayout(t *testing.T) {
	xy, _ := renderer.Standard(renderer.XYVSFilter)
	libass, _ := renderer.Standard(renderer.Libass)
	source := "[Script Info]\nScriptType: v4.00+\nPlayResX: 640\nPlayResY: 480\n[V4+ Styles]\nFormat: Name, Fontname, Bold, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Fontsize, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\nStyle: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,30,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\fs20}A\n"
	doc := ass.Parse(source)
	targets := AnalysisTargets{Primary: xy, Compatibility: []renderer.Profile{libass}}
	findings := AnalyzeDocumentForTargets(doc, targets)
	redundant := false
	for _, finding := range findings {
		if finding.ID != IssueNoEffect && finding.ID != IssueRedundantStyleOverrides {
			continue
		}
		redundant = true
		if finding.FixSafety == SafeFix || finding.FixProof != nil || len(finding.Edits) != 0 {
			t.Fatal("proof borrowed the primary renderer's Style layout")
		}
	}
	if !redundant {
		t.Fatal("xy-VSFilter did not report the redundant font size")
	}
	fixed, count, err := ApplyFixesForTargets(doc.Text, findings, false, targets)
	if err != nil || count != 0 || fixed != doc.Text {
		t.Fatalf("unsafe cross-renderer fix applied: count=%d err=%v", count, err)
	}

	// Recreate a primary-projected proof to verify application rechecks the source layouts.
	projected := proveSafeFixes(xy.DocumentView(doc), analyzeDocumentForTargetsUnproved(doc, targets, nil), targets.Profiles())
	if safe, _, _ := CountFixes(projected); safe == 0 {
		t.Fatal("test input did not reproduce a primary-view proof")
	}
	fixed, count, err = ApplyFixesForTargets(doc.Text, projected, false, targets)
	if err == nil || count != 0 {
		t.Fatalf("application accepted a primary-view proof: fixed=%q count=%d err=%v", fixed, count, err)
	}
}

func TestUnknownDialectKeepsOriginalVSFilterView(t *testing.T) {
	xy, _ := renderer.Standard(renderer.XYVSFilter)
	source := "[Script Info]\nScriptType: v4.00\nPlayResX: 640\nPlayResY: 480\n[V4 Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, TertiaryColour, BackColour, Bold, Italic, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, AlphaLevel, Encoding\nStyle: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,1,2,2,2,10,10,10,0,1\n[Events]\nFormat: Marked, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: Marked=0,0:00:00.00,0:00:01.00,Default,,0,0,0,,A\n"
	doc := ass.Parse(source)
	view := xy.DocumentView(doc)
	if len(view.Dialogues) != 1 || len(view.StyleFields) != len(doc.StyleFields) {
		t.Fatal("unsupported SSA records were projected as ASS v4+")
	}
	if _, ok := view.Dialogues[0].Field("marked"); !ok {
		t.Fatal("SSA Marked field was renamed to an ASS layer")
	}
	for _, finding := range AnalyzeDocumentForTargets(doc, AnalysisTargets{Primary: xy}) {
		if finding.ID == IssueUndefinedStyle {
			t.Fatalf("valid SSA Style was lost: %#v", finding)
		}
	}
}

func TestSafeFixDoesNotBorrowPrimaryStyleLayout(t *testing.T) {
	doc := parseStyleOverrideText("Name, MarginL", "Default,12.75", "A")
	libass, _ := renderer.Standard(renderer.Libass)
	local := AnalysisTargets{Primary: libass}
	localFindings := AnalyzeDocumentForTargets(doc, local)
	if safe, _, _ := CountFixes(localFindings); safe != 1 {
		t.Fatal("libass integer field proof missing")
	}
	shared := AnalyzeDocument(doc)
	if safe, _, _ := CountFixes(shared); safe != 0 {
		t.Fatal("xy font-name field borrowed libass integer proof")
	}
	fixed, count, err := ApplyFixes(doc.Text, shared, false)
	if err != nil || count != 0 || fixed != doc.Text {
		t.Fatal("wrong-layout proof changed source")
	}
}

func TestSafeFixReauthorizesSameSetWithDifferentPrimary(t *testing.T) {
	libass, _ := renderer.Standard(renderer.Libass)
	xy, _ := renderer.Standard(renderer.XYVSFilter)
	doc := parseStyleOverrideText("Name, Fontname, Fontsize", "Default,Arial,20", `{\fs20}A`)
	first := AnalysisTargets{Primary: libass, Compatibility: []renderer.Profile{xy}}
	second := AnalysisTargets{Primary: xy, Compatibility: []renderer.Profile{libass}}
	findings := AnalyzeDocumentForTargets(doc, first)
	fixed, count, err := ApplyFixesForTargets(doc.Text, findings, false, second)
	if err != nil || count != 1 || !strings.Contains(fixed, ",,A\n") {
		t.Fatalf("reauthorized same-set fix rejected: count=%d err=%v", count, err)
	}
	for i := range findings {
		if findings[i].FixProof == nil {
			continue
		}
		// Equal target sets do not authorize a different rule's edit policy.
		findings[i].ID = IssueNoEffect
		proof := findings[i].FixProof
		for j := range proof.SourceEdits {
			proof.SourceEdits[j].ID = IssueNoEffect
		}
		proof.ID = ""
		proof.ID, err = hashJSON(proof)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := ApplyFixesForTargets(doc.Text, findings, false, second); err == nil {
			t.Fatal("same target set bypassed caller-primary edit authorization")
		}
		return
	}
	t.Fatal("missing proof")
}
