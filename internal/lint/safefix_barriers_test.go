package lint

import "testing"

// The reporter can still issue diagnostics, but none of these consumers may
// expose a SafeFix after a later operation with unmodeled semantics.
func TestLateUnmodeledTagsRevokeLintSafeFixes(t *testing.T) {
	cases := []struct {
		name, text string
	}{
		{"review reproduction", `{\fs20\fs20\mystery}A`},
		{"unknown after no-effect", `{\fs21\fs21\mystery}A`},
		{"unknown after redundant-style", `{\fs20\mystery}A`},
		{"unknown in later block", `{\fs20}A{\mystery}B`},
		{"unknown in transform", `{\fs20}A{\t(\mystery)}B`},
		{"unknown in later transform", `{\fs20}A{\t(\mystery)}B{\fs20}C`},
		{"VSFilterMod after no-effect", `{\fs21\fs21\1img}A`},
		{"VSFilterMod after redundant-style", `{\fs20}A{\1img}B`},
		{"VSFilterMod inside transform", `{\fs20}A{\t(\1img)}B`},
		{"unknown numeric assignment", `{\fs20}A{\fsabc}B`},
		{"unknown numeric transform child", `{\fs20}A{\t(\fsabc)}B`},
		{"unknown named Style reset", `{\fs20}A{\rMissing}B`},
		{"unknown named reset after no-effect", `{\fs20\fs20}A{\rMissing}B`},
		{"absolute karaoke timing", `{\fs20}A{\kt20}B`},
		{"malformed karaoke duration", `{\fs20}A{\k20junk}B`},
		{"renderer-dependent clipping", `{\fs20}A{\clip(1.5,0,10,10)}B`},
		{"unmodeled vector clipping", `{\fs20}A{\clip(m 0 0 l 10 10)}B`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := parseFontOverrideText(tc.text)
			for _, consumer := range []struct {
				name        string
				diagnostics []Diagnostic
			}{
				{"document lint", analyzeDocumentTest(doc)},
				{"redundant-style standalone", AnalyzeRedundantStyleOverrides(doc)},
				{"no-effect standalone", Analyze(doc.Dialogues[0])},
			} {
				t.Run(consumer.name, func(t *testing.T) {
					var relevant []Diagnostic
					for _, d := range consumer.diagnostics {
						if d.Condition == IssueNoEffect || d.Condition == IssueRedundantStyleOverrides {
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
	for _, finding := range analyzeDocumentTest(doc) {
		if finding.Condition == IssueNoEffect {
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
