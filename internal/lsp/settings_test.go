package lsp

import (
	"bufio"
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"io"
	"testing"

	"assx/internal/ass/renderer"
	"assx/internal/lint"
)

func TestSettingsUseDefaultTargetsAndValidateBuildGates(t *testing.T) {
	defaults, err := parseSettings(jsontext.Value(`{}`))
	if err != nil || defaults.Options.Targets.Primary.Kind() != renderer.Libass || len(defaults.Options.Targets.Compatibility) != 1 || defaults.Options.Targets.Compatibility[0].Kind() != renderer.XYVSFilter {
		t.Fatalf("defaults = %+v, %v", defaults, err)
	}
	explicit, err := parseSettings(jsontext.Value(`{"assx":{"renderers":["xy-vsfilter"]}}`))
	if err != nil || len(explicit.Options.Targets.Compatibility) != 0 {
		t.Fatalf("explicit primary must not retain default compatibility: %+v, %v", explicit, err)
	}
	settings, err := parseSettings(jsontext.Value(`{"assx":{"renderers":["vsfiltermod","libass"],"rules":{"ASS007":{"disabled":true}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if settings.Options.Targets.Primary.Kind() != renderer.VSFilterMod || len(settings.Options.Targets.Compatibility) != 1 || settings.Options.Targets.Compatibility[0].Kind() != renderer.Libass || len(settings.Options.Rules) != 1 || settings.Options.Rules[0].ID != lint.IssueUnknownTag {
		t.Fatalf("settings = %+v", settings)
	}
	deduped, err := parseSettings(jsontext.Value(`{"assx":{"renderers":["libass","libass","xy-vsfilter"]}}`))
	if err != nil || deduped.Options.Targets.Primary.Kind() != renderer.Libass || len(deduped.Options.Targets.Compatibility) != 1 || deduped.Options.Targets.Compatibility[0].Kind() != renderer.XYVSFilter {
		t.Fatalf("deduped renderers = %+v, %v", deduped, err)
	}
	for _, invalid := range []string{
		`{"assx":{"renderers":["bogus"]}}`,
		`{"assx":{"vsfilterMod":{"mod":"yes"}}}`,
		`{"assx":{"rules":{"UNKNOWN":{"disabled":true}}}}`,
		`{"assx":{"compatibilityRenderers":["libass","libass"]}}`,
	} {
		if _, err := parseSettings(jsontext.Value(invalid)); err == nil {
			t.Errorf("accepted invalid settings %s", invalid)
		}
	}
}

func TestSessionConfigurationRefreshesAllOpenDocuments(t *testing.T) {
	input := framedMessages(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"initializationOptions":{"assx":{"renderers":["xy-vsfilter"]}}}}`,
		`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///b.ass","version":1,"text":"[Events]\nDialogue: broken\n"}}}`,
		`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///a.ass","version":1,"text":"[Events]\nDialogue: broken\n"}}}`,
		`{"jsonrpc":"2.0","method":"workspace/didChangeConfiguration","params":{"settings":{"assx":{"renderers":["libass"]}}}}`,
		`{"jsonrpc":"2.0","method":"workspace/didChangeConfiguration","params":{"settings":{"assx":{"renderers":["bogus"]}}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`)
	var server Server
	var output, warnings bytes.Buffer
	server.Log = &warnings
	if err := server.Run(t.Context(), bytes.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	if server.settings.Options.Targets.Primary.Kind() != renderer.Libass || warnings.Len() == 0 {
		t.Fatalf("settings = %+v; warnings = %q", server.settings, warnings.String())
	}
	reader := bufio.NewReader(&output)
	var uris []string
	for {
		payload, err := readFrame(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		var message struct {
			Method string `json:"method"`
			Params struct {
				URI string `json:"uri"`
			} `json:"params"`
		}
		if err := json.Unmarshal(payload, &message); err != nil {
			t.Fatal(err)
		}
		if message.Method == "textDocument/publishDiagnostics" {
			uris = append(uris, message.Params.URI)
		}
	}
	want := []string{"file:///b.ass", "file:///a.ass", "file:///a.ass", "file:///b.ass"}
	if len(uris) != len(want) {
		t.Fatalf("publish order = %v, want %v", uris, want)
	}
	for i, uri := range want {
		if uris[i] != uri {
			t.Fatalf("publish order = %v, want %v", uris, want)
		}
	}
}
