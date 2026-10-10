package lint

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"assx/internal/ass"
	"assx/internal/semantic"
	"golang.org/x/image/font/sfnt"
)

type fontFace struct {
	path     string
	index    int
	families []string
	bold     bool
	italic   bool
}

// FontChecker indexes discovered font families and lazily loads their glyph maps.
type FontChecker struct {
	families map[string][]fontFace
	loaded   map[string][]*sfnt.Font
}

type fontContext struct {
	family string
	bold   bool
	italic bool
}

type fontUsage struct {
	context fontContext
	column  int
	text    strings.Builder
}

// NewFontChecker indexes fonts in fontDir, or system font directories when it is empty.
func NewFontChecker(fontDir string) (*FontChecker, error) {
	custom := strings.TrimSpace(fontDir) != ""
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

	checker := &FontChecker{families: make(map[string][]fontFace), loaded: make(map[string][]*sfnt.Font)}
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
		sort.Slice(checker.families[key], func(i, j int) bool {
			a, b := checker.families[key][i], checker.families[key][j]
			if a.path == b.path {
				return a.index < b.index
			}
			return a.path < b.path
		})
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

func (c *FontChecker) missingRunes(context fontContext, text string) ([]rune, bool, error) {
	candidates := c.families[normalizeFontFamily(context.family)]
	if len(candidates) == 0 {
		return nil, false, nil
	}
	best := candidates[0]
	bestScore := styleDistance(best, context)
	for _, face := range candidates[1:] {
		if score := styleDistance(face, context); score < bestScore {
			best, bestScore = face, score
		}
	}
	fonts, ok := c.loaded[best.path]
	if !ok {
		data, err := os.ReadFile(best.path)
		if err != nil {
			return nil, true, fmt.Errorf("read font %q: %w", best.path, err)
		}
		collection, err := sfnt.ParseCollection(data)
		if err != nil {
			return nil, true, fmt.Errorf("parse font %q: %w", best.path, err)
		}
		fonts = make([]*sfnt.Font, collection.NumFonts())
		for i := range fonts {
			fonts[i], _ = collection.Font(i)
		}
		c.loaded[best.path] = fonts
	}
	if best.index >= len(fonts) || fonts[best.index] == nil {
		return nil, true, fmt.Errorf("font %q has no readable face %d", best.path, best.index)
	}

	selected := []*sfnt.Font{fonts[best.index]}
	if len(fonts) > 1 {
		seenFaces := map[int]bool{best.index: true}
		for _, face := range candidates {
			if face.path != best.path || styleDistance(face, context) != bestScore || seenFaces[face.index] || face.index >= len(fonts) || fonts[face.index] == nil {
				continue
			}
			selected = append(selected, fonts[face.index])
			seenFaces[face.index] = true
		}
	}
	seen := make(map[rune]bool)
	missing := make([]rune, 0)
	var buf sfnt.Buffer
	for _, r := range text {
		if r == '\n' || r == '\r' || seen[r] {
			continue
		}
		seen[r] = true
		found := false
		for _, font := range selected {
			glyph, err := font.GlyphIndex(&buf, r)
			if err == nil && glyph != 0 {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, r)
		}
	}
	return missing, true, nil
}

func styleDistance(face fontFace, context fontContext) int {
	score := 0
	if face.bold != context.bold {
		score++
	}
	if face.italic != context.italic {
		score++
	}
	return score
}

// AnalyzeFonts reports missing font families and missing glyphs in dialogue text.
func AnalyzeFonts(doc ass.Document, checker *FontChecker) ([]Diagnostic, error) {
	definitions := semantic.StyleDefinitionsByName(doc.StyleFields)
	styles := semantic.StyleStatesByName(doc.StyleFields)
	// ASS defaults for missing bold/italic fields are known to the font
	// checker even when the source omitted the corresponding Style columns.
	for name, fields := range definitions {
		style := styles[name]
		for _, property := range []string{"bold", "italic"} {
			if _, present := fields[property]; !present {
				style.Values[property] = semantic.KnownValue("0")
			}
		}
		styles[name] = style
	}

	var diagnostics []Diagnostic
	for _, dialogue := range doc.Dialogues {
		if _, exists := styles[semantic.DialogueStyleLookupName(dialogue.Style)]; !exists {
			continue
		}
		var usages []*fontUsage
		usageByContext := make(map[fontContext]*fontUsage)
		semantic.Evaluate(dialogue.ParsedText(), semantic.EvaluationOptions{
			Styles: styles, DialogueStyle: dialogue.Style, SkipNoEffectProofs: true,
			Observer: semantic.Observer{
				Text: func(text string, start int, state semantic.StateView) {
					family := state.Value("fontname")
					bold := state.Value("bold")
					italic := state.Value("italic")
					drawing := state.Value("drawing_scale")
					if !family.Known || !bold.Known || !italic.Known || !drawing.Known {
						return
					}
					scale, err := strconv.ParseInt(drawing.Value, 10, 32)
					if err != nil || scale > 0 {
						return
					}
					visible := fontVisibleText(text)
					if visible == "" {
						return
					}
					context := fontContext{
						family: family.Value,
						bold:   styleBoolean(bold.Value),
						italic: styleBoolean(italic.Value),
					}
					usage := usageByContext[context]
					if usage == nil {
						usage = &fontUsage{context: context, column: start + 1}
						usageByContext[context] = usage
						usages = append(usages, usage)
					}
					usage.text.WriteString(visible)
				},
			},
		})
		for _, usage := range usages {
			missing, found, err := checker.missingRunes(usage.context, usage.text.String())
			if err != nil {
				return nil, err
			}
			var id, detail string
			if !found {
				id = IssueFontMissing
				detail = fmt.Sprintf("Font family %q was not found in the scanned fonts.", usage.context.family)
			} else if len(missing) > 0 {
				id = IssueMissingGlyphs
				detail = fmt.Sprintf("Font family %q does not contain these characters: %q.", usage.context.family, string(missing))
			} else {
				continue
			}
			rule := Rules[id]
			diagnostics = append(diagnostics, Diagnostic{
				ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
				Fix: rule.Fix, Line: dialogue.Line, Column: usage.column, Detail: detail, Sources: rule.Sources,
			})
		}
	}
	return diagnostics, nil
}

func styleBoolean(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && value != "0"
}

func fontVisibleText(text string) string {
	var visible strings.Builder
	for i := 0; i < len(text); {
		if text[i] == '\\' && i+1 < len(text) {
			switch text[i+1] {
			case 'N', 'n':
				i += 2
				continue
			case 'h':
				visible.WriteRune('\u00a0')
				i += 2
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(text[i:])
		visible.WriteRune(r)
		i += size
	}
	return visible.String()
}
