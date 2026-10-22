//go:build windows

package fontcheck

import (
	"os"
	"path/filepath"
	"testing"

	"assx/internal/ass/renderer"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/sys/windows"
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
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := windows.CloseHandle(handle); err != nil {
			t.Error(err)
		}
	})
	if file, err := os.Open(path); err == nil {
		_ = file.Close()
		t.Fatal("fixture did not deny reads")
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
