package main

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"assx/internal/ass"
	"assx/internal/lint"
	"golang.org/x/image/font/gofont/goregular"
)

func TestRunSafeFixAndConciseSummary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "safe.ass")
	input := "[Script Info]\nPlayResX: 640\nPlayResY: 480\nYCbCr Matrix: None\nLayoutResX: 640\nLayoutResY: 480\n[V4+ Styles]\nFormat: Name, MarginL\nStyle: Default, 12.75\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0,1,Default,,0,0,0,,{\\fs10\\fs20}text\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	code := run([]string{"--fix", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run returned %d: %s", code, stderr.String())
	}
	fixed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fixed), "Style: Default, 12\n") || !strings.Contains(string(fixed), `{\fs20}text`) {
		t.Fatalf("safe edits not applied:\n%s", fixed)
	}
	if !strings.Contains(stdout.String(), "Applied fixes: 2 safe, 0 unsafe.") || !strings.Contains(stdout.String(), "Fixes available: 0 safe, 0 unsafe, 0 not auto-fixable.") {
		t.Fatalf("summary = %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "Overwritten before") || strings.Contains(stdout.String(), "fractional part") {
		t.Fatalf("implementation detail leaked into text output: %q", stdout.String())
	}
}

func TestRunRedundantStyleOverridesSafeFix(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "redundant-font.ass"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "redundant-font.ass")
	if err := os.WriteFile(path, fixture, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := run([]string{"--fix", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("run returned %d: %s", code, stderr.String())
	}
	fixed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(fixed), `\fn`) || !strings.Contains(string(fixed), "Default,alpha beta") {
		t.Fatalf("redundant font overrides were not removed:\n%s", fixed)
	}
	if !strings.Contains(stdout.String(), "Applied fixes: 1 safe, 0 unsafe.") {
		t.Fatalf("fix summary = %q", stdout.String())
	}
}

func TestRunVSFilterModTagWarningDisablesFixes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vsfiltermod.ass")
	input := "[Script Info]\nPlayResX: 640\nPlayResY: 480\nYCbCr Matrix: None\nLayoutResX: 640\nLayoutResY: 480\n" +
		"[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default, Arial, 20\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Text\n" +
		"Dialogue: 0, 0:00:00.00, 0:00:02.00, Default,{\\fs21\\fs20\\distort(1)}x\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := run([]string{"--format", "json", "--fix", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("run returned %d: %s", code, stderr.String())
	}
	var diagnostics []lint.Diagnostic
	if err := json.Unmarshal([]byte(stdout.String()), &diagnostics); err != nil {
		t.Fatalf("invalid JSON: %v\\n%s", err, stdout.String())
	}
	foundWarning := false
	for _, diagnostic := range diagnostics {
		if diagnostic.ID == lint.IssueVSFilterModTag {
			foundWarning = true
		}
		if diagnostic.FixSafety != "" || len(diagnostic.Edits) != 0 {
			t.Errorf("dialogue with VSFilterMod tag remains auto-fixable: %#v", diagnostic)
		}
	}
	if !foundWarning || !strings.Contains(stderr.String(), "0 safe, 0 unsafe, 2 not auto-fixable") {
		t.Fatalf("warning or no-fix summary missing: %s / %s", stdout.String(), stderr.String())
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != input {
		t.Fatalf("--fix changed content using a VSFilterMod tag:\n%s", after)
	}
}

func TestRunUnsafeFixIsOptInAndPreservesExitStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unsafe.ass")
	input := "[Script Info]\nPlayResX: 1920\nPlayResY: 1080\nYCbCr Matrix: TV.601\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default, Arial, 20\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0,1,Default,,0,0,0,,text\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := run([]string{path}, &stdout, &stderr); code != 0 {
		t.Fatalf("lint run returned %d: %s", code, stderr.String())
	}
	before, _ := os.ReadFile(path)
	if string(before) != input || !strings.Contains(stdout.String(), "Fixes available: 0 safe, 2 unsafe, 0 not auto-fixable.") {
		t.Fatalf("default run changed file or summary: %q / %q", before, stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"--unsafe-fix", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("unsafe-fix run returned %d: %s", code, stderr.String())
	}
	fixed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fixed), "YCbCr Matrix: TV.709") || !strings.Contains(string(fixed), "LayoutResX: 1920") || !strings.Contains(string(fixed), "LayoutResY: 1080") {
		t.Fatalf("unsafe edits not applied:\n%s", fixed)
	}
	if !strings.Contains(stdout.String(), "Applied fixes: 0 safe, 2 unsafe.") {
		t.Fatalf("unsafe summary = %q", stdout.String())
	}
}

