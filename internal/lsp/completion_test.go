package lsp

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompletionUsesRendererEligibilityAndUTF16Edit(t *testing.T) {
	const uri = "file:///complete.ass"
	text := "[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,😀{\\fsv}x\n"
	var server Server
	if err := server.documents.Open(uri, 1, text); err != nil {
		t.Fatal(err)
	}
	position, err := server.documents.items[uri].index.Position(strings.Index(text, `\fsv`) + len(`\fsv`))
	if err != nil {
		t.Fatal(err)
	}
	items, err := server.completion(t.Context(), languageParams(t, uri, position))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Label == `\fsvp` && strings.Contains(item.Detail, "verified") {
			t.Fatalf("default renderer certified Mod command: %+v", item)
		}
	}
	configured, err := parseSettings([]byte(`{"assx":{"primaryRenderer":"vsfiltermod","compatibilityRenderers":[],"vsfilterMod":{"mod":"enabled"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	server.settings = configured
	items, err = server.completion(t.Context(), languageParams(t, uri, position))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Label == `\fsvp` {
			if item.TextEdit.Range.Start.Character != position.Character-3 || item.TextEdit.NewText != "fsvp" {
				t.Fatalf("completion edit = %+v", item)
			}
			return
		}
	}
	t.Fatal("missing enabled Mod completion")
}

func TestInitializeNegotiatesCompletionSnippets(t *testing.T) {
	input := framedMessages(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"capabilities":{"textDocument":{"completion":{"completionItem":{"snippetSupport":true}}}}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`)
	var server Server
	var output bytes.Buffer
	if err := server.Run(t.Context(), bytes.NewReader(input), &output); err != nil || !server.snippetSupport {
		t.Fatalf("snippet negotiation = %t, %v", server.snippetSupport, err)
	}
}

func TestCompletionOffersUniqueStylesOnlyInStyleField(t *testing.T) {
	const uri = "file:///styles.ass"
	text := "[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Star,Arial,20\nStyle: Twin,Arial,20\nStyle: Twin,Arial,30\n[Events]\nDialogue: 0,0:00:00.00,0:00:01.00,St,,0,0,0,,x\n"
	var server Server
	if err := server.documents.Open(uri, 1, text); err != nil {
		t.Fatal(err)
	}
	position, err := server.documents.items[uri].index.Position(strings.LastIndex(text, "St,") + 2)
	if err != nil {
		t.Fatal(err)
	}
	items, err := server.completion(t.Context(), languageParams(t, uri, position))
	if err != nil || len(items) != 1 || items[0].Label != "Star" {
		t.Fatalf("Style completions = %+v, %v", items, err)
	}
	position, err = server.documents.items[uri].index.Position(strings.LastIndex(text, "x"))
	if err != nil {
		t.Fatal(err)
	}
	items, err = server.completion(t.Context(), languageParams(t, uri, position))
	if err != nil || len(items) != 0 {
		t.Fatalf("plain text completions = %+v, %v", items, err)
	}
}

func TestCompletionOffersNestedTagsAndResetStyles(t *testing.T) {
	const uri = "file:///nested.ass"
	text := "[V4+ Styles]\nStyle: Star,Arial,20\n[Events]\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\t(0,100,\\po)\\rSt}x\n"
	var server Server
	if err := server.documents.Open(uri, 1, text); err != nil {
		t.Fatal(err)
	}
	doc := server.documents.items[uri]
	for _, tc := range []struct {
		needle string
		label  string
		want   string
	}{{`\po`, `\pos`, "pos"}, {`\rSt`, "Star", "Star"}} {
		position, err := doc.index.Position(strings.Index(text, tc.needle) + len(tc.needle))
		if err != nil {
			t.Fatal(err)
		}
		items, err := server.completion(t.Context(), languageParams(t, uri, position))
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, item := range items {
			if item.Label == tc.label && item.TextEdit.NewText == tc.want {
				found = true
			}
		}
		if !found {
			t.Fatalf("%q completion = %+v", tc.needle, items)
		}
	}
}

func TestCompletionSnippetRequiresClientSupport(t *testing.T) {
	const uri = "file:///snippet.ass"
	text := "[Events]\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\po}x\n"
	var server Server
	if err := server.documents.Open(uri, 1, text); err != nil {
		t.Fatal(err)
	}
	position, err := server.documents.items[uri].index.Position(strings.Index(text, `\po`) + 3)
	if err != nil {
		t.Fatal(err)
	}
	request := languageParams(t, uri, position)
	find := func() completionItem {
		t.Helper()
		items, err := server.completion(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range items {
			if item.Label == `\pos` {
				return item
			}
		}
		t.Fatal("missing pos completion")
		return completionItem{}
	}
	plain := find()
	if plain.TextEdit.NewText != "pos" || plain.InsertTextFormat != 0 {
		t.Fatalf("plain completion = %+v", plain)
	}
	server.snippetSupport = true
	snippet := find()
	if snippet.InsertTextFormat != 2 || snippet.TextEdit.NewText != "pos(${1:arg1},${2:arg2})" {
		t.Fatalf("snippet completion = %+v", snippet)
	}
}

func TestCompletionOffersVerifiedIntegerArgumentHints(t *testing.T) {
	const uri = "file:///values.ass"
	text := "[Events]\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\an}x\n"
	var server Server
	if err := server.documents.Open(uri, 1, text); err != nil {
		t.Fatal(err)
	}
	position, err := server.documents.items[uri].index.Position(strings.Index(text, `\an`) + 3)
	if err != nil {
		t.Fatal(err)
	}
	items, err := server.completion(t.Context(), languageParams(t, uri, position))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Label == "1" && item.TextEdit.NewText == "1" && item.TextEdit.Range.Start == position {
			return
		}
	}
	t.Fatalf("no alignment value hint: %+v", items)
}
