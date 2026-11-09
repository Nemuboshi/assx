package lint

import (
	"assx/internal/ass/renderer"
	"testing"
)

func TestVSFilterModTagsWarnWithoutAutomaticFixes(t *testing.T) {
	tags := []string{
		"1img", "2img", "3img", "4img",
		"1vc", "2vc", "3vc", "4vc",
		"1va", "2va", "3va", "4va",
		"distort", "frs", "fsvp", "fshp", "jitter", "lua", "mover", "moves3", "moves4", "movevc",
		"rnds", "rndx", "rndy", "rndz", "rnd", "xblur", "yblur", "z", "ortho", "blend",
	}
	for _, tag := range tags {
		t.Run(tag, func(t *testing.T) {
			text := "{\\" + tag + "(1)}x"
			diagnostics := Analyze(renderer.Dialogue{Text: text})
			var diagnostic Diagnostic
			for _, finding := range diagnostics {
				if finding.Condition == IssueVSFilterModTag && finding.Tag == tag {
					diagnostic = finding
				}
				if finding.FixSafety != "" || len(finding.Edits) != 0 {
					t.Fatalf("Mod invocation authorized an edit: %s", finding.Condition)
				}
			}
			if diagnostic.Condition == "" {
				t.Fatalf("missing purposeful portability for %s", tag)
			}
			if diagnostic.Severity != Warning || diagnostic.FixSafety != "" || len(diagnostic.Edits) != 0 {
				t.Fatalf("mod-tag warning has fix metadata: %#v", diagnostic)
			}
			safe, unsafe, unfixable := CountFixes(diagnostics)
			if safe != 0 || unsafe != 0 || unfixable != len(diagnostics) {
				t.Fatalf("fix counts = (%d, %d, %d)", safe, unsafe, unfixable)
			}
			fixed, count, err := ApplyFixes(text, diagnostics, true)
			if err != nil || count != 0 || fixed != text {
				t.Fatalf("fix result = (%q, %d, %v)", fixed, count, err)
			}
		})
	}
}

func TestVSFilterModTagDisablesDialogueFixes(t *testing.T) {
	doc := parseFontOverrideText(`{\fs21\fs20\distort(1)}x`)
	diagnostics := analyzeDocumentTest(doc)
	foundWarning := false
	foundNoEffect := false
	for _, diagnostic := range diagnostics {
		if diagnostic.Condition == IssueVSFilterModTag {
			foundWarning = true
		}
		if diagnostic.Condition == IssueNoEffect {
			foundNoEffect = true
		}
		if diagnostic.FixSafety != "" || len(diagnostic.Edits) != 0 {
			t.Errorf("dialogue with a mod-only tag remains auto-fixable: %#v", diagnostic)
		}
	}
	if !foundWarning || !foundNoEffect {
		t.Fatalf("expected compatibility and no-effect findings, got %#v", diagnostics)
	}
	fixed, count, err := ApplyFixes(doc.Text, diagnostics, true)
	if err != nil || count != 0 || fixed != doc.Text {
		t.Fatalf("fix result = (%q, %d, %v)", fixed, count, err)
	}
}

func TestCommonTagsDoNotGetVSFilterModWarning(t *testing.T) {
	diagnostics := Analyze(renderer.Dialogue{Text: `{\fs20}x`})
	for _, diagnostic := range diagnostics {
		if diagnostic.Condition == IssueVSFilterModTag {
			t.Fatalf("common tag got VSFilterMod warning: %#v", diagnostic)
		}
	}
}