func TestRunJSONStaysValidAndReportsUnknownTagsAsSuggestion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unknown.ass")
	input := "[Script Info]\nYCbCr Matrix: None\nPlayResX: 640\nPlayResY: 480\nLayoutResX: 640\nLayoutResY: 480\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default, Arial, 20\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0,1,Default,,0,0,0,,{\\unknownTag}text\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := run([]string{"--format", "json", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("run returned %d: %s", code, stderr.String())
	}
	var diagnostics []map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &diagnostics); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if len(diagnostics) != 1 || diagnostics[0]["id"] != "ASS007" || diagnostics[0]["severity"] != "suggestion" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if !strings.Contains(stderr.String(), "Fixes available: 0 safe, 0 unsafe, 1 not auto-fixable.") {
		t.Fatalf("JSON summary = %q", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"--format", "plain", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("plain run returned %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "SUGGESTION[ASS007]") || !strings.Contains(stdout.String(), "1 suggestions") {
		t.Fatalf("plain suggestion output = %q", stdout.String())
	}
}

func TestRenderHumanDoesNotCountUnknownSeverityAsSuggestion(t *testing.T) {
	var stdout strings.Builder
	renderHuman(&stdout, "sample.ass", []lint.Diagnostic{{ID: "ASS999", Severity: lint.Severity("future"), Title: "Unknown severity"}}, 0, "", false)
	if !strings.Contains(stdout.String(), "Summary: 1 diagnostics (0 errors, 0 warnings, 0 suggestions).") {
		t.Fatalf("summary = %q", stdout.String())
	}
}

func TestRunPreservesUTF16EncodingWhenFixing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "utf16.ass")
	text := "[Script Info]\nYCbCr Matrix: None\nPlayResX: 640\nPlayResY: 480\nLayoutResX: 640\nLayoutResY: 480\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0,1,Default,,0,0,0,,{\\fs10\\fs20}text\n"
	words := utf16.Encode([]rune(text))
	raw := []byte{0xff, 0xfe}
	for _, word := range words {
		var pair [2]byte
		binary.LittleEndian.PutUint16(pair[:], word)
		raw = append(raw, pair[:]...)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := run([]string{"--fix", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("run returned %d: %s", code, stderr.String())
	}
	fixed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixed) < 2 || fixed[0] != 0xff || fixed[1] != 0xfe {
		t.Fatalf("UTF-16 LE BOM was not preserved: %x", fixed[:min(2, len(fixed))])
	}
	source, err := ass.DecodeSource(fixed)
	if err != nil {
		t.Fatal(err)
	}
	decoded := source.Text
	if strings.Contains(decoded, `\fs10`) || !strings.Contains(decoded, `{\fs20}text`) {
		t.Fatalf("safe fix did not preserve UTF-16 content:\n%s", decoded)
	}
}

func TestRunFontChecksAreOptionalAndNotAutoFixed(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "font-check.ass"))
	if err != nil {
		t.Fatal(err)
	}
	input := strings.ReplaceAll(string(fixture), "MISSING_GLYPH", string(rune(0x10ffff)))
	path := filepath.Join(t.TempDir(), "font-check.ass")
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	fontDir := filepath.Join(t.TempDir(), "fonts", "nested")
	if err := os.MkdirAll(fontDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fontDir, "regular.ttf"), goregular.TTF, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr strings.Builder
	if code := run([]string{"--font-dir", filepath.Dir(fontDir), path}, &stdout, &stderr); code != 0 {
		t.Fatalf("default font-check run returned %d: %s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), lint.IssueFontMissing) || strings.Contains(stdout.String(), lint.IssueMissingGlyphs) {
		t.Fatalf("font checks ran without --check-fonts: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	args := []string{"--check-fonts", "--font-dir", filepath.Dir(fontDir), "--format", "json", "--fix", path}
	if code := run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("font-check run returned %d: %s", code, stderr.String())
	}
	var diagnostics []lint.Diagnostic
	if err := json.Unmarshal([]byte(stdout.String()), &diagnostics); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	counts := map[string]int{}
	for _, diagnostic := range diagnostics {
		counts[diagnostic.ID]++
		if diagnostic.Severity != lint.Suggestion {
			t.Errorf("font finding %s severity = %q, want %q", diagnostic.ID, diagnostic.Severity, lint.Suggestion)
		}
		if diagnostic.FixSafety != "" || len(diagnostic.Edits) != 0 {
			t.Errorf("font finding %s has automatic fix metadata: %#v", diagnostic.ID, diagnostic)
		}
	}
	if counts[lint.IssueFontMissing] != 2 || counts[lint.IssueMissingGlyphs] != 1 || len(diagnostics) != 3 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if !strings.Contains(stderr.String(), "Fixes available: 0 safe, 0 unsafe, 3 not auto-fixable.") {
		t.Fatalf("fix summary = %q", stderr.String())
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != input {
		t.Fatal("font check modified the ASS file")
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"--check-fonts", "--font-dir", filepath.Dir(fontDir), "--format", "plain", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("plain font-check run returned %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "SUGGESTION[ASS016]") || !strings.Contains(stdout.String(), "does not contain these characters:") {
		t.Fatalf("font detail was not rendered by diagnostic data: %q", stdout.String())
	}
}

func TestRunErrorKeepsErrorExitStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "error.ass")
	input := "[Script Info]\nYCbCr Matrix: None\nPlayResX: 640\nPlayResY: 480\nLayoutResX: 640\nLayoutResY: 480\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0,1,Default,,0,0,0,,{\\fade(1)}text\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := run([]string{path}, &stdout, &stderr); code != 1 {
		t.Fatalf("run returned %d, want 1: %s", code, stderr.String())
	}
}
