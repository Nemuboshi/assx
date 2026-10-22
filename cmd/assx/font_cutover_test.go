package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"assx/internal/lint"
)

func TestFontCutoverRejectsLegacyFlags(t *testing.T) {
	path, _, source := fontCommandFixture(t)
	for _, prefix := range [][]string{nil, {"lint"}} {
		for _, flag := range []string{"--check-fonts", "--check-fonts=false", "--font-dir=missing"} {
			var out, stderr bytes.Buffer
			args := append(append([]string{}, prefix...), flag, path)
			if code := run(args, &out, &stderr); code != 2 || out.Len() != 0 || !strings.Contains(stderr.String(), "assx fonts check") {
				t.Fatalf("%v: exit %d, out %s, err %s", args, code, &out, &stderr)
			}
		}
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(source, after) {
		t.Fatal("rejected flags changed source")
	}
}

func TestFontCutoverLintDoesNotAccessCatalog(t *testing.T) {
	for _, id := range []string{"ASS015", "ASS016", "FONT001", "FONT002"} {
		if _, exists := lint.Rules[id]; exists {
			t.Fatalf("environmental rule remains active: %s", id)
		}
	}
	cache := t.TempDir()
	t.Setenv("LOCALAPPDATA", cache)
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("HOME", cache)
	t.Setenv("WINDIR", filepath.Join(cache, "unavailable"))
	for _, format := range []string{"plain", "pretty", "json"} {
		for _, options := range [][]string{nil, {"--fix"}, {"--unsafe-fix"}, {"--renderer=xy-vsfilter"}, {"--renderer=vsfiltermod", "--mod-build=enabled", "--lua-build=disabled"}, {"--compat=libass,xy-vsfilter"}} {
			path, _, _ := fontCommandFixture(t)
			var out, stderr bytes.Buffer
			args := append([]string{"lint", "--format", format}, options...)
			if code := run(append(args, path), &out, &stderr); code == 2 {
				t.Fatalf("lint failed: %v: %s", args, &stderr)
			}
			for _, id := range []string{"ASS015", "ASS016", "FONT001", "FONT002"} {
				if strings.Contains(out.String(), id) || strings.Contains(stderr.String(), id) {
					t.Fatalf("font finding in lint: %s", &out)
				}
			}
		}
	}
	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) != 0 {
		t.Fatalf("lint wrote font/cache data: %v, %v", entries, err)
	}
}
