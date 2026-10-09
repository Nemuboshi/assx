package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"assx/internal/lint"
)

func TestRunFixPreservesSymlinksAndUpdatesTheirTarget(t *testing.T) {
	input, err := os.ReadFile("../../testdata/redundant-font.ass")
	if err != nil {
		t.Fatal(err)
	}
	for _, option := range []string{"--fix", "--unsafe-fix"} {
		t.Run(option, func(t *testing.T) {
			dir := t.TempDir()
			targetDir := filepath.Join(dir, "target")
			if err := os.Mkdir(targetDir, 0o700); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(targetDir, "original.ass")
			if err := os.WriteFile(target, input, 0o640); err != nil {
				t.Fatal(err)
			}
			for _, link := range [][2]string{{"first.ass", filepath.Join("target", "original.ass")}, {"second.ass", "first.ass"}} {
				if err := os.Symlink(link[1], filepath.Join(dir, link[0])); err != nil {
					if runtime.GOOS == "windows" {
						t.Skipf("Windows symlink permission unavailable: %v", err)
					}
					t.Fatal(err)
				}
			}
			var stdout, stderr strings.Builder
			link := filepath.Join(dir, "second.ass")
			if code := run([]string{option, "--format", "plain", link}, &stdout, &stderr); code != 0 {
				t.Fatalf("run = %d: %s", code, stderr.String())
			}
			for _, name := range []string{"first.ass", "second.ass"} {
				info, err := os.Lstat(filepath.Join(dir, name))
				if err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("%s was replaced: %v", name, err)
				}
			}
			fixed, err := os.ReadFile(target)
			if err != nil || bytes.Equal(fixed, input) || bytes.Contains(fixed, []byte(`\fn`)) {
				t.Fatalf("target was not fixed: %q / %v", fixed, err)
			}
			throughLink, err := os.ReadFile(link)
			if err != nil || !bytes.Equal(fixed, throughLink) {
				t.Fatalf("link no longer reads the target: %v", err)
			}
			if !strings.Contains(stdout.String(), link) {
				t.Fatalf("output lost the user-supplied path: %s", stdout.String())
			}
			if runtime.GOOS != "windows" {
				info, err := os.Stat(target)
				if err != nil || info.Mode().Perm() != 0o640 {
					t.Fatalf("target permissions changed: %v", err)
				}
			}
		})
	}
}

func TestRunRedundantStyleDiagnosticHasPlainAndJSONMetadata(t *testing.T) {
	path := "../../testdata/redundant-font.ass"
	for _, format := range []string{"plain", "json"} {
		var stdout, stderr strings.Builder
		if code := run([]string{"--format", format, path}, &stdout, &stderr); code != 0 {
			t.Fatalf("run = %d: %s", code, stderr.String())
		}
		if format == "plain" {
			if !strings.Contains(stdout.String(), "Overrides match style") {
				t.Fatalf("missing diagnostic title: %s", stdout.String())
			}
			continue
		}
		var diagnostics []lint.Diagnostic
		if err := json.Unmarshal([]byte(stdout.String()), &diagnostics); err != nil {
			t.Fatal(err)
		}
		if len(diagnostics) != 1 || diagnostics[0].Title == "" || diagnostics[0].Description == "" || diagnostics[0].Fix == "" {
			t.Fatalf("incomplete JSON diagnostic: %#v", diagnostics)
		}
	}
}
