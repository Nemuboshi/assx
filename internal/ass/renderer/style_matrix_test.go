package renderer

import (
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The Style-field matrix is validated separately from the tag matrix. It shares
// the pins and the evidence discipline, but its rows are conversions, not
// signatures. These tests keep the evidence honest: every conversion must be
// either cited in pinned source or explicitly unmodeled.

type styleMatrixXML struct {
	Meta struct {
		Pins []struct {
			Renderer string `xml:"renderer,attr"`
			Commit   string `xml:"commit,attr"`
			Path     string `xml:"path,attr"`
		} `xml:"pin"`
		Parses []struct {
			ID string `xml:"id,attr"`
		} `xml:"parses>parse"`
		Posts []struct {
			ID string `xml:"id,attr"`
		} `xml:"posts>post"`
	} `xml:"meta"`
	Fields []struct {
		Name  string `xml:"name,attr"`
		Kind  string `xml:"kind,attr"`
		Convs []struct {
			Renderer string `xml:"renderer,attr"`
			Parse    string `xml:"parse,attr"`
			Post     string `xml:"post,attr"`
			Verified string `xml:"verified,attr"`
			Cite     string `xml:"cite,attr"`
		} `xml:"conv"`
	} `xml:"stylefield"`
}

// styleMatrixPins mirrors the tag matrix pins. The two matrices must not drift.
var styleMatrixPins = map[string]struct {
	commit string
	path   string
}{
	"libass":      {"f61db567e6593df3470e91594bcd4ad2d0473aff", "ref/renderers/libass"},
	"xy-VSFilter": {"135a30153a38fa846cb5c39df0f258403e92096e", "ref/renderers/xy-VSFilter"},
	"VSFilterMod": {"7a00567e4a49b6310691b9a6791646b2a018bfa2", "ref/renderers/VSFilterMod"},
}

var styleCiteToken = regexp.MustCompile(`^(libass|xy|vsm):[\w/+.@-]+:\d+(-\d+)?(,\d+(-\d+)?)*$`)

// styleMatrixRenderers is the required conv set for every field row.
var styleMatrixRenderers = []string{"libass", "xy", "vsm"}

func loadStyleMatrix(t *testing.T) *styleMatrixXML {
	t.Helper()
	path := filepath.Join("..", "..", "..", "docs", "ass-style-fields.xml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("matrix file %s: %v", path, err)
	}
	var m styleMatrixXML
	if err := xml.Unmarshal(data, &m); err != nil {
		t.Fatalf("%s is not well-formed: %v", path, err)
	}
	return &m
}

func TestStyleMatrixRendererSourcePins(t *testing.T) {
	m := loadStyleMatrix(t)
	if len(m.Meta.Pins) != len(styleMatrixPins) {
		t.Fatalf("expected %d unique renderer pins, got %d", len(styleMatrixPins), len(m.Meta.Pins))
	}
	seen := make(map[string]bool)
	for _, pin := range m.Meta.Pins {
		expected, ok := styleMatrixPins[pin.Renderer]
		if !ok || seen[pin.Renderer] {
			t.Errorf("unexpected or duplicate renderer pin %q", pin.Renderer)
			continue
		}
		seen[pin.Renderer] = true
		if pin.Commit != expected.commit || pin.Path != expected.path {
			t.Errorf("%s pin drift: commit=%q path=%q (want %q, %q)",
				pin.Renderer, pin.Commit, pin.Path, expected.commit, expected.path)
		}
		if !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(pin.Commit) {
			t.Errorf("%s must use a full 40-character commit SHA", pin.Renderer)
		}
		local := filepath.Join("..", "..", "..", filepath.FromSlash(pin.Path))
		if _, err := os.Stat(filepath.Join(local, ".git")); err == nil {
			output, err := exec.Command("git", "-C", local, "rev-parse", "HEAD").CombinedOutput()
			if err != nil {
				t.Errorf("%s local checkout: %v: %s", pin.Renderer, err, output)
			} else if got := strings.TrimSpace(string(output)); got != expected.commit {
				t.Errorf("%s local checkout at %s, want pinned %s", pin.Renderer, got, expected.commit)
			}
		}
	}
}

// Every citation must name a real pinned file and a line range inside it. A
// citation that does not resolve is worse than no citation: it reads as proof.
func TestStyleMatrixCitationsResolve(t *testing.T) {
	m := loadStyleMatrix(t)
	root := filepath.Join("..", "..", "..")
	for _, field := range m.Fields {
		for _, conv := range field.Convs {
			where := "stylefield " + field.Name + "/" + conv.Renderer
			if conv.Cite == "" {
				continue
			}
			for _, token := range strings.Fields(conv.Cite) {
				if !styleCiteToken.MatchString(token) {
					t.Errorf("%s: cite token %q is malformed", where, token)
					continue
				}
				rendererName, rest, _ := strings.Cut(token, ":")
				path, lines, _ := strings.Cut(rest, ":")
				pin, ok := styleMatrixPins[rendererForToken(rendererName)]
				if !ok {
					t.Errorf("%s: cite token %q has no pin", where, token)
					continue
				}
				data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(pin.path), filepath.FromSlash(path)))
				if err != nil {
					t.Errorf("%s: cite token %q: %v", where, token, err)
					continue
				}
				total := len(strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"))
				for _, part := range strings.Split(lines, ",") {
					low, high, _ := strings.Cut(part, "-")
					if high == "" {
						high = low
					}
					lowN, highN := atoiOrFail(t, where, token, low), atoiOrFail(t, where, token, high)
					if lowN < 1 || highN < lowN || highN > total {
						t.Errorf("%s: cite token %q range %s-%s outside file (%d lines)", where, token, low, high, total)
					}
				}
			}
		}
	}
}

