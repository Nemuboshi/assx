package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failingOutput struct {
	remaining int
	err       error
}

func (w *failingOutput) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		n := w.remaining
		w.remaining = 0
		return n, w.err
	}
	w.remaining -= len(p)
	return len(p), nil
}

func TestRunReportsOutputFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.ass")
	input := "[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" + strings.Repeat("Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\fsbad}Text\n", 100)
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"plain", "pretty", "json"} {
		for _, limit := range []int{0, 4096} {
			t.Run(fmt.Sprintf("%s/after_%d_bytes", format, limit), func(t *testing.T) {
				failure := errors.New("output unavailable")
				writer := &failingOutput{remaining: limit, err: failure}
				var stderr strings.Builder
				if code := run([]string{"--format", format, path}, writer, &stderr); code != 2 {
					t.Fatalf("got exit %d, want output error: %s", code, stderr.String())
				}
				if !strings.Contains(stderr.String(), failure.Error()) {
					t.Fatalf("lost output failure: %s", stderr.String())
				}
			})
		}
	}
}

func TestWriteAtomicallyCleansUpAfterRenameFailure(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "existing")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(target, "keep.txt")
	if err := os.WriteFile(marker, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := writeAtomically(target, []byte("replacement"), 0o600)
	if err == nil {
		t.Fatal("expected rename to fail over an existing directory")
	}
	var renameError *os.LinkError
	if !errors.As(err, &renameError) {
		t.Fatalf("lost primary rename error: %v", err)
	}
	data, readErr := os.ReadFile(marker)
	if readErr != nil || string(data) != "original" {
		t.Fatalf("failed replacement changed destination: %q %v", data, readErr)
	}
	temporary, globErr := filepath.Glob(filepath.Join(directory, ".assx-*.tmp"))
	if globErr != nil || len(temporary) != 0 {
		t.Fatalf("temporary files left after failure: %v %v", temporary, globErr)
	}
}
