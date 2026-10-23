package lsp

import (
	"bufio"
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"io"
	"strings"
	"testing"

	"assx/internal/lint"
)

func TestSessionPublishesCurrentVersionAndClearsRejectedInput(t *testing.T) {
	uri := "file:///test.ass"
	input := framedMessages(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+uri+`","version":1,"text":"[Events]\nDialogue: broken\n"}}}`,
		`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":"`+uri+`","version":2},"contentChanges":[{"text":"WEBVTT\n"}]}}`,
		`{"jsonrpc":"2.0","method":"textDocument/didClose","params":{"textDocument":{"uri":"`+uri+`"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`)
	var server Server
	var output bytes.Buffer
	if err := server.Run(t.Context(), bytes.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(&output)
	var publishes []struct {
		URI         string `json:"uri"`
		Version     *int   `json:"version"`
		Diagnostics []struct {
			Code   string `json:"code"`
			Source string `json:"source"`
		} `json:"diagnostics"`
	}
	for {
		payload, err := readFrame(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		var envelope struct {
			Method string         `json:"method"`
			Params jsontext.Value `json:"params"`
		}
		if err := json.Unmarshal(payload, &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Method == "textDocument/publishDiagnostics" {
			var params struct {
				URI         string `json:"uri"`
				Version     *int   `json:"version"`
				Diagnostics []struct {
					Code   string `json:"code"`
					Source string `json:"source"`
				} `json:"diagnostics"`
			}
			if err := json.Unmarshal(envelope.Params, &params); err != nil {
				t.Fatal(err)
			}
			publishes = append(publishes, params)
		}
	}
	if len(publishes) != 3 {
		t.Fatalf("got %d diagnostic publishes, want open/change/close", len(publishes))
	}
	if publishes[0].URI != uri || publishes[0].Version == nil || *publishes[0].Version != 1 || len(publishes[0].Diagnostics) == 0 {
		t.Fatalf("open diagnostics = %+v", publishes[0])
	}
	for _, diagnostic := range publishes[0].Diagnostics {
		if diagnostic.Code == "" || diagnostic.Source != "assx" {
			t.Fatalf("unidentified diagnostic = %+v", diagnostic)
		}
	}
	if publishes[1].Version == nil || *publishes[1].Version != 2 || len(publishes[1].Diagnostics) != 0 {
		t.Fatalf("rejected content diagnostics = %+v", publishes[1])
	}
	if len(publishes[2].Diagnostics) != 0 {
		t.Fatalf("close diagnostics = %+v", publishes[2])
	}
}

func TestPublishedDiagnosticUsesExactUTF16Range(t *testing.T) {
	text := "[Events]\r\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\r\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,😀{\\unknown}x\r\n"
	var server Server
	if err := server.documents.Open("file:///unicode.ass", 4, text); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := server.publish(t.Context(), &output, "file:///unicode.ass"); err != nil {
		t.Fatal(err)
	}
	payload, err := readFrame(bufio.NewReader(&output))
	if err != nil {
		t.Fatal(err)
	}
	var message struct {
		Params struct {
			Diagnostics []struct {
				Code  string `json:"code"`
				Range struct {
					Start struct {
						Line      int `json:"line"`
						Character int `json:"character"`
					} `json:"start"`
					End struct {
						Line      int `json:"line"`
						Character int `json:"character"`
					} `json:"end"`
				} `json:"range"`
			} `json:"diagnostics"`
		} `json:"params"`
	}
	if err := json.Unmarshal(payload, &message); err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range message.Params.Diagnostics {
		if diagnostic.Code == "ASS007" {
			if diagnostic.Range.Start.Line != 2 || diagnostic.Range.Start.Character != 53 || diagnostic.Range.End.Character != 61 {
				t.Fatalf("unknown tag range = %+v", diagnostic.Range)
			}
			return
		}
	}
	t.Fatal("missing unknown tag diagnostic")
}

func TestForcedAnalysisDoesNotTreatForeignTextAsASS(t *testing.T) {
	var server Server
	server.settings.ForceASS = true
	for _, tc := range []struct {
		uri  string
		text string
		want bool
	}{
		{"file:///empty.ass", "", true},
		{"file:///foreign.ass", "WEBVTT\n00:01.000 --> 00:02.000\n", false},
	} {
		if err := server.documents.Open(tc.uri, 1, tc.text); err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		if err := server.publish(t.Context(), &output, tc.uri); err != nil {
			t.Fatal(err)
		}
		payload, err := readFrame(bufio.NewReader(&output))
		if err != nil {
			t.Fatal(err)
		}
		var message struct {
			Params struct {
				Diagnostics []struct{} `json:"diagnostics"`
			} `json:"params"`
		}
		if err := json.Unmarshal(payload, &message); err != nil {
			t.Fatal(err)
		}
		if got := len(message.Params.Diagnostics) > 0; got != tc.want {
			t.Errorf("%s diagnostic presence = %t, want %t", tc.uri, got, tc.want)
		}
	}
}

func TestPublishedDiagnosticsHonorRulePolicy(t *testing.T) {
	var server Server
	server.settings.Options.Rules = lint.RulePolicy{{ID: lint.IssueUnknownTag, Disabled: true}}
	text := "[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\unknown}x\n"
	uri := "file:///test.ass"
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
	if bytes.Contains(payload, []byte(`"code":"ASS007"`)) {
		t.Fatalf("disabled rule was published: %s", payload)
	}
}

func TestPublishedDiagnosticsRetainRendererContext(t *testing.T) {
	var server Server
	text := "[Events]\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\blur101}x\n"
	uri := "file:///test.ass"
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
	var message struct {
		Params struct {
			Diagnostics []struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"diagnostics"`
		} `json:"params"`
	}
	if err := json.Unmarshal(payload, &message); err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range message.Params.Diagnostics {
		if diagnostic.Code == "ASS031" && strings.Contains(diagnostic.Message, "libass@") && strings.Contains(diagnostic.Message, "xy-VSFilter@") {
			return
		}
	}
	t.Fatalf("missing renderer context in diagnostics: %+v", message.Params.Diagnostics)
}

func TestPublishedDiagnosticsUseSelectedPrimaryRenderer(t *testing.T) {
	text := "[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\pos(1,2,3)}x\n"
	const uri = "file:///profile.ass"
	var server Server
	if err := server.documents.Open(uri, 1, text); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		config string
		want   bool
	}{
		{`{"assx":{"primaryRenderer":"libass","compatibilityRenderers":[]}}`, true},
		{`{"assx":{"primaryRenderer":"vsfiltermod","compatibilityRenderers":[],"vsfilterMod":{"mod":"enabled","lua":"disabled"}}}`, false},
	} {
		configured, err := parseSettings(jsontext.Value(tc.config))
		if err != nil {
			t.Fatal(err)
		}
		server.settings = configured
		var output bytes.Buffer
		if err := server.publish(t.Context(), &output, uri); err != nil {
			t.Fatal(err)
		}
		payload, err := readFrame(bufio.NewReader(&output))
		if err != nil {
			t.Fatal(err)
		}
		got := bytes.Contains(payload, []byte(`"code":"ASS001"`))
		if got != tc.want {
			t.Errorf("ASS001 present = %t, want %t: %s", got, tc.want, payload)
		}
	}
}
