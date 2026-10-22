package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

func fontCommandFixture(t *testing.T) (string, string, []byte) {
	t.Helper()
	dir := t.TempDir()
	fonts := filepath.Join(dir, "fonts")
	if err := os.Mkdir(fonts, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fonts, "go.ttf"), goregular.TTF, 0o600); err != nil {
		t.Fatal(err)
	}
	source := []byte("[V4+ Styles]\nFormat: Name, Fontname, Bold, Italic\nStyle: Default,Go,0,0\n[Events]\nFormat: Style, Text\nDialogue: Default,é" + string(rune(0x10ffff)) + "{\\fnAbsent}中{\\rUnknown}X\n")
	path := filepath.Join(dir, "source.ass")
	if err := os.WriteFile(path, source, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, fonts, source
}

func TestFontsCommandReportsDomainAndStaysReadOnly(t *testing.T) {
	path, fonts, source := fontCommandFixture(t)
	for _, format := range []string{"plain", "pretty", "json"} {
		t.Run(format, func(t *testing.T) {
			var out, stderr bytes.Buffer
			args := []string{"fonts", "check", "--font-dir", fonts, "--format", format, path}
			if code := run(args, &out, &stderr); code != 0 {
				t.Fatalf("exit %d: %s", code, &stderr)
			}
			for _, id := range []string{"FONT001", "FONT002"} {
				if !strings.Contains(out.String(), id) {
					t.Fatalf("missing %s: %s", id, &out)
				}
			}
			for _, forbidden := range []string{"ASS015", "ASS016", "Fixes available", "Fix: edit", "fix_safety", "edits"} {
				if strings.Contains(out.String(), forbidden) {
					t.Fatalf("font report includes %s: %s", forbidden, &out)
				}
			}
			if format == "json" {
				var report struct {
					Domain   string                          `json:"domain"`
					Renderer struct{ Name, Revision string } `json:"renderer"`
					Findings []struct {
						ID     string
						Usages []struct {
							Span struct{ Start, End int } `json:"source_span"`
						}
						Matched []struct{ Path string } `json:"matched_faces"`
					} `json:"findings"`
					Unchecked []json.RawMessage `json:"unchecked"`
				}
				if err := json.Unmarshal(out.Bytes(), &report); err != nil {
					t.Fatal(err)
				}
				if report.Domain != "font" || report.Renderer.Name != "libass" || report.Renderer.Revision == "" || len(report.Findings) != 2 || len(report.Unchecked) != 1 {
					t.Fatalf("report = %#v", report)
				}
				if report.Findings[0].ID != "FONT002" || len(report.Findings[0].Matched) != 1 {
					t.Fatalf("face selection = %#v", report.Findings)
				}
				span := report.Findings[1].Usages[0].Span
				if string(source[span.Start:span.End]) != "中" {
					t.Fatalf("span = %#v", span)
				}
			}
			var again bytes.Buffer
			if code := run(args, &again, &stderr); code != 0 || !bytes.Equal(out.Bytes(), again.Bytes()) {
				t.Fatal("font output is not deterministic")
			}
		})
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(source, after) {
		t.Fatal("font command changed source")
	}
}

func TestFontsCommandRejectsInvalidScopesAndCatalogs(t *testing.T) {
	path, fonts, _ := fontCommandFixture(t)
	for _, flags := range [][]string{{"--fix"}, {"--unsafe-fix"}, {"--compat=xy-vsfilter"}, {"--renderer=bad"}, {"--mod-build=enabled"}, {"--format=bad"}, {"--font-dir="}, {"--font-dir", filepath.Join(fonts, "absent")}} {
		var out, stderr bytes.Buffer
		args := append([]string{"fonts", "check"}, flags...)
		if code := run(append(args, path), &out, &stderr); code != 2 || stderr.Len() == 0 || out.Len() != 0 {
			t.Fatalf("%v: exit %d, out %s, err %s", flags, code, &out, &stderr)
		}
	}
	if err := os.WriteFile(filepath.Join(fonts, "bad.ttf"), []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if code := run([]string{"fonts", "check", "--font-dir", fonts, path}, &out, &stderr); code != 2 || out.Len() != 0 {
		t.Fatalf("corrupt catalog: exit %d, out %s, err %s", code, &out, &stderr)
	}
}

func TestExplicitLintAliasPreservesOutput(t *testing.T) {
	path, _, _ := fontCommandFixture(t)
	var implicit, explicit, stderr bytes.Buffer
	a := run([]string{"--format=json", path}, &implicit, &stderr)
	b := run([]string{"lint", "--format=json", path}, &explicit, &stderr)
	if a != b || !bytes.Equal(implicit.Bytes(), explicit.Bytes()) {
		t.Fatalf("alias differs: %d, %d, %s", a, b, &stderr)
	}
}

type fontFailedWriter struct{}

func (fontFailedWriter) Write([]byte) (int, error) { return 0, errors.New("output failed") }

func TestFontsCommandEmptyCatalogAndOutputFailure(t *testing.T) {
	path, _, _ := fontCommandFixture(t)
	empty := t.TempDir()
	for _, format := range []string{"json", "pretty", "plain"} {
		args := []string{"fonts", "check", "--format", format, "--font-dir", empty, path}
		var out, stderr bytes.Buffer
		if code := run(args, &out, &stderr); code != 0 || !strings.Contains(out.String(), "FONT001") || strings.Contains(out.String(), "FONT002") {
			t.Fatalf("empty catalog: exit %d: %s, %s", code, &out, &stderr)
		}
		if code := run(args, fontFailedWriter{}, &stderr); code != 2 || !strings.Contains(stderr.String(), "output failed") {
			t.Fatalf("output failure: exit %d: %s", code, &stderr)
		}
	}
}
