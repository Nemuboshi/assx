package lsp

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"strings"
	"testing"

	"assx/internal/sourcepos"
)

func TestStyleNavigationUsesExactUniqueDefinition(t *testing.T) {
	const uri = "file:///styles.ass"
	text := "[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: 星,Arial,20\nStyle: Other,Arial,20\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,星,,0,0,0,,x\nDialogue: 0,0:00:01.00,0:00:02.00,Other,,0,0,0,,{\\r星}x\n"
	var server Server
	if err := server.documents.Open(uri, 1, text); err != nil {
		t.Fatal(err)
	}
	doc := server.documents.items[uri]
	position, err := doc.index.Position(strings.Index(text, "星,,"))
	if err != nil {
		t.Fatal(err)
	}
	definitions, err := server.definition(t.Context(), languageParams(t, uri, position))
	if err != nil || len(definitions) != 1 || definitions[0].Range.Start.Line != 2 || definitions[0].Range.Start.Character != 7 {
		t.Fatalf("definition = %+v, %v", definitions, err)
	}
	references, err := server.references(t.Context(), languageParams(t, uri, position))
	if err != nil || len(references) != 2 || references[0].Range.Start.Line != 6 || references[1].Range.Start.Line != 7 {
		t.Fatalf("references = %+v, %v", references, err)
	}
	request, err := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}})
	if err != nil {
		t.Fatal(err)
	}
	symbols, err := server.documentSymbols(t.Context(), jsontext.Value(request))
	if err != nil || len(symbols) == 0 {
		t.Fatalf("symbols = %+v, %v", symbols, err)
	}
}

func TestDuplicateStyleDoesNotProduceDefinition(t *testing.T) {
	const uri = "file:///duplicate.ass"
	text := "[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Twin,Arial,20\nStyle: Twin,Arial,30\n[Events]\nDialogue: 0,0:00:00.00,0:00:01.00,Twin,,0,0,0,,x\n"
	var server Server
	if err := server.documents.Open(uri, 1, text); err != nil {
		t.Fatal(err)
	}
	position, err := server.documents.items[uri].index.Position(strings.LastIndex(text, "Twin"))
	if err != nil {
		t.Fatal(err)
	}
	definitions, err := server.definition(t.Context(), languageParams(t, uri, position))
	if err != nil || len(definitions) != 0 {
		t.Fatalf("duplicate definition = %+v, %v", definitions, err)
	}
}

func TestSymbolPositionInvalidRequest(t *testing.T) {
	var server Server
	_, err := server.definition(t.Context(), languageParams(t, "file:///missing.ass", sourcepos.Position{Line: 0}))
	if err != nil {
		t.Fatal(err)
	}
}

func TestDocumentSymbolsGroupRecordsUnderRepeatedSections(t *testing.T) {
	const uri = "file:///grouped.ass"
	text := "[V4+ Styles]\nStyle: A,Arial,20\n[Events]\nDialogue: 0,0:00:00.00,0:00:01.00,A,,0,0,0,,x\n[Events]\nDialogue: 0,0:00:01.00,0:00:02.00,A,,0,0,0,,y\n"
	var server Server
	if err := server.documents.Open(uri, 1, text); err != nil {
		t.Fatal(err)
	}
	request, err := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}})
	if err != nil {
		t.Fatal(err)
	}
	symbols, err := server.documentSymbols(t.Context(), request)
	if err != nil || len(symbols) != 3 || symbols[0].Name != "V4+ Styles" || symbols[1].Name != "Events" || symbols[2].Name != "Events" {
		t.Fatalf("sections = %+v, %v", symbols, err)
	}
	for _, symbol := range symbols {
		if len(symbol.Children) != 1 || symbol.Range.End.Line < symbol.Children[0].Range.End.Line {
			t.Fatalf("section bounds or child = %+v", symbol)
		}
	}
}

func TestDefaultStyleAliasAndCaseSensitiveNames(t *testing.T) {
	const uri = "file:///case.ass"
	text := "[V4+ Styles]\nStyle: Default,Arial,20\nStyle: star,Arial,20\n[Events]\nDialogue: 0,0:00:00.00,0:00:01.00,*default,,0,0,0,,x\nDialogue: 0,0:01.00,0:00:02.00,Star,,0,0,0,,x\n"
	var server Server
	if err := server.documents.Open(uri, 1, text); err != nil {
		t.Fatal(err)
	}
	doc := server.documents.items[uri]
	for _, tc := range []struct {
		needle string
		want   int
	}{{"*default", 1}, {"Star,,", 0}} {
		position, err := doc.index.Position(strings.Index(text, tc.needle) + 1)
		if err != nil {
			t.Fatal(err)
		}
		links, err := server.definition(t.Context(), languageParams(t, uri, position))
		if err != nil || len(links) != tc.want {
			t.Fatalf("%q links = %+v, %v", tc.needle, links, err)
		}
	}
}
