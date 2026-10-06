package lint

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type cachedFontFaces struct {
	size    int64
	modTime int64
	faces   []fontFace
}

type fontIndexEntry struct {
	Path    string          `json:"path"`
	Size    int64           `json:"size"`
	ModTime int64           `json:"mod_time"`
	Faces   []fontIndexFace `json:"faces"`
}

type fontIndexFace struct {
	Index    int      `json:"index"`
	Families []string `json:"families"`
	Bold     bool     `json:"bold"`
	Italic   bool     `json:"italic"`
}

func fontIndexCachePath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(cacheDir, "assx", "font-index-v1.json")
}

func loadFontIndexCache() map[string]cachedFontFaces {
	cache := make(map[string]cachedFontFaces)
	path := fontIndexCachePath()
	if path == "" {
		return cache
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cache
	}
	var entries []fontIndexEntry
	if json.Unmarshal(data, &entries) != nil {
		return cache
	}
	for _, entry := range entries {
		faces := make([]fontFace, 0, len(entry.Faces))
		for _, face := range entry.Faces {
			faces = append(faces, fontFace{path: entry.Path, index: face.Index, families: face.Families, bold: face.Bold, italic: face.Italic})
		}
		cache[entry.Path] = cachedFontFaces{size: entry.Size, modTime: entry.ModTime, faces: faces}
	}
	return cache
}

func saveFontIndexCache(cache map[string]cachedFontFaces) {
	path := fontIndexCachePath()
	if path == "" {
		return
	}
	entries := make([]fontIndexEntry, 0, len(cache))
	for name, cached := range cache {
		entry := fontIndexEntry{Path: name, Size: cached.size, ModTime: cached.modTime}
		for _, face := range cached.faces {
			entry.Faces = append(entry.Faces, fontIndexFace{Index: face.index, Families: face.families, Bold: face.bold, Italic: face.italic})
		}
		entries = append(entries, entry)
	}
	data, err := json.Marshal(entries)
	if err != nil || os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}
