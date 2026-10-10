package spec

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

type xmlMatrix struct {
	Meta struct {
		Pins []struct {
			Renderer string `xml:"renderer,attr"`
			Commit   string `xml:"commit,attr"`
			Path     string `xml:"path,attr"`
		} `xml:"pin"`
		Scenarios []struct {
			ID      string `xml:"id,attr"`
			Default string `xml:"default,attr"`
		} `xml:"scenarios>scenario"`
		Vocab struct {
			Results []struct {
				Name string `xml:"name,attr"`
			} `xml:"result"`
		} `xml:"vocab"`
		Kinds []struct {
			ID string `xml:"id,attr"`
		} `xml:"kinds>kind"`
	} `xml:"meta"`
	Tags []struct {
		Name   string     `xml:"name,attr"`
		Status string     `xml:"status,attr"`
		Params *xmlParams `xml:"params"`
		Scens  []struct {
			ID          string `xml:"id,attr"`
			Status      string `xml:"status,attr"`
			Verified    string `xml:"verified,attr"`
			Base        string `xml:"base,attr"`
			VSFilter    string `xml:"vsfilter,attr"`
			VSFilterMod string `xml:"vsfiltermod,attr"`
			Cite        string `xml:"cite,attr"`
		} `xml:"scen"`
	} `xml:"tag"`
	Groups []struct {
		Kind   string `xml:"kind,attr"`
		Status string `xml:"status,attr"`
		Names  string `xml:"names,attr"`
	} `xml:"taggroup"`
}

type xmlSig struct {
	N        string `xml:"n,attr"`
	Form     string `xml:"form,attr"`
	Renderer string `xml:"renderer,attr"`
	Params   []struct {
		I     string `xml:"i,attr"`
		Name  string `xml:"name,attr"`
		Kind  string `xml:"kind,attr"`
		Range string `xml:"range,attr"`
	} `xml:"p"`
}

var citeToken = regexp.MustCompile(`^(libass|xy|vsm|docs):[\w/+.@-]+:\d+(-\d+)?(,\d+(-\d+)?)*$`)

var sigForms = map[string]bool{"bare": true, "paren": true, "both": true}

var sigRenderers = map[string]bool{"": true, "libass": true, "xy": true, "vsm": true}

func loadMatrix(t *testing.T) *xmlMatrix {
	t.Helper()
	path := filepath.Join("..", "..", "..", "docs", "ass-tags.xml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("matrix file %s: %v", path, err)
	}
	var m xmlMatrix
	if err := xml.Unmarshal(data, &m); err != nil {
		t.Fatalf("%s is not well-formed: %v", path, err)
	}
	return &m
}

func TestMatrixStylesheetWellFormed(t *testing.T) {
	path := filepath.Join("..", "..", "..", "docs", "ass-tags.xsl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("stylesheet %s: %v", path, err)
	}
	if err := xml.Unmarshal(data, new(struct{})); err != nil {
		t.Errorf("%s is not well-formed XML: %v", path, err)
	}
}

func TestMatrixTagNamesMatchSpec(t *testing.T) {
	m := loadMatrix(t)
	inMatrix := make(map[string]bool)
	for _, tag := range m.Tags {
		if inMatrix[tag.Name] {
			t.Errorf("tag %q listed twice in docs/ass-tags.xml", tag.Name)
		}
		inMatrix[tag.Name] = true
	}
	for _, group := range m.Groups {
		for _, name := range strings.Fields(group.Names) {
			if inMatrix[name] {
				t.Errorf("tag %q appears both individually and in group %q", name, group.Kind)
			}
			inMatrix[name] = true
		}
	}
	for name := range TagSpecs {
		if !inMatrix[name] {
			t.Errorf("TagSpecs name %q is missing from docs/ass-tags.xml", name)
		}
	}
	for name := range inMatrix {
		if _, ok := TagSpecs[name]; !ok {
			t.Errorf("docs/ass-tags.xml lists unknown name %q (not in TagSpecs)", name)
		}
	}
}

