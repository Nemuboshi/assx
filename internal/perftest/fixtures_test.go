package perftest_test

import (
	"testing"

	"assx/internal/ass"
	"assx/internal/perftest"
)

func TestPerformanceFixturesHaveCompleteEvents(t *testing.T) {
	wantCounts := map[string]int{
		"dialogue":    96,
		"typesetting": 48,
		"drawing":     32,
		"transforms":  48,
		"long_10000":  perftest.LongDialogueCount,
	}
	for _, test := range perftest.Cases(t) {
		t.Run(test.Name, func(t *testing.T) {
			doc := ass.Parse(test.Text)
			if len(doc.Dialogues) != wantCounts[test.Name] {
				t.Fatalf("got %d dialogues, want %d", len(doc.Dialogues), wantCounts[test.Name])
			}
			if len(doc.StyleFields) == 0 {
				t.Fatal("no styles parsed")
			}
			for _, dialogue := range doc.Dialogues {
				if dialogue.MissingFields {
					t.Fatalf("incomplete event at line %d", dialogue.Line)
				}
				if len(dialogue.ParsedText().Nodes) == 0 {
					t.Fatalf("empty parsed text at line %d", dialogue.Line)
				}
			}
		})
	}
}
