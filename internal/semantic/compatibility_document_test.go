package semantic

import (
	"assx/internal/ass"
	"strings"
	"testing"
)

func TestDocumentCompatibilitySourceAlignedLayoutsAndTags(t *testing.T) {
	source := "[Script Info]\r\nScriptType: v4.00+\r\n[V4+ Styles]\r\nFormat: Fontname, Name, Fontsize\r\nStyle: Arial,Default,20\r\n[Events]\r\nFormat: Start, End, Layer, Style, Actor, MarginL, MarginR, MarginV, Effect, Text\r\nDialogue: 0:00:00.00,0:00:01.00,0,Default,,0,0,0,,{\\pos(1,,2)\\pos(4,5)}A\r\n"
	findings := CompareDocument(ass.ParseConcreteDocument(source), nil)
	var style, event, args, ownership bool
	for _, f := range findings {
		if f.Dimension == "document/style-layout" && f.Status == CompatibilityDivergent {
			style = true
		}
		if f.Dimension == "document/dialogue-layout" && f.Status == CompatibilityDivergent {
			event = true
		}
		if f.Dimension == "arguments" && f.Status == CompatibilityDivergent {
			args = true
			if source[f.Source.Start:f.Source.End] != `\pos(1,,2)` {
				t.Fatal("dialogue span is not absolute document source")
			}
		}
		if f.Dimension == "ownership" && f.Status == CompatibilityDivergent {
			ownership = true
		}
	}
	if !style || !event || !args || !ownership {
		t.Fatalf("missing comparisons: style=%t event=%t args=%t ownership=%t", style, event, args, ownership)
	}
}

func TestDocumentCompatibilityUnresolvedDialectAndDistinctText(t *testing.T) {
	for _, source := range []string{
		"[Events]\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\fs20}A\n",
		"[Script Info]\nScriptType: v4.00++\n[Events]\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\fs20}A\n",
		"[Script Info]\nScriptType: v4.00+\n[Events]\nFormat: Text\nDialogue: {\\fs20}A\n",
	} {
		findings := CompareDocument(ass.ParseConcreteDocument(source), nil)
		if len(findings) == 0 {
			t.Fatal("dropped unresolved document")
		}
		unresolved := false
		for _, f := range findings {
			if f.Status == CompatibilityUnresolved {
				unresolved = true
			}
			if f.Dimension == "state" && f.Status == CompatibilityEquivalent {
				t.Fatal("document context invented state equivalence")
			}
		}
		if !unresolved {
			t.Fatalf("no unresolved result for %q", source)
		}
	}
}

func TestDocumentCompatibilityIgnoresMalformedFormatKeywords(t *testing.T) {
	for _, source := range []string{
		"[Script Info]\nScriptType: v4.00+\n[Events]\nformat: Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,A\n",
		"[Script Info]\nScriptType: v4.00+\n[Events]\nFormat : Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,A\n",
	} {
		for _, f := range CompareDocument(ass.ParseConcreteDocument(source), nil) {
			if strings.HasSuffix(f.Dimension, "-layout") && f.Status != CompatibilityEquivalent {
				t.Fatalf("libass consumed an ignored Format keyword: %#v", f)
			}
		}
	}
}

func TestDocumentCompatibilityActorAlias(t *testing.T) {
	source := "[Script Info]\nScriptType: v4.00+\n[Events]\nFormat: Layer, Start, End, Style, Actor, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,A\n"
	for _, f := range CompareDocument(ass.ParseConcreteDocument(source), nil) {
		if strings.HasSuffix(f.Dimension, "-layout") && f.Status != CompatibilityEquivalent {
			t.Fatalf("Actor alias introduced a false difference: %#v", f)
		}
	}
}
