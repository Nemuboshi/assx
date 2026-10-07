package lint

import (
	"testing"

	"assx/internal/ass"
)

func TestFractionalRectClipRendererDifference(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{text: `{\clip(1.5,0,100,100)}A`, want: true},
		{text: `{\iclip(1.5,0,100,100)}A`, want: true},
		{text: `{\clip(1.4,0,100,100)}A`, want: false},
		{text: `{\clip(1,0,100,100)}A`, want: false},
	}
	for _, test := range cases {
		dialogue := ass.Dialogue{Text: test.text, Line: 1, Syntax: ass.ParseDialogueText(test.text)}
		found := false
		for _, diagnostic := range Analyze(dialogue) {
			if diagnostic.ID == IssueRendererDiff && (diagnostic.Tag == "clip" || diagnostic.Tag == "iclip") {
				found = true
			}
		}
		if found != test.want {
			t.Fatalf("%s: renderer diff=%v, want %v", test.text, found, test.want)
		}
	}
}
