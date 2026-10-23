package lsp

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestFrameRoundTripAndSplitInput(t *testing.T) {
	var stream bytes.Buffer
	for _, payload := range []string{`{"jsonrpc":"2.0","method":"initialize","id":1}`, `{"jsonrpc":"2.0","method":"exit"}`} {
		if err := writeFrame(&stream, []byte(payload)); err != nil {
			t.Fatal(err)
		}
	}
	reader := bufio.NewReaderSize(&oneByteReader{data: stream.Bytes()}, 4096)
	for _, want := range []string{`{"jsonrpc":"2.0","method":"initialize","id":1}`, `{"jsonrpc":"2.0","method":"exit"}`} {
		got, err := readFrame(reader)
		if err != nil || string(got) != want {
			t.Fatalf("frame = %q, %v; want %q", got, err, want)
		}
	}
	if _, err := readFrame(reader); !errors.Is(err, io.EOF) {
		t.Fatalf("final read = %v, want EOF", err)
	}
}

func TestFrameRejectsMalformedOrTruncatedInput(t *testing.T) {
	for _, input := range []string{
		"Content-Length: no\r\n\r\n",
		"Content-Length: -1\r\n\r\n",
		"Content-Length: 9000000\r\n\r\n",
		"Content-Length: 4\r\nContent-Length: 4\r\n\r\nnull",
		"Content-Length: 4\r\n\r\n{}",
		"Content-Length: 4\n\nnull",
		strings.Repeat("x", 5000),
	} {
		if _, err := readFrame(bufio.NewReaderSize(strings.NewReader(input), 4096)); err == nil {
			t.Errorf("accepted malformed frame %q", input[:min(len(input), 40)])
		}
	}
}

func FuzzFrameReader(f *testing.F) {
	for _, seed := range []string{
		"Content-Length: 4\r\n\r\nnull",
		"Content-Length: -1\r\n\r\n",
		"Content-Length: 4\r\n\r\n{}",
		"broken\r\n\r\n",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > maxMessageBytes+8192 {
			t.Skip()
		}
		payload, err := readFrame(bufio.NewReader(bytes.NewReader(input)))
		if err == nil && (len(payload) == 0 || len(payload) > maxMessageBytes) {
			t.Fatalf("accepted out-of-range payload length %d", len(payload))
		}
	})
}

type oneByteReader struct{ data []byte }

func (r *oneByteReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	p[0] = r.data[0]
	r.data = r.data[1:]
	return 1, nil
}
