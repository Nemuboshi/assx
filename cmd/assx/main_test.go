package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/charmbracelet/x/ansi"

	"assx/internal/ass"
	"assx/internal/lint"
	"assx/internal/report"
	"assx/internal/report/plain"
)

func TestReportColorPolicy(t *testing.T) {
	for _, tc := range []struct {
		name    string
		tty     bool
		term    string
		noColor string
		want    bool
	}{
		{name: "interactive", tty: true, term: "xterm-256color", want: true},
		{name: "redirected", term: "xterm-256color"},
		{name: "dumb terminal", tty: true, term: "dumb"},
		{name: "NO_COLOR", tty: true, term: "xterm-256color", noColor: "1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := reportColorAllowed(tc.tty, tc.term, tc.noColor); got != tc.want {
				t.Fatalf("colorAllowed = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestScanStatusFitsWindow(t *testing.T) {
	path := `C:\fansub\project\SweetSub-unpublished\Seihantai na Kimi to Boku\publish\[SweetSub] Seihantai na Kimi to Boku - 24.chs.ass`
	for _, width := range []int{20, 40, 80, 120} {
		line := scanStatus("⠙", path, width)
		if got := ansi.StringWidth(line); got > width-1 {
			t.Errorf("width %d: %q uses %d cells", width, line, got)
		}
	}
	if line := scanStatus("⠋", "short.ass", 80); line != "⠋ Scanning short.ass" {
		t.Errorf("short status = %q", line)
	}
	if got := ansi.StringWidth(scanStatus("⠋", strings.Repeat("字幕", 20), 30)); got > 29 {
		t.Errorf("wide path uses %d cells", got)
	}
}

func TestScanRedrawFallsBackWithoutANSI(t *testing.T) {
	plain := scanRedraw("Scanning x.ass", 20, false)
	if strings.Contains(plain, "\x1b") {
		t.Fatalf("plain redraw contains ANSI: %q", plain)
	}
	if got := ansi.StringWidth(strings.TrimPrefix(plain, "\r")); got != 19 {
		t.Fatalf("plain redraw pads to %d cells, want 19", got)
	}
	if got := scanRedraw("x", 20, true); got != "\r\x1b[2Kx" {
		t.Fatalf("ANSI redraw = %q", got)
	}
}

func TestTerminalTextEscapesControls(t *testing.T) {
	got := terminalText("bad\x1b[31m\x7f.ass")
	if strings.Contains(got, "\x1b") || strings.ContainsRune(got, '\x7f') {
		t.Fatalf("terminal path contains controls: %q", got)
	}
}

func TestRunSafeFixAndConciseSummary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "safe.ass")
	input := "[Script Info]\nPlayResX: 640\nPlayResY: 480\nYCbCr Matrix: None\nLayoutResX: 640\nLayoutResY: 480\n[V4+ Styles]\nFormat: Name, MarginL\nStyle: Default, 12.75\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\fs10\\fs20}text\n"
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
	if !strings.Contains(string(fixed), "Style: Default, 12.75\n") || !strings.Contains(string(fixed), `{\fs20}text`) {
		t.Fatalf("safe edits not applied:\n%s", fixed)
	}
	if !strings.Contains(stdout.String(), "Applied fixes: 1 safe, 0 unsafe.") || !strings.Contains(stdout.String(), "Fixes available: 0 safe, 0 unsafe, 3 not auto-fixable.") {
		t.Fatalf("summary = %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "Overwritten before") {
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
	if strings.Contains(string(fixed), `\fn`) || !strings.Contains(string(fixed), "Default,,0,0,0,,alpha beta") {
		t.Fatalf("redundant style overrides were not removed:\n%s", fixed)
	}
	if !strings.Contains(stdout.String(), "Applied fixes: 1 safe, 0 unsafe.") {
		t.Fatalf("fix summary = %q", stdout.String())
	}
}

func TestRunVSFilterModTagWarningDisablesFixes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vsfiltermod.ass")
	input := "[Script Info]\nPlayResX: 640\nPlayResY: 480\nYCbCr Matrix: None\nLayoutResX: 640\nLayoutResY: 480\n" +
		"[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default, Arial, 20\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: 0, 0:00:00.00, 0:00:02.00, Default,,0,0,0,,{\\fs21\\fs20\\distort(1)}x\n"
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
		if diagnostic.Condition == lint.IssueVSFilterModTag {
			foundWarning = true
		}
		if diagnostic.FixSafety != "" || len(diagnostic.Edits) != 0 {
			t.Errorf("dialogue with VSFilterMod tag remains auto-fixable: %#v", diagnostic)
		}
	}
	if !foundWarning || !strings.Contains(stderr.String(), fmt.Sprintf("0 safe, 0 unsafe, %d not auto-fixable", len(diagnostics))) {
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
	input := "[Script Info]\nPlayResX: 1920\nPlayResY: 1080\nYCbCr Matrix: TV.601\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default, Arial, 20\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,text\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := run([]string{path}, &stdout, &stderr); code != 0 {
		t.Fatalf("lint run returned %d: %s", code, stderr.String())
	}
	before, _ := os.ReadFile(path)
	if string(before) != input || !strings.Contains(stdout.String(), "Fixes available: 0 safe, 2 unsafe, 2 not auto-fixable.") {
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
	input := "[Script Info]\nYCbCr Matrix: None\nPlayResX: 640\nPlayResY: 480\nLayoutResX: 640\nLayoutResY: 480\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default, Arial, 20\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\mysteryTag}text\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := run([]string{"--format", "json", "--renderer=libass", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("run returned %d: %s", code, stderr.String())
	}
	var diagnostics []map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &diagnostics); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if len(diagnostics) != 1 || diagnostics[0]["condition"] != "unknown-tag" || diagnostics[0]["severity"] != "suggestion" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if !strings.Contains(stderr.String(), "Fixes available: 0 safe, 0 unsafe, 1 not auto-fixable.") {
		t.Fatalf("JSON summary = %q", stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"--format", "plain", "--renderer=libass", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("plain run returned %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "SUGGESTION[unknown-tag]") || !strings.Contains(stdout.String(), "1 suggestions") {
		t.Fatalf("plain suggestion output = %q", stdout.String())
	}
}

func TestRunSeverityThresholdFiltersDiagnostics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "levels.ass")
	input := "[Script Info]\nYCbCr Matrix: None\nPlayResX: 640\nPlayResY: 480\nLayoutResX: 640\nLayoutResY: 480\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default, Arial, 20\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\mysteryTag}text\nDialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,{\\fade(1)}text\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		level          string
		wantSuggestion bool
		wantError      bool
	}{
		{"suggestion", true, true},
		{"warning", false, true},
		{"error", false, true},
	} {
		t.Run(tc.level, func(t *testing.T) {
			var stdout, stderr strings.Builder
			if code := run([]string{"--format", "json", "--renderer=libass", "--severity=" + tc.level, path}, &stdout, &stderr); code != 1 {
				t.Fatalf("run returned %d, want 1: %s", code, stderr.String())
			}
			var diagnostics []map[string]any
			if err := json.Unmarshal([]byte(stdout.String()), &diagnostics); err != nil {
				t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
			}
			var suggestions, errors int
			for _, diagnostic := range diagnostics {
				switch diagnostic["severity"] {
				case "suggestion":
					suggestions++
				case "error":
					errors++
				}
			}
			if suggestions != 0 && !tc.wantSuggestion {
				t.Fatalf("severity %s kept suggestions: %#v", tc.level, diagnostics)
			}
			if tc.wantSuggestion && suggestions == 0 {
				t.Fatalf("severity %s dropped suggestions: %#v", tc.level, diagnostics)
			}
			if tc.wantError && errors == 0 {
				t.Fatalf("severity %s dropped errors: %#v", tc.level, diagnostics)
			}
		})
	}
	var stdout, stderr strings.Builder
	if code := run([]string{"--severity=hint", path}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "Unknown severity") {
		t.Fatalf("invalid severity returned %d: %s", code, stderr.String())
	}
}