func atoiOrFail(t *testing.T, where, token, raw string) int {
	t.Helper()
	value := 0
	if raw == "" {
		t.Errorf("%s: cite token %q has an empty line number", where, token)
		return 0
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			t.Errorf("%s: cite token %q has a non-numeric line %q", where, token, raw)
			return 0
		}
		value = value*10 + int(r-'0')
	}
	return value
}

func rendererForToken(name string) string {
	switch name {
	case "xy":
		return "xy-VSFilter"
	case "vsm":
		return "VSFilterMod"
	default:
		return name
	}
}

// A conversion is either cited and verified, or uncited and unmodeled. No row
// may claim a modeled step without a citation, and no row may claim a citation
// without naming its own renderer.
func TestStyleMatrixConversionEvidence(t *testing.T) {
	m := loadStyleMatrix(t)
	for _, field := range m.Fields {
		if field.Name == "" {
			t.Errorf("stylefield row without a name")
			continue
		}
		if len(field.Convs) != len(styleMatrixRenderers) {
			t.Errorf("stylefield %s: need one conv per renderer, got %d", field.Name, len(field.Convs))
		}
		byRenderer := make(map[string]bool)
		for _, conv := range field.Convs {
			where := "stylefield " + field.Name + "/" + conv.Renderer
			if byRenderer[conv.Renderer] {
				t.Errorf("%s: duplicate conv", where)
			}
			byRenderer[conv.Renderer] = true
			if conv.Verified != "" && conv.Verified != conv.Renderer {
				t.Errorf("%s: verified=%q must name its own renderer", where, conv.Verified)
			}
			if conv.Cite == "" {
				if conv.Verified != "" {
					t.Errorf("%s: verified scope without a citation", where)
				}
				if conv.Parse != "unmodeled" || conv.Post != "unmodeled" {
					t.Errorf("%s: uncited conversion must be unmodeled, got parse=%q post=%q", where, conv.Parse, conv.Post)
				}
				continue
			}
			if conv.Verified != conv.Renderer {
				t.Errorf("%s: citation requires verified=%q", where, conv.Renderer)
			}
			if conv.Parse == "unmodeled" && conv.Post == "unmodeled" {
				t.Errorf("%s: citation must model at least one step", where)
			}
		}
	}
}

