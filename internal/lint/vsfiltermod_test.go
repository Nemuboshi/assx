package lint

import (
	"testing"

	"assx/internal/ass"
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
			diagnostics := Analyze(ass.Dialogue{Text: text})
			if len(diagnostics) != 1 || diagnostics[0].ID != IssueVSFilterModTag || diagnostics[0].Tag != tag {
				t.Fatalf("diagnostics = %#v", diagnostics)
			}
			diagnostic := diagnostics[0]
			if diagnostic.Severity != Warning || diagnostic.FixSafety != "" || len(diagnostic.Edits) != 0 {
				t.Fatalf("mod-tag warning has fix metadata: %#v", diagnostic)
			}
			safe, unsafe, unfixable := CountFixes(diagnostics)
			if safe != 0 || unsafe != 0 || unfixable != 1 {
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
	diagnostics := AnalyzeDocument(doc)
	foundWarning := false
	foundNoEffect := false
	for _, diagnostic := range diagnostics {
		if diagnostic.ID == IssueVSFilterModTag {
			foundWarning = true
		}
		if diagnostic.ID == IssueNoEffect {
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
	diagnostics := Analyze(ass.Dialogue{Text: `{\fs20}x`})
	for _, diagnostic := range diagnostics {
		if diagnostic.ID == IssueVSFilterModTag {
			t.Fatalf("common tag got VSFilterMod warning: %#v", diagnostic)
		}
	}
}
