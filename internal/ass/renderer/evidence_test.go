package renderer

import (
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"assx/internal/ass/spec"
)

func repoFile(parts ...string) string {
	return filepath.Join(append([]string{"..", "..", ".."}, parts...)...)
}

func TestGeneratedMatrixHasNoEvidenceDrift(t *testing.T) {
	data, err := os.ReadFile(repoFile("docs", "ass-tags.xml"))
	if err != nil {
		t.Fatal(err)
	}
	// Git may materialize XML using CRLF on Windows and LF on CI.
	// Evidence checksums deliberately ignore physical newline style.
	canonical := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	canonical = bytes.ReplaceAll(canonical, []byte("\r"), []byte("\n"))
	actual := fmt.Sprintf("%x", sha256.Sum256(canonical))
	if actual != matrixSHA256 {
		t.Fatalf("evidence matrix changed without regenerating metadata: want %s, have %s; run go generate ./internal/ass/renderer", actual, matrixSHA256)
	}
	var m struct {
		Pins []struct {
			Renderer string `xml:"renderer,attr"`
			Commit   string `xml:"commit,attr"`
		} `xml:"meta>pin"`
	}
	if err := xml.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Pins) != 3 {
		t.Fatalf("unexpected source pin count: %d", len(m.Pins))
	}
	for _, pin := range m.Pins {
		var kind Kind
		switch pin.Renderer {
		case "libass":
			kind = Libass
		case "xy-VSFilter":
			kind = XYVSFilter
		case "VSFilterMod":
			kind = VSFilterMod
		default:
			t.Errorf("unexpected pinned renderer %s", pin.Renderer)
			continue
		}
		p, _ := Standard(kind)
		if p.Version() != pin.Commit {
			t.Errorf("renderer pin drift: %s vs %s", p.Version(), pin.Commit)
		}
	}
}

func TestRegisteredNamesHaveEvidenceEntries(t *testing.T) {
	for _, names := range [][]string{libassOrder, xyRegistry, modOrder} {
		seen := map[string]bool{}
		for _, name := range names {
			if seen[name] {
				t.Errorf("duplicate command in dispatch order: %s", name)
			}
			seen[name] = true
			if _, ok := spec.TagSpecs[name]; !ok {
				t.Errorf("%s is not in the semantic registry", name)
			}
			if _, ok := matrixTags[name]; !ok {
				t.Errorf("%s is missing evidence metadata", name)
			}
		}
	}
	if _, ok := matrixTags["fad"]; !ok {
		t.Error("exact fad application lacks evidence")
	}
}

func readOptional(t *testing.T, parts ...string) string {
	t.Helper()
	path := repoFile(append([]string{"ref", "renderers"}, parts...)...)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Skipf("optional pinned source checkout absent: %s", path)
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
func orderedUnique(matches [][]string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range matches {
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		out = append(out, m[1])
	}
	return out
}

// The source checkouts are deliberately optional (ref/ is ignored by git).
// When present, test actual dispatch order instead of assuming that lexical
// evidence or XML scenarios are a complete substitute for implementation.
func TestDispatchTablesMatchPinnedSourceWhenAvailable(t *testing.T) {
	t.Run("libass", func(t *testing.T) {
		data := readOptional(t, "libass", "libass", "ass_parse.c")
		start := strings.Index(data, "// New tags introduced in vsfilter 2.39")
		if start < 0 {
			t.Fatal("libass dispatch anchor missing")
		}
		end := strings.Index(data[start:], "return p;")
		if end < 0 {
			t.Fatal("libass dispatch terminator missing")
		}
		rx := regexp.MustCompile("(?:complex_)?tag\\(\"([^\"]+)\"\\)")
		names := orderedUnique(rx.FindAllStringSubmatch(data[start:start+end], -1))
		if !reflect.DeepEqual(names, libassOrder) {
			t.Errorf("libass branch order drift: source=%v implementation=%v", names, libassOrder)
		}
	})
	t.Run("xy-VSFilter", func(t *testing.T) {
		data := readOptional(t, "xy-VSFilter", "src", "subtitles", "RTS.cpp")
		start := strings.Index(data, "void CRenderedTextSubtitle::InitCmdMap()")
		if start < 0 {
			t.Fatal("xy map anchor missing")
		}
		end := strings.Index(data[start:], "m_cmd_pos_level.SetCount")
		if end < 0 {
			t.Fatal("xy map terminator missing")
		}
		rx := regexp.MustCompile("m_cmdMap.SetAt\\(L\"([^\"]+)\"")
		names := orderedUnique(rx.FindAllStringSubmatch(data[start:start+end], -1))
		if !reflect.DeepEqual(names, xyRegistry) {
			t.Errorf("xy map drift: source=%v implementation=%v", names, xyRegistry)
		}
	})
	t.Run("VSFilterMod", func(t *testing.T) {
		data := readOptional(t, "VSFilterMod", "src", "subtitles", "RTS.cpp")
		start := strings.Index(data, "if(!cmd.Find(L\"1c\")")
		if start < 0 {
			t.Fatal("mod normalization anchor missing")
		}
		end := strings.Index(data[start:], "nUnrecognizedTags++;")
		if end < 0 {
			t.Fatal("mod normalization terminator missing")
		}
		rx := regexp.MustCompile("!\\s*cmd.Find\\(L\"([^\"]+)\"\\)")
		names := orderedUnique(rx.FindAllStringSubmatch(data[start:start+end], -1))
		if !reflect.DeepEqual(names, modOrder) {
			t.Errorf("mod normalization drift: source=%v implementation=%v", names, modOrder)
		}
		// A mistaken evidence note claimed be shadows blend. Character-wise
		// prefix checks cannot do that; blend is handled before the b branch.
		if !strings.HasPrefix("blend", "blend") || strings.HasPrefix("blend", "be") {
			t.Fatal("unexpected byte-prefix behavior")
		}
		if strings.Index(data, `cmd.Find(L"blend")`) < strings.Index(data, `cmd.Find(L"b")`) {
			// intentionally leave the more specific normalization before b
		} else {
			t.Fatal("blend would be shadowed by the generic b branch")
		}
	})
}
