package fontcheck

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/image/font/gofont/goregular"
)

func isolateCache(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("HOME", dir)
}

func TestCatalogInvalidationAndCorruptFiles(t *testing.T) {
	isolateCache(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "font.ttf")
	if err := os.WriteFile(path, goregular.TTF, 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := NewCatalog(Source{Directory: dir})
	if err != nil || len(first.families["go"]) == 0 {
		t.Fatalf("first scan: %#v, %v", first, err)
	}
	if err := os.WriteFile(path, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := NewCatalog(Source{Directory: dir})
	if err != nil || len(second.families["go"]) != 0 || len(second.Issues) != 1 {
		t.Fatalf("changed file: %#v, %v", second, err)
	}
	if err := os.WriteFile(path, goregular.TTF, 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(time.Second)
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatal(err)
	}
	third, err := NewCatalog(Source{Directory: dir})
	if err != nil || len(third.families["go"]) == 0 || len(third.Issues) != 0 {
		t.Fatalf("repaired file: %#v, %v", third, err)
	}
}

func TestCatalogCollectionAndConcurrentCoverage(t *testing.T) {
	isolateCache(t)
	dir := t.TempDir()
	// TTC tables use file offsets, not offsets from each face header.
	data := make([]byte, 20)
	copy(data, "ttcf")
	binary.BigEndian.PutUint32(data[4:], 0x10000)
	binary.BigEndian.PutUint32(data[8:], 2)
	for i := range 2 {
		offset := len(data)
		binary.BigEndian.PutUint32(data[12+i*4:], uint32(offset))
		face := append([]byte(nil), goregular.TTF...)
		for j := range int(binary.BigEndian.Uint16(face[4:])) {
			at := 12 + j*16 + 8
			binary.BigEndian.PutUint32(face[at:], binary.BigEndian.Uint32(face[at:])+uint32(offset))
		}
		data = append(data, face...)
	}
	if err := os.WriteFile(filepath.Join(dir, "collection.ttc"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	catalog, err := NewCatalog(Source{Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.selectedFaces(fontContext{family: "Go"})) != 2 {
		t.Fatalf("faces = %#v", catalog.families)
	}
	for i := range 4 {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			t.Parallel()
			missing, found, err := catalog.missingRunes(fontContext{family: "Go"}, "A"+string(rune(0x10ffff)))
			if err != nil || !found || string(missing) != string(rune(0x10ffff)) {
				t.Fatalf("coverage = %q, %v, %v", string(missing), found, err)
			}
		})
	}
}

func BenchmarkCatalogScan(b *testing.B) {
	dir := b.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "font.ttf"), goregular.TTF, 0o600); err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := NewCatalog(Source{Directory: dir}); err != nil {
			b.Fatal(err)
		}
	}
}
