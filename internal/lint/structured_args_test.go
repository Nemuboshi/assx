package lint

import (
	"strings"
	"testing"
)

// TestStructuredTailFindingsHaveNoSafeFix pins the prefix/exact split for
// structured arguments with ignored tails. The renderer value stays unknown,
// so no edit may be authorized.
func TestStructuredTailFindingsHaveNoSafeFix(t *testing.T) {
	cases := []struct {
		text      string
		condition string
	}{
		{`{\pos(10junk,20)}A`, IssueRendererUnresolved},
		{`{\move(0,0,100junk,100)}A`, IssueRendererUnresolved},
		{`{\clip(1.5,0,30junk,40)}A`, IssueRendererUnresolved},
		{`{\clip(1,0,30)}A`, IssueArgumentCount},
		{`{\clip(m 0 0 l 1 1)}A`, IssueRendererUnresolved},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			doc := parseFontOverrideText(tc.text)
			findings := analyzeDocumentTest(doc)
			var found bool
			for _, diagnostic := range findings {
				if diagnostic.Condition == tc.condition {
					found = true
				}
				if diagnostic.FixSafety == SafeFix || len(diagnostic.Edits) != 0 {
					t.Fatalf("structured tail authorized a fix: %#v", diagnostic)
				}
			}
			if !found {
				t.Fatalf("condition %s is missing", tc.condition)
			}
			fixed, count, err := ApplyFixes(doc.Text, findings, false)
			if err != nil || count != 0 || fixed != doc.Text {
				t.Fatalf("structured tail fix was applied: count=%d err=%v", count, err)
			}
		})
	}
}

// TestMalformedTagKeepsOtherDialoguesFixable covers the positive end-to-end
// case: an unresolved tail on one line must not revoke a safe edit on another.
func TestMalformedTagKeepsOtherDialoguesFixable(t *testing.T) {
	doc := htmlFixtureDocDialogues(`{\pos(10junk,20)}A`, `{\b0\b0}plain`)
	findings := analyzeDocumentTest(doc)
	fixed, count, err := ApplyFixes(doc.Text, findings, false)
	if err != nil || count != 1 {
		t.Fatalf("apply: count=%d err=%v", count, err)
	}
	if !strings.Contains(fixed, `{\pos(10junk,20)}A`) {
		t.Errorf("malformed dialogue changed: %q", fixed)
	}
	if !strings.Contains(fixed, ",,plain") {
		t.Errorf("safe dialogue was not fixed: %q", fixed)
	}
}