func TestPlainDoesNotCountUnknownSeverityAsSuggestion(t *testing.T) {
	var stdout strings.Builder
	if err := plain.Render(&stdout, "sample.ass", []report.Diagnostic{{Condition: "ASS999", Severity: "future", Title: "Unknown severity"}}, 0, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Summary: 1 diagnostics (0 errors, 0 warnings, 0 suggestions).") {
		t.Fatalf("summary = %q", stdout.String())
	}
}

func TestRunPrettyKeepsRedirectedReportOnStdout(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "redundant-font.ass")
	var stdout, stderr strings.Builder
	_ = run([]string{"--format", "pretty", path}, &stdout, &stderr)
	if !strings.Contains(stdout.String(), "assx  ") || strings.Contains(stderr.String(), "Scanning ") {
		t.Fatalf("redirected streams mixed: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunPreservesUTF16EncodingWhenFixing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "utf16.ass")
	text := "[Script Info]\nYCbCr Matrix: None\nPlayResX: 640\nPlayResY: 480\nLayoutResX: 640\nLayoutResY: 480\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\fs10\\fs20}text\n"
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

func TestPrettyFramesDecodeUTF16Source(t *testing.T) {
	text := "[Script Info]\nYCbCr Matrix: None\nPlayResX: 640\nPlayResY: 480\nLayoutResX: 640\nLayoutResY: 480\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\pos(1,2,3)}text\n"
	path := filepath.Join(t.TempDir(), "utf16-report.ass")
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
	if code := run([]string{"--format", "pretty", path}, &stdout, &stderr); code != 1 {
		t.Fatalf("run returned %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `\pos(1,2,3)`) || !strings.Contains(stdout.String(), "^~~~~~~~~") {
		t.Fatalf("UTF-16 source frame missing or misaligned: %s", stdout.String())
	}
}

func TestRunErrorKeepsErrorExitStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "error.ass")
	input := "[Script Info]\nYCbCr Matrix: None\nPlayResX: 640\nPlayResY: 480\nLayoutResX: 640\nLayoutResY: 480\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\fade(1)}text\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := run([]string{path}, &stdout, &stderr); code != 1 {
		t.Fatalf("run returned %d, want 1: %s", code, stderr.String())
	}
}