func TestMatrixVocabulary(t *testing.T) {
	m := loadMatrix(t)

	scenarios := make(map[string]bool)
	for _, s := range m.Meta.Scenarios {
		if s.Default == "" {
			t.Errorf("scenario %q has no default result", s.ID)
		}
		scenarios[s.ID] = true
	}
	results := make(map[string]bool)
	for _, r := range m.Meta.Vocab.Results {
		results[r.Name] = true
	}
	kinds := make(map[string]bool)
	for _, k := range m.Meta.Kinds {
		kinds[k.ID] = true
	}
	if len(kinds) == 0 {
		t.Fatalf("<meta><kinds> declares no parameter kinds")
	}

	checkResult := func(where, value string) {
		t.Helper()
		if value != "" && !results[value] {
			t.Errorf("%s: result %q is not declared in <vocab>", where, value)
		}
	}

	checkCite := func(where, cite string) {
		t.Helper()
		if cite == "" {
			t.Errorf("%s: cite is required", where)
		}
		for _, token := range strings.Fields(cite) {
			if !citeToken.MatchString(token) {
				t.Errorf("%s: cite token %q is malformed (want renderer:file:lines)", where, token)
			}
		}
	}

	checkStatus := func(where, status string) {
		t.Helper()
		if status != "V" && status != "S" {
			t.Errorf("%s: status %q must be V or S", where, status)
		}
	}

	for _, tag := range m.Tags {
		checkStatus("tag "+tag.Name, tag.Status)
		if tag.Params == nil {
			if tag.Status == "V" {
				t.Errorf("tag %q: a V tag must declare its <params> signature list", tag.Name)
			}
		} else {
			checkParams(t, kinds, checkCite, tag.Name, tag.Params)
		}
		seen := make(map[string]bool)
		for _, scen := range tag.Scens {
			where := fmt.Sprintf("tag %q scen %q", tag.Name, scen.ID)
			if !scenarios[scen.ID] {
				t.Errorf("%s: scenario is not declared in <scenarios>", where)
			}
			if seen[scen.ID] {
				t.Errorf("%s: duplicate scenario row", where)
			}
			seen[scen.ID] = true
			checkStatus(where, scen.Status)
			if scen.Base == "" {
				t.Errorf("%s: base result is required", where)
			}
			checkResult(where, scen.Base)
			checkResult(where+" vsfilter", scen.VSFilter)
			checkResult(where+" vsfiltermod", scen.VSFilterMod)
			checkCite(where, scen.Cite)
		}
	}

	for _, group := range m.Groups {
		checkStatus("group "+group.Kind, group.Status)
		if strings.Fields(group.Names) == nil {
			t.Errorf("group %q: names attribute is empty", group.Kind)
		}
	}
}

// checkParams validates the argument-shape layer: one <sig> per accepted
// arity, and each parameter carries a declared kind plus a legal range.
func checkParams(t *testing.T, kinds map[string]bool, checkCite func(where, cite string), tagName string, params *xmlParams) {
	t.Helper()
	if params.Status != "V" && params.Status != "S" {
		t.Errorf("tag %q params: status %q must be V or S", tagName, params.Status)
	}
	checkCite("tag "+tagName+" params", params.Cite)

	seenSig := make(map[string]bool)
	for _, sig := range params.Sigs {
		n, err := strconv.Atoi(sig.N)
		if err != nil || n < 0 {
			t.Errorf("tag %q sig %q: n must be a non-negative arity", tagName, sig.N)
			continue
		}
		where := fmt.Sprintf("tag %q sig n=%s renderer=%q", tagName, sig.N, sig.Renderer)
		if !sigForms[sig.Form] {
			t.Errorf("%s: form %q is not bare, paren or both", where, sig.Form)
		}
		if !sigRenderers[sig.Renderer] {
			t.Errorf("%s: renderer %q is not libass, xy, vsm or empty (all)", where, sig.Renderer)
		}
		key := sig.N + "|" + sig.Form + "|" + sig.Renderer
		if seenSig[key] {
			t.Errorf("%s: duplicate signature", where)
		}
		seenSig[key] = true

		args := 0
		for i, p := range sig.Params {
			if want := strconv.Itoa(i + 1); p.I != want {
				t.Errorf("%s p %d: i=%q must be the 1-based position %s", where, i+1, p.I, want)
			}
			if p.Name == "" {
				t.Errorf("%s p %d: name is required", where, i+1)
			}
			if !kinds[p.Kind] {
				t.Errorf("%s p %q: kind %q is not declared in <meta><kinds>", where, p.Name, p.Kind)
			}
			if strings.TrimSpace(p.Range) == "" {
				t.Errorf("%s p %q: range is required (write \"any\" when nothing constrains it)", where, p.Name)
			}
			if p.Kind != "none" {
				args++
			}
		}
		if args != n {
			t.Errorf("%s: %d argument parameters (kind != none) do not match n=%d", where, args, n)
		}
	}
}

type xmlParams struct {
	Verified string   `xml:"verified,attr"`
	Status   string   `xml:"status,attr"`
	Cite     string   `xml:"cite,attr"`
	Sigs     []xmlSig `xml:"sig"`
}
