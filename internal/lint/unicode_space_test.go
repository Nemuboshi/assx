package lint

import "testing"

// TestUnicodeSpaceArgumentsHaveNoSafeFix keeps non-ASCII whitespace from
// becoming an exact argument value. The renderers do not treat these runes as
// numeric whitespace, so a fix that assumes the value must stay unavailable.
func TestUnicodeSpaceArgumentsHaveNoSafeFix(t *testing.T) {
	spaces := []string{"\u00a0", "\u2003", "\u3000"}
	for _, space := range spaces {
		texts := []string{
			"{\\fs20\\fs" + space + "20}A",
			"{\\pos(0,0)\\pos(" + space + "0,0)}A",
			"{\\clip(0,0,10,10)\\clip(" + space + "0,0,10,10)}A",
			"{\\a1\\a" + space + "1}A",
		}
		for _, text := range texts {
			t.Run(text, func(t *testing.T) {
				doc := parseFontOverrideText(text)
				findings := analyzeDocumentTest(doc)
				if len(findings) == 0 {
					t.Fatal("expected at least one diagnostic")
				}
				for _, diagnostic := range findings {
					if diagnostic.FixSafety == SafeFix || len(diagnostic.Edits) != 0 {
						t.Fatalf("unicode space authorized a fix: %#v", diagnostic)
					}
				}
				fixed, count, err := ApplyFixes(doc.Text, findings, false)
				if err != nil || count != 0 || fixed != doc.Text {
					t.Fatalf("unicode space fix was applied: count=%d err=%v", count, err)
				}
			})
		}
	}
}

// TestASCIISpaceArgumentsKeepSafeFixes is the control for the Unicode test.
func TestASCIISpaceArgumentsKeepSafeFixes(t *testing.T) {
	doc := parseFontOverrideText("{\\fs20\\fs 20}A")
	findings := analyzeDocumentTest(doc)
	fixed, count, err := ApplyFixes(doc.Text, findings, false)
	if err != nil || count == 0 || fixed == doc.Text {
		t.Fatalf("ascii space fix was lost: count=%d err=%v", count, err)
	}
}
