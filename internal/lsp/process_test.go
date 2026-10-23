package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRealProcessProtocolAndCleanStdout(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the assx command")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "assx")
	if filepath.Separator == '\\' {
		path += ".exe"
	}
	build := exec.CommandContext(ctx, "go", "build", "-o", path, "./cmd/assx")
	build.Dir = filepath.Join("..", "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, output)
	}
	const uri = "file:///process.ass"
	input := framedMessages(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"capabilities":{"workspace":{"workspaceEdit":{"documentChanges":true}}}}}`,
		`{"jsonrpc":"2.0","method":"initialized","params":{}}`,
		`{"jsonrpc":"2.0","method":"textDocument/didOpen","params":{"textDocument":{"uri":"`+uri+`","version":1,"text":"[Events]\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\pos(1,2)}x\n"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"textDocument/hover","params":{"textDocument":{"uri":"`+uri+`"},"position":{"line":1,"character":55}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`)
	command := exec.CommandContext(ctx, path, "lsp")
	command.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("server: %v: %s", err, stderr.String())
	}
	reader := bufio.NewReader(&stdout)
	seen := make(map[string]bool)
	for range 4 {
		payload, err := readFrame(reader)
		if err != nil {
			t.Fatalf("invalid stdout frame: %v; %q", err, stdout.String())
		}
		var message struct {
			ID     jsontext.Value `json:"id"`
			Method string         `json:"method"`
		}
		if err := json.Unmarshal(payload, &message); err != nil {
			t.Fatal(err)
		}
		if message.Method != "" {
			seen[message.Method] = true
		} else {
			seen[string(message.ID)] = true
		}
	}
	if !seen["1"] || !seen["2"] || !seen["3"] || !seen["textDocument/publishDiagnostics"] || reader.Buffered() != 0 {
		t.Fatalf("incomplete or extra output: %+v, trailing %d", seen, reader.Buffered())
	}
}
