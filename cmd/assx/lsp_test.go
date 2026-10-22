package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestLSPCommandWritesOnlyProtocolFrames(t *testing.T) {
	var input bytes.Buffer
	for _, message := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`,
	} {
		fmt.Fprintf(&input, "Content-Length: %d\r\n\r\n%s", len(message), message)
	}
	var stdout, stderr strings.Builder
	if code := runLSP(nil, &input, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit %d, stderr %s", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "Content-Length: ") || strings.Contains(stdout.String(), "Scanning") {
		t.Fatalf("stdout is not protocol-only: %q", stdout.String())
	}
}
