package lint

import "testing"

// The reporter can still issue diagnostics, but none of these consumers may
// expose a SafeFix after a later operation with unmodeled semantics.
func TestLateUnmodeledTagsRevokeLintSafeFixes(t *testing.T) {
	cases := []struct {
		name, text string
	}{
		{"review reproduction", `{\fs20\fs20\mystery}A`},
		{"unknown after ASS006", `{\fs21\fs21\mystery}A`},
		{"unknown after ASS013", `{\fs20\mystery}A`},
		{"unknown in later block", `{\fs20}A{\mystery}B`},
		{"unknown in transform", `{\fs20}A{\t(\mystery)}B`},
		{"unknown in later transform", `{\fs20}A{\t(\mystery)}B{\fs20}C`},
		{"VSFilterMod after ASS006", `{\fs21\fs21\1img}A`},
		{"VSFilterMod after ASS013", `{\fs20}A{\1img}B`},
		{"VSFilterMod inside transform", `{\fs20}A{\t(\1img)}B`},
		{"unknown numeric assignment", `{\fs20}A{\fsabc}B`},
		{"unknown numeric transform child", `{\fs20}A{\t(\fsabc)}B`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := parseFontOverrideText(tc.text)
			for _, consumer := range []struct {
				name        string
				diagnostics []Diagnostic
			}{
				{"document lint", AnalyzeDocument(doc)},
				{"ASS013 standalone", AnalyzeRedundantStyleOverrides(doc)},
				{"ASS006 standalone", Analyze(doc.Dialogues[0])},
			} {
				t.Run(consumer.name, func(t *testing.T) {
					var relevant []Diagnostic
					for _, d := range consumer.diagnostics {
						if d.ID == IssueNoEffect || d.ID == IssueRedundantStyleOverrides {
							relevant = append(relevant, d)
							if d.FixSafety == SafeFix || len(d.Edits) != 0 {
								t.Errorf("unsafe fix survived: %#v", d)
							}
						}
					}
					fixed, count, err := ApplyFixes(doc.Text, relevant, false)
					if err != nil || count != 0 || fixed != doc.Text {
						t.Fatalf("unproven fix was applied: count=%d err=%v", count, err)
					}
				})
			}
		})
	}
}

func TestUnknownTagRetainsNonFixableASS006Diagnostic(t *testing.T) {
	doc := parseFontOverrideText(`{\fs20\fs20\mystery}A`)
	var seen bool
	for _, finding := range AnalyzeDocument(doc) {
		if finding.ID == IssueNoEffect {
			seen = true
			if finding.FixSafety != "" || len(finding.Edits) != 0 {
				t.Fatalf("no-effect diagnostic still advertises a SafeFix: %#v", finding)
			}
		}
	}
	if !seen {
		t.Fatal("previous no-effect diagnostic was lost instead of downgraded")
	}
}
