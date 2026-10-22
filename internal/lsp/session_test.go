package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json/v2"
	"io"
	"strings"
	"testing"
	"time"
)

func TestSessionLifecycleAndProtocolResponses(t *testing.T) {
	input := framedMessages(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"initialized","params":{}}`,
		`{"jsonrpc":"2.0","id":"unknown","method":"assx/missing","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`,
	)
	var output bytes.Buffer
	var server Server
	if err := server.Run(t.Context(), bytes.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(&output)
	for _, tc := range []struct {
		id        any
		code      float64
		hasResult bool
	}{
		{float64(1), 0, true},
		{"unknown", -32601, false},
		{float64(2), 0, true},
	} {
		payload, err := readFrame(reader)
		if err != nil {
			t.Fatal(err)
		}
		var message map[string]any
		if err := json.Unmarshal(payload, &message); err != nil {
			t.Fatal(err)
		}
		if message["jsonrpc"] != "2.0" || message["id"] != tc.id {
			t.Fatalf("response = %s", payload)
		}
		if tc.hasResult {
			if _, ok := message["result"]; !ok {
				t.Fatalf("result missing: %s", payload)
			}
		} else if errorValue, ok := message["error"].(map[string]any); !ok || errorValue["code"] != tc.code {
			t.Fatalf("wrong error: %s", payload)
		}
	}
	if _, err := readFrame(reader); err != io.EOF {
		t.Fatalf("extra output: %v", err)
	}
}

func TestSessionAppliesVersionedBufferChanges(t *testing.T) {
	uri := "file:///C:/sub%20titles/test.ass"
	input := framedMessages(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"initialized"}`,
		`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+uri+`","languageId":"ass","version":1,"text":"中😀x"}}}`,
		`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":"`+uri+`","version":2},"contentChanges":[{"range":{"start":{"line":0,"character":1},"end":{"line":0,"character":3}},"text":"日"},{"range":{"start":{"line":0,"character":2},"end":{"line":0,"character":2}},"text":"\n"}]}}`,
		`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":"`+uri+`","version":1},"contentChanges":[{"text":"stale"}]}}`,
		`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`,
	)
	var server Server
	var output bytes.Buffer
	if err := server.Run(context.Background(), bytes.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	if got, version, ok := server.documents.Snapshot(uri); !ok || got != "中日\nx" || version != 2 {
		t.Fatalf("snapshot = %q, %d, %t", got, version, ok)
	}
	if bytes.Contains(output.Bytes(), []byte("stale")) {
		t.Fatal("stale notification produced protocol output")
	}
}

func TestSessionRejectsUninitializedRequestAndAbruptEOF(t *testing.T) {
	input := framedMessages(t, `{"jsonrpc":"2.0","id":7,"method":"textDocument/hover","params":{}}`)
	var output bytes.Buffer
	var server Server
	if err := server.Run(t.Context(), bytes.NewReader(input), &output); err == nil {
		t.Fatal("abrupt EOF succeeded")
	}
	payload, err := readFrame(bufio.NewReader(&output))
	if err != nil || !strings.Contains(string(payload), `"code":-32002`) {
		t.Fatalf("pre-initialize response = %s, %v", payload, err)
	}
}

func TestSessionRejectsMissingChangeFieldsWithoutMutation(t *testing.T) {
	uri := "file:///test.ass"
	for _, change := range []string{
		`{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}`,
		`{"range":{},"text":"insert"}`,
	} {
		t.Run(change, func(t *testing.T) {
			input := framedMessages(t,
				`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
				`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+uri+`","version":1,"text":"abc"}}}`,
				`{"jsonrpc":"2.0","method":"textDocument/didChange","params":{"textDocument":{"uri":"`+uri+`","version":2},"contentChanges":[`+change+`]}}`,
				`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`,
				`{"jsonrpc":"2.0","method":"exit"}`)
			var server Server
			if err := server.Run(t.Context(), bytes.NewReader(input), io.Discard); err != nil {
				t.Fatal(err)
			}
			if got, version, _ := server.documents.Snapshot(uri); got != "abc" || version != 1 {
				t.Fatalf("malformed change committed %q at version %d", got, version)
			}
		})
	}
}

func TestSessionRejectsMissingOpenText(t *testing.T) {
	input := framedMessages(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"file:///test.ass","version":1}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`)
	var server Server
	if err := server.Run(t.Context(), bytes.NewReader(input), io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := server.documents.Snapshot("file:///test.ass"); ok {
		t.Fatal("opened document without required text")
	}
}

func TestSessionDistinguishesInvalidJSONFromInvalidRequest(t *testing.T) {
	input := framedMessages(t, `[]`, `{`,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`)
	var server Server
	var output bytes.Buffer
	if err := server.Run(t.Context(), bytes.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(&output)
	for _, code := range []string{`"code":-32600`, `"code":-32700`} {
		payload, err := readFrame(reader)
		if err != nil || !bytes.Contains(payload, []byte(code)) {
			t.Fatalf("want %s; got %s, %v", code, payload, err)
		}
	}
}

func TestSessionCancellationInterruptsBlockedInput(t *testing.T) {
	input, output := io.Pipe()
	defer func() { _ = output.Close() }()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		var server Server
		done <- server.Run(ctx, input, io.Discard)
	}()
	if _, err := io.WriteString(output, "Content-Length: "); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("Run returned %v, want cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not interrupt blocked input")
	}
}

func TestSessionSaveKeepsBufferAndCloseRemovesIt(t *testing.T) {
	uri := "file:///test.ass"
	input := framedMessages(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+uri+`","version":1,"text":"unsaved"}}}`,
		`{"jsonrpc":"2.0","method":"textDocument/didSave","params":{"textDocument":{"uri":"`+uri+`"}}}`,
		`{"jsonrpc":"2.0","method":"textDocument/didClose","params":{"textDocument":{"uri":"`+uri+`"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`)
	var server Server
	if err := server.Run(t.Context(), bytes.NewReader(input), io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := server.documents.Snapshot(uri); ok {
		t.Fatal("close retained buffer")
	}
}

func framedMessages(t *testing.T, messages ...string) []byte {
	t.Helper()
	var input bytes.Buffer
	for _, message := range messages {
		if err := writeFrame(&input, []byte(message)); err != nil {
			t.Fatal(err)
		}
	}
	return input.Bytes()
}
