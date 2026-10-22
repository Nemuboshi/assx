//go:build !windows

package fontcheck

import (
	"os"
	"path/filepath"
	"testing"

	"assx/internal/ass/renderer"
	"golang.org/x/image/font/gofont/goregular"
)

func TestWarmCatalogDetectsDeniedRead(t *testing.T) {
	isolateCache(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "font.ttf")
	if err := os.WriteFile(path, goregular.TTF, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewCatalog(Source{Directory: dir}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(path, 0o600); err != nil {
			t.Error(err)
		}
	})
	if file, err := os.Open(path); err == nil {
		_ = file.Close()
		t.Skip("process can read files without permission")
	}
	catalog, err := NewCatalog(Source{Directory: dir})
	if err != nil {
		return
	}
	findings, err := Analyze(usageDocument(`{\fnAbsent}A`), catalog, renderer.Profile{})
	if err == nil || len(findings) != 0 {
		t.Fatalf("unreadable cached font allowed absence claim: %#v, %v", findings, err)
	}
}
