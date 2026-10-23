package lsp

import (
	"bufio"
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"strings"
	"testing"

	"assx/internal/sourcepos"
)

func languageParams(t *testing.T, uri string, position sourcepos.Position) jsontext.Value {
	t.Helper()
	value, err := json.Marshal(map[string]any{
		"textDocument": map[string]string{"uri": uri},
		"position":     position,
	})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestSessionAdvertisesAndServesHoverAndSignatureHelp(t *testing.T) {
	const uri = "file:///language.ass"
	input := framedMessages(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+uri+`","version":1,"text":"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\pos(1,2)}x\n"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":"`+uri+`"},"position":{"line":2,"character":53}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"textDocument/signatureHelp","params":{"textDocument":{"uri":"`+uri+`"},"position":{"line":2,"character":59}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`)
	var server Server
	var output bytes.Buffer
	if err := server.Run(t.Context(), bytes.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(&output)
	var gotCapabilities, gotHover, gotSignature bool
	for range 5 {
		payload, err := readFrame(reader)
		if err != nil {
			t.Fatal(err)
		}
		var message struct {
			ID     jsontext.Value `json:"id"`
			Result jsontext.Value `json:"result"`
		}
		if err := json.Unmarshal(payload, &message); err != nil {
			t.Fatal(err)
		}
		switch string(message.ID) {
		case "1":
			gotCapabilities = bytes.Contains(message.Result, []byte(`"hoverProvider":true`)) && bytes.Contains(message.Result, []byte(`"signatureHelpProvider"`))
		case "2":
			gotHover = bytes.Contains(message.Result, []byte(`"plaintext"`))
		case "3":
			gotSignature = bytes.Contains(message.Result, []byte(`"signatures"`))
		}
	}
	if !gotCapabilities || !gotHover || !gotSignature {
		t.Fatalf("language responses: capabilities=%t hover=%t signature=%t", gotCapabilities, gotHover, gotSignature)
	}
}

func TestHoverUsesPinnedRendererEvidenceAndIgnoresPlainText(t *testing.T) {
	const uri = "file:///hover.ass"
	text := "[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,😀{\\pos(1,2,3)}x\n"
	var server Server
	if err := server.documents.Open(uri, 1, text); err != nil {
		t.Fatal(err)
	}
	doc := server.documents.items[uri]
	position, err := doc.index.Position(strings.Index(text, `\pos`) + 2)
	if err != nil {
		t.Fatal(err)
	}
	request := languageParams(t, uri, position)
	hover, err := server.hover(t.Context(), request)
	if err != nil || hover == nil || !strings.Contains(hover.Contents.Value, "libass@") || !strings.Contains(hover.Contents.Value, "rejected") {
		t.Fatalf("libass hover = %+v, %v", hover, err)
	}
	configured, err := parseSettings(jsontext.Value(`{"assx":{"primaryRenderer":"vsfiltermod","compatibilityRenderers":[],"vsfilterMod":{"mod":"enabled","lua":"disabled"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	server.settings = configured
	hover, err = server.hover(t.Context(), request)
	if err != nil || hover == nil || !strings.Contains(hover.Contents.Value, "VSFilterMod@") || !strings.Contains(hover.Contents.Value, "verified") {
		t.Fatalf("Mod hover = %+v, %v", hover, err)
	}
	plain, err := doc.index.Position(strings.Index(text, "😀"))
	if err != nil {
		t.Fatal(err)
	}
	hover, err = server.hover(t.Context(), languageParams(t, uri, plain))
	if err != nil || hover != nil {
		t.Fatalf("plain text hover = %+v, %v", hover, err)
	}
}

func TestSignatureHelpTracksArgumentsAndNestedTransform(t *testing.T) {
	const uri = "file:///signature.ass"
	text := "[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\pos(1,2)\\t(0,100,\\pos(3,4))}x\n"
	var server Server
	if err := server.documents.Open(uri, 1, text); err != nil {
		t.Fatal(err)
	}
	doc := server.documents.items[uri]
	for _, tc := range []struct {
		needle string
		want   int
	}{
		{`1,2`, 1},
		{`3,4`, 1},
	} {
		position, err := doc.index.Position(strings.Index(text, tc.needle) + 2)
		if err != nil {
			t.Fatal(err)
		}
		help, err := server.signatureHelp(t.Context(), languageParams(t, uri, position))
		if err != nil || help == nil || help.ActiveParameter != tc.want || len(help.Signatures) == 0 || !strings.HasPrefix(help.Signatures[help.ActiveSignature].Label, `\pos(`) {
			t.Fatalf("%q signature = %+v, %v", tc.needle, help, err)
		}
	}
}

func TestHoverReportsShadowedAndConditionalDispatchWithoutVerification(t *testing.T) {
	const uri = "file:///collision.ass"
	text := "[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\fsvp6}x\n"
	var server Server
	if err := server.documents.Open(uri, 1, text); err != nil {
		t.Fatal(err)
	}
	doc := server.documents.items[uri]
	position, err := doc.index.Position(strings.Index(text, `\fsvp6`) + 2)
	if err != nil {
		t.Fatal(err)
	}
	request := languageParams(t, uri, position)
	hover, err := server.hover(t.Context(), request)
	if err != nil || hover == nil || !strings.Contains(hover.Contents.Value, `shadows \fsvp`) {
		t.Fatalf("shadowed hover = %+v, %v", hover, err)
	}
	configured, err := parseSettings(jsontext.Value(`{"assx":{"primaryRenderer":"vsfiltermod","compatibilityRenderers":[]}}`))
	if err != nil {
		t.Fatal(err)
	}
	server.settings = configured
	hover, err = server.hover(t.Context(), request)
	if err != nil || hover == nil || !strings.Contains(hover.Contents.Value, "conditional build feature") || strings.Contains(hover.Contents.Value, "verified signature") {
		t.Fatalf("conditional hover = %+v, %v", hover, err)
	}
}

func TestHoverWithholdsUnknownAndIncompleteClaims(t *testing.T) {
	const uri = "file:///partial.ass"
	text := "[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\mystery\\pos(1,2}x\n"
	var server Server
	if err := server.documents.Open(uri, 1, text); err != nil {
		t.Fatal(err)
	}
	doc := server.documents.items[uri]
	for _, tc := range []struct {
		needle string
		want   string
	}{
		{`\mystery`, "unknown name"},
		{`\pos`, "unresolved signature"},
	} {
		position, err := doc.index.Position(strings.Index(text, tc.needle) + 2)
		if err != nil {
			t.Fatal(err)
		}
		hover, err := server.hover(t.Context(), languageParams(t, uri, position))
		if err != nil || hover == nil || !strings.Contains(hover.Contents.Value, tc.want) {
			t.Fatalf("%q hover = %+v, %v", tc.needle, hover, err)
		}
	}
}

func TestSignatureHelpOnlyInsideArgumentsAndNoCommentHover(t *testing.T) {
	const uri = "file:///context.ass"
	text := "[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nComment: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\pos(1,2)}x\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\pos(1,2)}x\n"
	var server Server
	if err := server.documents.Open(uri, 1, text); err != nil {
		t.Fatal(err)
	}
	doc := server.documents.items[uri]
	for _, offset := range []int{strings.Index(text, `\pos`), strings.LastIndex(text, `\pos`) + 2} {
		position, err := doc.index.Position(offset)
		if err != nil {
			t.Fatal(err)
		}
		help, err := server.signatureHelp(t.Context(), languageParams(t, uri, position))
		if err != nil || help != nil {
			t.Fatalf("signature at %d = %+v, %v", offset, help, err)
		}
	}
	position, err := doc.index.Position(strings.Index(text, `\pos`) + 2)
	if err != nil {
		t.Fatal(err)
	}
	hover, err := server.hover(t.Context(), languageParams(t, uri, position))
	if err != nil || hover != nil {
		t.Fatalf("comment hover = %+v, %v", hover, err)
	}
}

func TestHoverDescribesUniqueStyleReferenceAndKnownScriptField(t *testing.T) {
	const uri = "file:///fields.ass"
	text := "[Script Info]\nPlayResX: 1920\n[V4+ Styles]\nStyle: Star,Arial,20\n[Events]\nDialogue: 0,0:00:00.00,0:00:01.00,Star,,0,0,0,,x\n"
	var server Server
	if err := server.documents.Open(uri, 1, text); err != nil {
		t.Fatal(err)
	}
	doc := server.documents.items[uri]
	for _, tc := range []struct {
		needle string
		want   string
	}{{"Star,,", "Style reference: Star"}, {"PlayResX", "Script field: PlayResX"}} {
		position, err := doc.index.Position(strings.Index(text, tc.needle) + 1)
		if err != nil {
			t.Fatal(err)
		}
		hover, err := server.hover(t.Context(), languageParams(t, uri, position))
		if err != nil || hover == nil || !strings.Contains(hover.Contents.Value, tc.want) {
			t.Fatalf("%q hover = %+v, %v", tc.needle, hover, err)
		}
	}
}
