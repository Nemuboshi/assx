package perftest_test

import (
	"fmt"
	"strings"
	"testing"

	"assx/internal/ass"
	"assx/internal/perftest"
)

func TestPerformanceScalingFixtures(t *testing.T) {
	cases := perftest.Scaling(t)
	want := []int{250, 1000, 4000, 10000}
	if len(cases) != len(want) {
		t.Fatalf("got %d scaling fixtures, want %d", len(cases), len(want))
	}
	for i, test := range cases {
		if got, wantName := test.Name, fmt.Sprintf("scaling_%d", want[i]); got != wantName {
			t.Fatalf("unexpected scaling name %q, want %q", got, wantName)
		}
		doc := ass.Parse(test.Text)
		if len(doc.Dialogues) != want[i] {
			t.Fatalf("%s has %d dialogues, want %d", test.Name, len(doc.Dialogues), want[i])
		}
		if len(doc.StyleFields) == 0 {
			t.Fatalf("%s parsed no styles", test.Name)
		}
		for _, dialogue := range doc.Dialogues {
			if dialogue.MissingFields {
				t.Fatalf("%s has an incomplete event at line %d", test.Name, dialogue.Line)
			}
			if !strings.HasPrefix(dialogue.Text, `{\b1\b1\fs20`) {
				t.Fatalf("%s dialogue %d lacks the no-effect prefix: %q", test.Name, dialogue.Line, dialogue.Text)
			}
			if dialogue.Style != "Default" {
				t.Fatalf("%s dialogue %d uses style %q, want Default", test.Name, dialogue.Line, dialogue.Style)
			}
		}
		if i > 0 && !strings.HasPrefix(test.Text, cases[i-1].Text) {
			t.Fatalf("%s does not extend the previous scaling document", test.Name)
		}
	}
}

func TestPerformanceStyleEditFixture(t *testing.T) {
	text := perftest.StyleEdit(t)
	doc := ass.Parse(text)
	if len(doc.Dialogues) != perftest.StyleEditDialogueCount {
		t.Fatalf("got %d dialogues, want %d", len(doc.Dialogues), perftest.StyleEditDialogueCount)
	}
	marginl := ""
	for _, field := range doc.StyleFields {
		if field.StyleName == "Default" && field.Name == "marginl" {
			marginl = field.Value
		}
	}
	if marginl != "0.0" {
		t.Fatalf("Default.marginl = %q, want fractional value %q", marginl, "0.0")
	}
	for _, dialogue := range doc.Dialogues {
		if !strings.HasPrefix(dialogue.Text, `{\b1\b1\fs20\fs20\t(0,500,\fscx100)}`) {
			t.Fatalf("dialogue %d lacks the no-effect prefix: %q", dialogue.Line, dialogue.Text)
		}
	}
}
