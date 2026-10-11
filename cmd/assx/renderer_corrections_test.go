package main

import (
	"strings"
	"testing"

	"assx/internal/ass"
	"assx/internal/lint"
)

// Independent of snapshots: pinned prefix dispatch must not reuse legacy names.
func TestDefaultProfilePrefixCorrections(t *testing.T) {
	for _, tc := range []struct{ source, matched, portable string }{
		{`{\fsvp6}x`, "fs", "fsvp"},
		{`{\frs10}x`, "fr", "frs"},
		{`{\blend(add)}x`, "b", "blend"},
	} {
		t.Run(tc.portable, func(t *testing.T) {
			var invalid, portability bool
			for _, d := range lint.Analyze(ass.Dialogue{Text: tc.source}) {
				invalid = invalid || d.ID == lint.IssueInvalidValue && d.Tag == tc.matched && d.Severity == lint.Error && d.Column == 3
				portability = portability || d.ID == lint.IssueVSFilterModTag && d.Tag == tc.portable && d.Severity == lint.Warning
				if d.FixSafety != "" || len(d.Edits) != 0 {
					t.Fatal("prefix collision authorized an edit")
				}
			}
			if !invalid || !portability {
				t.Fatalf("missing actual-prefix validity or purposeful portability: invalid=%t portable=%t", invalid, portability)
			}
		})
	}
}

func TestDefaultCompatibilityAddsScopedObservations(t *testing.T) {
	path := writeRendererFixture(t, `{\pos(1,2)}x`)
	for _, args := range [][]string{nil, {"--compat="}} {
		var out, errOut strings.Builder
		if code := run(append(append([]string{"--format=json"}, args...), path), &out, &errOut); code != 0 {
			t.Fatalf("compatibility changed primary success: %d %s", code, errOut.String())
		}
		paired := strings.Contains(out.String(), " / xy-VSFilter@")
		if paired != (args == nil) {
			t.Fatalf("compatibility selection not reflected: args=%v paired=%t", args, paired)
		}
	}
}

func TestRejectedPositionKeepsStrictErrorAndFixFence(t *testing.T) {
	source := `{\fs21\fs21\pos(1)\pos(4,5)}x`
	diagnostics := lint.Analyze(ass.Dialogue{Text: source})
	var strict bool
	for _, diagnostic := range diagnostics {
		strict = strict || diagnostic.ID == lint.IssueArgumentCount && diagnostic.Severity == lint.Error
		if diagnostic.FixSafety == lint.SafeFix {
			t.Fatal("malformed position lost its fix fence")
		}
	}
	fixed, count, err := lint.ApplyFixes(source, diagnostics, false)
	if !strict || err != nil || count != 0 || fixed != source {
		t.Fatalf("strictness/source equality: strict=%t count=%d err=%v", strict, count, err)
	}
}
