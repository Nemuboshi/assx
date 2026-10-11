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
