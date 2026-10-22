// Package fontcheck checks font requests against an explicit font catalog.
// It does not predict renderer fallback or shaping.
package fontcheck

import (
	"cmp"
	"fmt"
	"golang.org/x/image/font/sfnt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
)

// Source selects either system directories or one exclusive directory.
type Source struct {
	Directory string `json:"directory,omitempty"`
}

// CatalogIssue records an unreadable file. Such a catalog cannot prove absence.
type CatalogIssue struct {
	Path   string `json:"path"`
	Detail string `json:"detail"`
}

var catalogCacheMu sync.Mutex

type fontFace struct {
	path     string
	index    int
	families []string
	bold     bool
	italic   bool
}

// Catalog indexes discovered font families and lazily loads their glyph maps.
type Catalog struct {
	families map[string][]fontFace
	loaded   map[string][]*sfnt.Font
	mu       sync.Mutex
	Source   Source
	Issues   []CatalogIssue
}

type fontContext struct {
	family string
	bold   bool
	italic bool
}

// NewCatalog indexes fonts in fontDir, or system font directories when it is empty.
func NewCatalog(source Source) (*Catalog, error) {
	fontDir := source.Directory
	custom := fontDir != ""
	var dirs []string
	if custom {
		info, err := os.Stat(fontDir)
		if err != nil {
			return nil, fmt.Errorf("font directory %q: %w", fontDir, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("font directory %q is not a directory", fontDir)
		}
		dir, err := filepath.EvalSymlinks(fontDir)
		if err != nil {
			return nil, fmt.Errorf("font directory %q: %w", fontDir, err)
		}
		dirs = []string{dir}
	} else {
		dirs = systemFontDirs()
	}

	catalogCacheMu.Lock()
	defer catalogCacheMu.Unlock()
	checker := &Catalog{Source: source, families: make(map[string][]fontFace), loaded: make(map[string][]*sfnt.Font)}
	cache := loadFontIndexCache()
	seen := make(map[string]bool)
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				if custom {
					return walkErr
				}
				return nil
			}
			if entry.IsDir() || !isFontFile(path) {
				return nil
			}
			if seen[path] {
				return nil
			}
			seen[path] = true
			info, err := entry.Info()
			if err != nil {
				return nil
			}
			faces, ok := cache[path]
			if !ok || faces.size != info.Size() || faces.modTime != info.ModTime().UnixNano() {
				faces = cachedFontFaces{size: info.Size(), modTime: info.ModTime().UnixNano(), faces: readFontFaces(path)}
				if len(faces.faces) == 0 {
					checker.Issues = append(checker.Issues, CatalogIssue{Path: path, Detail: "No readable font faces."})
				}
				if len(faces.faces) > 0 {
					cache[path] = faces
				} else {
					delete(cache, path)
				}
			}
			for _, face := range faces.faces {
				face.path = path
				for _, family := range face.families {
					key := normalizeFontFamily(family)
					if key != "" {
						checker.families[key] = append(checker.families[key], face)
					}
				}
			}
			return nil
		})
		if err != nil && !custom && !os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("scan font directory %q: %w", dir, err)
		}
	}
	for path := range cache {
		if !seen[path] {
			delete(cache, path)
		}
	}
	saveFontIndexCache(cache)
	for key := range checker.families {
		slices.SortFunc(checker.families[key], func(a, b fontFace) int { return cmp.Or(cmp.Compare(a.path, b.path), cmp.Compare(a.index, b.index)) })
	}
	return checker, nil
}

func systemFontDirs() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		windows := os.Getenv("WINDIR")
		if windows == "" {
			windows = `C:\Windows`
		}
		dirs := []string{filepath.Join(windows, "Fonts")}
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			dirs = append(dirs, filepath.Join(local, "Microsoft", "Windows", "Fonts"))
		}
		return dirs
	case "darwin":
		return []string{"/System/Library/Fonts", "/Library/Fonts", filepath.Join(home, "Library", "Fonts")}
	default:
		dirs := []string{"/usr/share/fonts", "/usr/local/share/fonts", filepath.Join(home, ".fonts")}
		dataHome := os.Getenv("XDG_DATA_HOME")
		if dataHome == "" {
			dataHome = filepath.Join(home, ".local", "share")
		}
		return append(dirs, filepath.Join(dataHome, "fonts"))
	}
}

func isFontFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ttf", ".otf", ".ttc":
		return true
	default:
		return false
	}
}

func readFontFaces(path string) []fontFace {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	collection, err := sfnt.ParseCollection(data)
	if err != nil {
		return nil
	}
	faces := make([]fontFace, 0, collection.NumFonts())
	for i := 0; i < collection.NumFonts(); i++ {
		font, err := collection.Font(i)
		if err != nil {
			continue
		}
		var buf sfnt.Buffer
		familyNames := []string{
			fontName(font, &buf, sfnt.NameIDTypographicFamily),
			fontName(font, &buf, sfnt.NameIDFamily),
			fontName(font, &buf, sfnt.NameIDFull),
		}
		if strings.EqualFold(filepath.Ext(path), ".ttc") {
			familyNames = append(familyNames, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
		}
		families := make([]string, 0, len(familyNames))
		seen := make(map[string]bool)
		for _, name := range familyNames {
			if name != "" && !seen[name] {
				families = append(families, name)
				seen[name] = true
			}
		}
		if len(families) == 0 {
			continue
		}
		subfamily := strings.ToLower(fontName(font, &buf, sfnt.NameIDTypographicSubfamily, sfnt.NameIDSubfamily))
		faces = append(faces, fontFace{
			path: path, index: i, families: families,
			bold:   strings.Contains(subfamily, "bold"),
			italic: strings.Contains(subfamily, "italic") || strings.Contains(subfamily, "oblique"),
		})
	}
	return faces
}

func fontName(font *sfnt.Font, buf *sfnt.Buffer, ids ...sfnt.NameID) string {
	for _, id := range ids {
		if name, err := font.Name(buf, id); err == nil && strings.TrimSpace(name) != "" {
			return strings.TrimSpace(name)
		}
	}
	return ""
}

func normalizeFontFamily(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
