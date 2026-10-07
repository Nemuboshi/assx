package semantic

import (
	"testing"

	"assx/internal/ass"
)

func TestDialogueStyleLookupSemantics(t *testing.T) {
	for _, input := range []string{"Default", "default", "DEFAULT", "*Default", "  *DEFAULT  "} {
		if got := DialogueStyleLookupName(input); got != "Default" {
			t.Fatalf("DialogueStyleLookupName(%q) = %q", input, got)
		}
	}
	if got := DialogueStyleLookupName("Main"); got != "Main" {
		t.Fatalf("Main normalized to %q", got)
	}
	if got := DialogueStyleLookupName("main"); got != "main" {
		t.Fatalf("main normalized to %q", got)
	}
}

func TestStyleDefinitionsKeepCaseDistinctAndRejectDuplicates(t *testing.T) {
	fields := []ass.StyleField{
		{StyleName: "Main", Name: "name", Value: "Main", Line: 1},
		{StyleName: "Main", Name: "fontsize", Value: "20", Line: 1},
		{StyleName: "main", Name: "name", Value: "main", Line: 2},
		{StyleName: "main", Name: "fontsize", Value: "30", Line: 2},
	}
	styles := StyleDefinitionsByName(fields)
	if styles["Main"]["fontsize"] != "20" || styles["main"]["fontsize"] != "30" {
		t.Fatalf("styles = %#v", styles)
	}

	fields = append(fields,
		ass.StyleField{StyleName: "Main", Name: "name", Value: "Main", Line: 3},
		ass.StyleField{StyleName: "Main", Name: "fontsize", Value: "40", Line: 3},
	)
	styles = StyleDefinitionsByName(fields)
	if _, ok := styles["Main"]; ok {
		t.Fatalf("duplicate Main style was treated as unambiguous: %#v", styles)
	}
	if styles["main"]["fontsize"] != "30" {
		t.Fatalf("case-distinct style disappeared: %#v", styles)
	}
}
