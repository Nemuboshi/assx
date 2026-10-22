package main

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestInputRecognitionProtectsFiles(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		code          int
	}{
		{"plain text", "A note about a project.\n", 2},
		{"tentative", "[Events]\n", 0},
		{"comment-only INI", "[Events]\nComment: project notes\n", 0},
	} {
		for _, flag := range []string{"--fix", "--unsafe-fix"} {
			t.Run(tc.name+flag, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "misleading.ass")
				if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
					t.Fatal(err)
				}
				var stdout, stderr strings.Builder
				if code := run([]string{"--format=json", flag, path}, &stdout, &stderr); code != tc.code {
					t.Fatalf("exit = %d, want %d; stderr = %s", code, tc.code, stderr.String())
				}
				got, err := os.ReadFile(path)
				if err != nil || string(got) != tc.content {
					t.Fatalf("input changed: %q, %v", got, err)
				}
				if !strings.Contains(stderr.String(), "ASS/SSA") {
					t.Fatalf("missing input-level notice: %s", stderr.String())
				}
				if tc.code == 2 && stdout.Len() != 0 {
					t.Fatalf("rejected input produced lint output: %s", stdout.String())
				}
				if tc.code == 0 && !json.Valid([]byte(stdout.String())) {
					t.Fatalf("tentative input has invalid JSON: %s", stdout.String())
				}
			})
		}
	}
}

func TestForceASSOverridesRecognitionOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("A note.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := run([]string{"--force-ass", "--format=json", "--unsafe-fix", path}, &stdout, &stderr); code == 2 {
		t.Fatalf("explicit override rejected: %s", stderr.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "[Script Info]") {
		t.Fatalf("explicit unsafe override did not permit edits: %q", got)
	}
}

func TestRecognitionUsesContentNotExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subtitle.txt")
	if err := os.WriteFile(path, []byte("[Events]\nDialogue: broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := run([]string{"--format=json", path}, &stdout, &stderr); code == 2 {
		t.Fatalf("recognized content rejected by suffix: %s", stderr.String())
	}
	if !json.Valid([]byte(stdout.String())) {
		t.Fatalf("invalid JSON: %s", stdout.String())
	}
}

func TestForceASSDoesNotBypassEncodingGuard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.ass")
	words := utf16.Encode([]rune("A note.\n"))
	words = append(words, 0xd800)
	input := []byte{0xff, 0xfe}
	for _, word := range words {
		input = binary.LittleEndian.AppendUint16(input, word)
	}
	if err := os.WriteFile(path, input, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := run([]string{"--force-ass", "--unsafe-fix", path}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "losslessly round-tripped") {
		t.Fatalf("encoding guard: exit %d, stderr %s", code, stderr.String())
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(input) {
		t.Fatalf("source changed: %v", err)
	}
}

func TestLegacyBytesDoNotBecomeWritable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.ass")
	input := []byte("[Events]\nDialogue: ")
	input = append(input, 0xff, '\n')
	if err := os.WriteFile(path, input, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := run([]string{"--unsafe-fix", path}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "not valid UTF-8") {
		t.Fatalf("legacy encoding result: exit %d, stderr %s", code, stderr.String())
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(input) {
		t.Fatalf("legacy input changed: %v", err)
	}
}

func TestRecognitionPreservesBOMInputs(t *testing.T) {
	text := "[Events]\nDialogue: broken\n"
	utf16LE := []byte{0xff, 0xfe}
	for _, word := range utf16.Encode([]rune(text)) {
		utf16LE = binary.LittleEndian.AppendUint16(utf16LE, word)
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"UTF-8 BOM", append([]byte{0xef, 0xbb, 0xbf}, text...)},
		{"UTF-16 LE", utf16LE},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "subtitle.txt")
			if err := os.WriteFile(path, tc.data, 0o600); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr strings.Builder
			if code := run([]string{"--format=json", path}, &stdout, &stderr); code == 2 || !json.Valid([]byte(stdout.String())) {
				t.Fatalf("recognized BOM input: exit %d, stderr %s, stdout %s", code, stderr.String(), stdout.String())
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != string(tc.data) {
				t.Fatalf("BOM input changed: %v", err)
			}
		})
	}
}

func TestForceASSCanWriteTentativeInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial.ass")
	if err := os.WriteFile(path, []byte("[Events]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := run([]string{"--force-ass", "--unsafe-fix", path}, &stdout, &stderr); code == 2 {
		t.Fatalf("explicit override rejected: %s", stderr.String())
	}
	got, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(got), "[Script Info]") {
		t.Fatalf("tentative input was not changed by explicit override: %q, %v", got, err)
	}
}

func TestFontsRejectsForeignInputBeforeCatalog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.ass")
	if err := os.WriteFile(path, []byte("A note.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := run([]string{"fonts", "check", "--font-dir", filepath.Join(t.TempDir(), "missing"), path}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "ASS/SSA") {
		t.Fatalf("foreign input reached catalog: exit %d, stderr %s", code, stderr.String())
	}
}
