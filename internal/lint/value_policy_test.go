package lint

import (
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/spec"
)

// validateIR must read the range from the resolved policy, because the policy
// now comes from the profile's matrix row.
func TestValueCheckUsesResultPolicy(t *testing.T) {
	tag := ass.Tag{Name: "q", Raw: `\q61`, Start: 0, End: 4, Column: 1, Args: []string{"61"}}
	for _, test := range []struct {
		name string
		max  int
		want bool
	}{
		{"outside the renderer range", 60, true},
		{"inside the renderer range", 100, false},
	} {
		policy := spec.TagSpec{Value: spec.IntegerValue, Min: 0, Max: test.max}
		analyzer := dialogueAnalyzer{}
		analyzer.validateIR(tag, ass.DecodeTagWithSpec(tag, policy, true), false)
		got := false
		for _, diagnostic := range analyzer.diagnostics {
			if diagnostic.Condition == IssueInvalidValue {
				got = true
			}
		}
		if got != test.want {
			t.Errorf("%s: value finding = %v, want %v", test.name, got, test.want)
		}
	}
}
