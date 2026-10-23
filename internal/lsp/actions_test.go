package lsp

import (
	"bufio"
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"testing"

	"assx/internal/edit"
	"assx/internal/sourcepos"
)

func TestCodeActionRequiresCurrentPublishedIdentityAndVersionedEdits(t *testing.T) {
	const uri = "file:///fix.ass"
	text := "[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\fs10\\fs20}word\n"
	var server Server
	server.editSupport = true
	if err := server.documents.Open(uri, 7, text); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := server.publish(t.Context(), &output, uri); err != nil {
		t.Fatal(err)
	}
	payload, err := readFrame(bufio.NewReader(&output))
	if err != nil {
		t.Fatal(err)
	}
	var published struct {
		Params struct {
			Diagnostics []struct {
				Code  string          `json:"code"`
				Data  string          `json:"data"`
				Range sourcepos.Range `json:"range"`
			} `json:"diagnostics"`
		} `json:"params"`
	}
	if err := json.Unmarshal(payload, &published); err != nil {
		t.Fatal(err)
	}
	var data string
	var diagnosticRange sourcepos.Range
	for _, diagnostic := range published.Params.Diagnostics {
		if diagnostic.Code == "ASS006" {
			data, diagnosticRange = diagnostic.Data, diagnostic.Range
			break
		}
	}
	if data == "" {
		t.Fatal("no identity for SafeFix candidate")
	}
	request := func(identity string) jsontext.Value {
		encoded, err := json.Marshal(map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"range":        diagnosticRange,
			"context": map[string]any{"diagnostics": []any{
				map[string]any{"code": "ASS006", "data": identity, "range": diagnosticRange},
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	actions, err := server.codeActions(t.Context(), request(data))
	if err != nil || len(actions) != 1 {
		t.Fatalf("actions = %+v, %v", actions, err)
	}
	change := actions[0].Edit.DocumentChanges[0]
	if change.TextDocument.URI != uri || change.TextDocument.Version != 7 || len(change.Edits) == 0 {
		t.Fatalf("unversioned or empty edit: %+v", change)
	}
	if len(actions[0].Edit.DocumentChanges) != 1 {
		t.Fatalf("unexpected document changes: %+v", actions[0])
	}
	current := server.documents.items[uri]
	var sourceEdits []edit.TextEdit
	for _, item := range change.Edits {
		start, err := current.index.Offset(item.Range.Start)
		if err != nil {
			t.Fatal(err)
		}
		end, err := current.index.Offset(item.Range.End)
		if err != nil {
			t.Fatal(err)
		}
		sourceEdits = append(sourceEdits, edit.TextEdit{Start: start, End: end, Replacement: item.NewText})
	}
	fixed, err := edit.Apply(text, sourceEdits)
	if err != nil || fixed != "[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\fs20}word\n" {
		t.Fatalf("fixed source = %q, %v", fixed, err)
	}
	stale, err := server.codeActions(t.Context(), request("old-identity"))
	if err != nil || len(stale) != 0 {
		t.Fatalf("stale identity offered actions: %+v, %v", stale, err)
	}
	server.settingsGeneration++
	stale, err = server.codeActions(t.Context(), request(data))
	if err != nil || len(stale) != 0 {
		t.Fatalf("old settings offered actions: %+v, %v", stale, err)
	}
	server.settingsGeneration--
	if err := server.documents.Change(uri, 8, []Change{{Text: text + "\n"}}); err != nil {
		t.Fatal(err)
	}
	stale, err = server.codeActions(t.Context(), request(data))
	if err != nil || len(stale) != 0 {
		t.Fatalf("old version offered actions: %+v, %v", stale, err)
	}
}

func TestSessionAdvertisesActionsOnlyForVersionedEdits(t *testing.T) {
	for _, tc := range []struct {
		capability string
		want       bool
	}{
		{`{}`, false},
		{`{"workspace":{"workspaceEdit":{"documentChanges":true}}}`, true},
	} {
		input := framedMessages(t,
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"capabilities":`+tc.capability+`}}`,
			`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`,
			`{"jsonrpc":"2.0","method":"exit"}`)
		var server Server
		var output bytes.Buffer
		if err := server.Run(t.Context(), bytes.NewReader(input), &output); err != nil {
			t.Fatal(err)
		}
		payload, err := readFrame(bufio.NewReader(&output))
		if err != nil {
			t.Fatal(err)
		}
		var response struct {
			Result struct {
				Capabilities struct {
					CodeActionProvider bool `json:"codeActionProvider"`
				} `json:"capabilities"`
			} `json:"result"`
		}
		if err := json.Unmarshal(payload, &response); err != nil {
			t.Fatal(err)
		}
		if response.Result.Capabilities.CodeActionProvider != tc.want || server.editSupport != tc.want {
			t.Fatalf("capabilities = %s, want code actions %t", payload, tc.want)
		}
	}
}

func TestCodeActionsWithholdUnsafeAndTentativeFindings(t *testing.T) {
	const uri = "file:///unsafe.ass"
	var server Server
	server.editSupport = true
	text := "[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,x\n"
	if err := server.documents.Open(uri, 1, text); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := server.publish(t.Context(), &output, uri); err != nil {
		t.Fatal(err)
	}
	payload, err := readFrame(bufio.NewReader(&output))
	if err != nil {
		t.Fatal(err)
	}
	var notification struct {
		Params struct {
			Diagnostics []struct {
				Code  string          `json:"code"`
				Data  string          `json:"data"`
				Range sourcepos.Range `json:"range"`
			} `json:"diagnostics"`
		} `json:"params"`
	}
	if err := json.Unmarshal(payload, &notification); err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, diagnostic := range notification.Params.Diagnostics {
		if diagnostic.Code != "ASS008" {
			continue
		}
		found = true
		request, err := json.Marshal(map[string]any{
			"textDocument": map[string]any{"uri": uri},
			"range":        diagnostic.Range,
			"context":      map[string]any{"diagnostics": []any{diagnostic}},
		})
		if err != nil {
			t.Fatal(err)
		}
		actions, err := server.codeActions(t.Context(), request)
		if err != nil || len(actions) != 0 {
			t.Fatalf("unsafe finding offered actions: %+v, %v", actions, err)
		}
		if err := server.documents.Change(uri, 2, []Change{{Text: "[Events]\n"}}); err != nil {
			t.Fatal(err)
		}
		actions, err = server.codeActions(t.Context(), request)
		if err != nil || len(actions) != 0 {
			t.Fatalf("tentative document offered actions: %+v, %v", actions, err)
		}
		break
	}
	if !found {
		t.Fatal("missing unsafe header finding")
	}
}
