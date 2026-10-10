package perftest_test

import (
	"strings"
	"testing"
	"unicode/utf8"

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

// These checks protect the *shape* of each performance workload. A benchmark
// can continue to run after all its demanding syntax is accidentally deleted,
// which would silently invalidate comparisons against the existing baseline.
// Inspect parsed structures rather than relying only on substrings.
func TestPerformanceFixtureFeatures(t *testing.T) {
	for _, test := range perftest.Cases(t) {
		t.Run(test.Name, func(t *testing.T) {
			features := inspectFixture(ass.Parse(test.Text))
			requireFeature := func(name string, count int) {
				t.Helper()
				if count < 1 {
					t.Errorf("missing required workload feature: %s", name)
				}
			}

			switch test.Name {
			case "dialogue":
				if features.plainDialogues < 80 {
					t.Errorf("mostly plain dialogue expected: got %d of 96", features.plainDialogues)
				}
				requireFeature("multilingual dialogue", features.nonASCIIDialogues)
				requireFeature("ASS line-break escape", features.lineBreaks)
			case "typesetting":
				for _, tag := range []string{"pos", "move", "r", "k", "clip", "iclip", "fad"} {
					requireFeature("typesetting \\"+tag, features.tags[tag])
				}
				if len(features.usedStyles) < 2 {
					t.Errorf("expected multiple referenced Styles, got %d", len(features.usedStyles))
				}
			case "drawing":
				requireFeature("drawing mode \\p", features.tags["p"])
				requireFeature("parsed vector drawing", features.drawings)
				requireFeature("vector line command", features.lineCommands)
				requireFeature("vector bezier command", features.bezierCommands)
				requireFeature("vector \\clip", features.vectorClip)
				requireFeature("vector \\iclip", features.inverseVectorClip)
			case "transforms":
				requireFeature("animation \\t", features.tags["t"])
				requireFeature("nested \\t with nested AST children", features.nestedTransforms)
				requireFeature("multiple transforms in one dialogue", features.multipleTransforms)
				requireFeature("Style reset \\r", features.tags["r"])
				requireFeature("transformed position/scale/border", features.tags["fscx"]+features.tags["bord"]+features.tags["frz"])
			case "long_10000":
				requireFeature("plain dialogue", features.plainDialogues)
				requireFeature("multilingual dialogue", features.nonASCIIDialogues)
				requireFeature("typesetting position", features.tags["pos"])
				requireFeature("nested transforms", features.nestedTransforms)
				requireFeature("parsed vector drawings", features.drawings)
				requireFeature("bezier paths", features.bezierCommands)
				requireFeature("vector clips", features.vectorClip+features.inverseVectorClip)
			default:
				t.Fatalf("missing fixture feature assertions for %q", test.Name)
			}
		})
	}
}

type fixtureFeatures struct {
	tags               map[string]int
	usedStyles         map[string]bool
	plainDialogues     int
	nonASCIIDialogues  int
	lineBreaks         int
	drawings           int
	lineCommands       int
	bezierCommands     int
	vectorClip         int
	inverseVectorClip  int
	nestedTransforms   int
	multipleTransforms int
}

func inspectFixture(doc ass.Document) fixtureFeatures {
	f := fixtureFeatures{
		tags:       make(map[string]int),
		usedStyles: make(map[string]bool),
	}
	for _, dialogue := range doc.Dialogues {
		f.usedStyles[dialogue.Style] = true
		if utf8.RuneCountInString(dialogue.Text) != len(dialogue.Text) {
			f.nonASCIIDialogues++
		}
		if strings.Contains(dialogue.Text, "\\N") {
			f.lineBreaks++
		}

		tree := dialogue.ParsedText()
		hasOverrides := false
		localTransforms := 0
		for _, node := range tree.Nodes {
			if node.Kind != ass.OverrideNode {
				continue
			}
			hasOverrides = true
			if node.Block == nil {
				continue
			}
			for _, item := range node.Block.Items {
				if item.Tag != nil {
					f.nestedTransforms += countNestedTransforms(*item.Tag)
				}
			}
		}
		if !hasOverrides {
			f.plainDialogues++
		}

		tree.WalkTokens(func(token ass.TokenView) bool {
			if !token.HasTag {
				return true
			}
			tag := token.Tag
			f.tags[tag.Name]++
			if tag.Name == "t" {
				localTransforms++
			}
			if (tag.Name == "clip" || tag.Name == "iclip") && len(tag.Args) > 0 {
				vector := strings.TrimSpace(tag.Args[len(tag.Args)-1])
				if strings.HasPrefix(vector, "m ") {
					if tag.Name == "clip" {
						f.vectorClip++
					} else {
						f.inverseVectorClip++
					}
				}
			}
			return true
		})
		if localTransforms >= 2 {
			f.multipleTransforms++
		}
		for _, drawing := range tree.Drawings() {
			if len(drawing.Commands) == 0 {
				continue
			}
			f.drawings++
			for _, command := range drawing.Commands {
				switch command.Name {
				case 'l':
					if len(command.Coordinates) >= 2 {
						f.lineCommands++
					}
				case 'b':
					if len(command.Coordinates) >= 6 {
						f.bezierCommands++
					}
				}
			}
		}
	}
	return f
}

func countNestedTransforms(tag ass.Tag) int {
	count := 0
	for _, child := range tag.Children {
		if tag.Name == "t" && child.Name == "t" {
			count++
		}
		count += countNestedTransforms(child)
	}
	return count
}
