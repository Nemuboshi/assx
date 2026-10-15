package lint

import (
	"testing"

	"assx/internal/ass"
)

// ASS v4+ Alignment is a numpad value. The pinned renderers place text for 1
// to 9 only, so a value outside that range is reported. Legacy SSA uses a
// bitfield mapping, so its Alignment is not checked.
func TestStyleAlignmentNumpadRange(t *testing.T) {
	assDoc := func(value string) string {
		return "[V4+ Styles]\nFormat: Name, Alignment\nStyle: Default," + value + "\n"
	}
	ssaDoc := func(value string) string {
		return "[V4 Styles]\nFormat: Name, Alignment\nStyle: Default," + value + "\n"
	}
	cases := []struct {
		name  string
		doc   string
		value string
		want  bool
	}{
		{"ass minimum", assDoc("1"), "1", false},
		{"ass maximum", assDoc("9"), "9", false},
		{"ass middle", assDoc("5"), "5", false},
		{"ass zero", assDoc("0"), "0", true},
		{"ass above range", assDoc("10"), "10", true},
		{"ass negative", assDoc("-1"), "-1", true},
		{"ass fractional", assDoc("2.5"), "2.5", false},
		{"ass empty", assDoc(""), "", false},
		{"ssa bitfield", ssaDoc("10"), "10", false},
		{"ssa zero", ssaDoc("0"), "0", false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			doc := ass.Parse(test.doc)
			findings := diagnosticsForIDs(AnalyzeStyles(doc), IssueStyleAlignment)
			if (len(findings) != 0) != test.want {
				t.Fatalf("Alignment %q: findings = %#v, want report=%t", test.value, findings, test.want)
			}
			for _, finding := range findings {
				if finding.Severity != Warning || finding.FixSafety != "" || len(finding.Edits) != 0 {
					t.Fatalf("Alignment finding must be a warning without a fix: %#v", finding)
				}
				if finding.Field != "Default.alignment" {
					t.Fatalf("Alignment finding field = %q", finding.Field)
				}
			}
		})
	}
}

// A fractional Alignment value is an ASS011 concern, not an ASS032 concern, so
// the two rules never both report the same field.
func TestStyleAlignmentFractionalLeftToASS011(t *testing.T) {
	doc := ass.Parse("[V4+ Styles]\nFormat: Name, Alignment\nStyle: Default,0.5\n")
	if findings := diagnosticsForIDs(AnalyzeStyles(doc), IssueStyleAlignment); len(findings) != 0 {
		t.Fatalf("fractional Alignment must not be an ASS032 finding: %#v", findings)
	}
	if findings := diagnosticsForIDs(AnalyzeStyles(doc), IssueStyleInteger); len(findings) == 0 {
		t.Fatalf("fractional Alignment must stay an ASS011 finding")
	}
}
