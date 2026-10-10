package main

import (
	"testing"

	"assx/internal/ass"
	"assx/internal/lint"
)

// These policy invariants remain separate from main@6ba7453 historical goldens.
// Do not pin transitional diagnostics for names subject to prefix dispatch.
func TestDefaultRejectsModOnlyPositionArity(t *testing.T) {
	invalid := lint.Analyze(ass.Dialogue{Text: `{\pos(1,2,3)}x`})
	invalidPos := false
	for _, d := range invalid {
		if d.Tag == "pos" && d.Severity == lint.Error {
			invalidPos = true
		}
	}
	if !invalidPos {
		t.Fatalf("default renderer accepted a VSFilterMod-only 3-coordinate position: %#v", invalid)
	}

	valid := lint.Analyze(ass.Dialogue{Text: `{\pos(1,2)}x`})
	for _, d := range valid {
		if d.Tag == "pos" && d.Severity == lint.Error {
			t.Fatalf("default renderer rejected a valid 2-coordinate position: %#v", valid)
		}
	}
}

// Names such as fsvp/frs/blend have renderer-dependent dispatch. A future
// parser may resolve them through fs/fr/be instead of reporting legacy ASS005.
// Until renderer-scoped proof exists, a prefix collision must not authorize a
// retroactive SafeFix for an earlier candidate.
func TestPrefixCollisionsDoNotAuthorizeSafeFix(t *testing.T) {
	for _, name := range []string{"fsvp6", "frs", "blend(add)"} {
		t.Run(name, func(t *testing.T) {
			input := "{\\fs10\\fs20\\" + name + "}x"
			diagnostics := lint.Analyze(ass.Dialogue{Text: input})
			for _, d := range diagnostics {
				if d.FixSafety == lint.SafeFix {
					t.Fatalf("unproven edit marked SafeFix after a prefix collision: %#v", d)
				}
			}
			fixed, count, err := lint.ApplyFixes(input, diagnostics, false)
			if err != nil || count != 0 || fixed != input {
				t.Fatalf("default --fix changed ambiguous prefix input: output=%q edits=%d err=%v", fixed, count, err)
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