func TestStyleMatrixVocabularyDeclared(t *testing.T) {
	m := loadStyleMatrix(t)
	parses := make(map[string]bool)
	for _, p := range m.Meta.Parses {
		if p.ID == "" || parses[p.ID] {
			t.Errorf("duplicate or unnamed parse token %q", p.ID)
		}
		parses[p.ID] = true
	}
	posts := make(map[string]bool)
	for _, p := range m.Meta.Posts {
		if p.ID == "" || posts[p.ID] {
			t.Errorf("duplicate or unnamed post token %q", p.ID)
		}
		posts[p.ID] = true
	}
	if len(parses) == 0 || len(posts) == 0 {
		t.Fatalf("matrix declares no parse or post vocabulary")
	}
	for _, field := range m.Fields {
		for _, conv := range field.Convs {
			if !parses[conv.Parse] {
				t.Errorf("stylefield %s/%s: undeclared parse token %q", field.Name, conv.Renderer, conv.Parse)
			}
			if !posts[conv.Post] {
				t.Errorf("stylefield %s/%s: undeclared post token %q", field.Name, conv.Renderer, conv.Post)
			}
		}
	}
}

// Every standard v4+ Style column and the SSA-only columns must have a row, so
// a new column cannot be added to the parser without an evidence decision here.
func TestStyleMatrixCoversKnownColumns(t *testing.T) {
	m := loadStyleMatrix(t)
	have := make(map[string]bool)
	for _, field := range m.Fields {
		have[field.Name] = true
	}
	for _, column := range assStyleColumns {
		if !have[column] {
			t.Errorf("stylefield %q has no evidence row", column)
		}
	}
	for _, column := range []string{"tertiarycolour", "alphalevel", "justify"} {
		if !have[column] {
			t.Errorf("SSA-only stylefield %q has no evidence row", column)
		}
	}
}

// The generated table must match the XML. The generator -check covers the same
// ground; this keeps a stale in-package table from passing silently.
func TestStyleMatrixGeneratedTableMatchesXML(t *testing.T) {
	m := loadStyleMatrix(t)
	if len(styleFieldMatrix) != len(m.Fields) {
		t.Fatalf("generated table has %d rows, XML declares %d", len(styleFieldMatrix), len(m.Fields))
	}
	for _, field := range m.Fields {
		row, ok := styleFieldMatrix[field.Name]
		if !ok {
			t.Errorf("generated table is missing %q", field.Name)
			continue
		}
		if row.kind != field.Kind {
			t.Errorf("%s: kind %q, XML declares %q", field.Name, row.kind, field.Kind)
		}
		for _, conv := range field.Convs {
			got, ok := row.convs[conv.Renderer]
			if !ok {
				t.Errorf("%s: generated table is missing renderer %q", field.Name, conv.Renderer)
				continue
			}
			if got.parse != conv.Parse || got.post != conv.Post || got.citation != conv.Cite {
				t.Errorf("%s/%s: generated %#v, XML declares parse=%q post=%q cite=%q",
					field.Name, conv.Renderer, got, conv.Parse, conv.Post, conv.Cite)
			}
			if want := conv.Verified == conv.Renderer; got.verified != want {
				t.Errorf("%s/%s: generated verified=%t, want %t", field.Name, conv.Renderer, got.verified, want)
			}
		}
	}
}

// StyleFieldConversion must answer for every pinned profile and must refuse an
// unknown field, so a caller cannot read a zero conversion as evidence.
func TestStyleFieldConversionLookup(t *testing.T) {
	for _, kind := range []Kind{Libass, XYVSFilter, VSFilterMod} {
		profile, err := Standard(kind)
		if err != nil {
			t.Fatal(err)
		}
		parse, post, verified, citation, ok := profile.StyleFieldConversion("fontsize")
		if !ok || !verified || citation == "" {
			t.Fatalf("%s: fontsize conversion not verified: %q %q %t %q", kind, parse, post, verified, citation)
		}
		if parse != "libass-double" && parse != "vsfilter-float" {
			t.Errorf("%s: unexpected fontsize parse %q", kind, parse)
		}
		if _, _, _, _, ok := profile.StyleFieldConversion("no-such-field"); ok {
			t.Errorf("%s: unknown field resolved", kind)
		}
	}
}
