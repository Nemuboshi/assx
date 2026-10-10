package main

import (
	"testing"

	"assx/internal/ass"
	"assx/internal/lint"
)

// These policy invariants must stay independent of golden-file regeneration.
// They characterize the default libass/xy acceptance envelope at main@6ba7453.
func TestDefaultRejectsModOnlyPositionArity(t *testing.T) {
	invalid := lint.Analyze(ass.Dialogue{Text: `{\pos(1,2,3)}x`})
	invalidPos := false
	for _, d := range invalid {
		if d.ID == "ASS001" && d.Tag == "pos" && d.Severity == lint.Error {
			invalidPos = true
		}
	}
	if !invalidPos {
		t.Fatalf("default renderer accepted a VSFilterMod-only 3-coordinate position: %#v", invalid)
	}

	valid := lint.Analyze(ass.Dialogue{Text: `{\pos(1,2)}x`})
	for _, d := range valid {
		if d.ID == "ASS001" && d.Tag == "pos" {
			t.Fatalf("default renderer rejected a valid 2-coordinate position: %#v", valid)
		}
	}
}

func TestDefaultModOnlyTagsDoNotBecomeSilentlyValid(t *testing.T) {
	for _, name := range []string{"fsvp6", "frs", "blend(add)"} {
		t.Run(name, func(t *testing.T) {
			diagnostics := lint.Analyze(ass.Dialogue{Text: "{\\" + name + "}x"})
			found := false
			for _, d := range diagnostics {
				if d.ID == lint.IssueVSFilterModTag {
					found = true
					if d.FixSafety != "" || len(d.Edits) != 0 {
						t.Fatalf("VSFilterMod-only tag unexpectedly acquired an automatic edit: %#v", d)
					}
				}
			}
			if !found {
				t.Fatalf("VSFilterMod-only tag lost compatibility warning: %#v", diagnostics)
			}
		})
	}
}

func TestUnknownOperationBlocksRetroactiveSafeFix(t *testing.T) {
	diagnostics := lint.Analyze(ass.Dialogue{Text: `{\fs10\fs20\unknownTag}x`})
	foundUnknown := false
	for _, d := range diagnostics {
		if d.ID == lint.IssueUnknownTag {
			foundUnknown = true
		}
		if d.FixSafety == lint.SafeFix && len(d.Edits) > 0 {
			t.Fatalf("edit escaped an unknown-operation proof barrier: %#v", d)
		}
	}
	if !foundUnknown {
		t.Fatalf("unknown operation disappeared from default lint diagnostics: %#v", diagnostics)
	}
}
