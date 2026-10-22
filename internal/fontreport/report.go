// Package fontreport presents font results without lint rules or edit metadata.
package fontreport

import (
	"bufio"
	"fmt"
	"io"

	"assx/internal/ass/renderer"
	"assx/internal/fontcheck"
	"assx/internal/report"
	"assx/internal/report/pretty"
)

type Renderer struct {
	Name     string `json:"name"`
	Revision string `json:"revision"`
	Mod      string `json:"mod_build"`
	Lua      string `json:"lua_build"`
}

type Catalog struct {
	Mode      string `json:"mode"`
	Directory string `json:"directory,omitempty"`
}

type Diagnostic struct {
	ID       string            `json:"id"`
	Severity string            `json:"severity"`
	Detail   string            `json:"detail"`
	Usages   []fontcheck.Usage `json:"usages"`
	Missing  string            `json:"missing_characters,omitempty"`
	Matched  []fontcheck.Face  `json:"matched_faces"`
}

// Result is the versioned font-domain JSON contract. It has no fix fields.
type Result struct {
	Domain    string            `json:"domain"`
	Version   int               `json:"version"`
	File      string            `json:"file"`
	Renderer  Renderer          `json:"renderer"`
	Catalog   Catalog           `json:"catalog"`
	Findings  []Diagnostic      `json:"findings"`
	Unchecked []fontcheck.Usage `json:"unchecked"`
	Limit     string            `json:"limit"`
}

func Build(path string, profile renderer.Profile, source fontcheck.Source, findings []fontcheck.Finding, usages []fontcheck.Usage) Result {
	r := Result{Domain: "font", Version: 1, File: path,
		Renderer: Renderer{Name: profile.Kind().String(), Revision: profile.Version(), Mod: feature(profile.Build().Mod), Lua: feature(profile.Build().Lua)},
		Catalog:  Catalog{Mode: "system", Directory: source.Directory},
		Findings: []Diagnostic{}, Unchecked: []fontcheck.Usage{},
		Limit: "Glyph lookup does not test renderer fallback or shaping.",
	}
	if source.Directory != "" {
		r.Catalog.Mode = "directory"
	}
	for _, f := range findings {
		d := Diagnostic{ID: "FONT001", Severity: "info", Usages: f.Usages, Missing: f.Missing, Matched: f.Matched}
		family := f.Usages[0].Request.Family
		d.Detail = fmt.Sprintf("Font family %q was not found in the scanned font set.", family)
		if f.Kind == fontcheck.MissingCharacters {
			d.ID = "FONT002"
			d.Detail = fmt.Sprintf("Selected font face(s) for %q do not contain these characters: %q.", family, f.Missing)
		}
		r.Findings = append(r.Findings, d)
	}
	for _, usage := range usages {
		if !usage.Known && !usage.Drawing && usage.Text != "" {
			r.Unchecked = append(r.Unchecked, usage)
		}
	}
	return r
}

func feature(f renderer.Feature) string {
	switch f {
	case renderer.FeatureEnabled:
		return "enabled"
	case renderer.FeatureDisabled:
		return "disabled"
	default:
		return "unknown"
	}
}

func (r Result) notes() []string {
	catalog := "Font catalog: system directories."
	if r.Catalog.Mode == "directory" {
		catalog = fmt.Sprintf("Font catalog: exclusive directory %q.", r.Catalog.Directory)
	}
	return []string{catalog, "Renderer: " + r.Renderer.Name + "@" + r.Renderer.Revision + " (_VSMOD=" + r.Renderer.Mod + ", _LUA=" + r.Renderer.Lua + ")",
		fmt.Sprintf("Unchecked text runs: %d (unknown semantic state).", len(r.Unchecked)), r.Limit}
}

func Pretty(w io.Writer, r Result, source string, color bool, width int) error {
	view := report.Summary{Heading: "Font check: ", Advisory: true, Total: len(r.Findings), Notes: r.notes()}
	group := report.Group{File: r.File}
	for _, d := range r.Findings {
		u := d.Usages[0]
		group.Diagnostics = append(group.Diagnostics, report.Diagnostic{
			ID: d.ID, Severity: d.Severity, Title: "Font dependency", Detail: d.Detail,
			Line: u.Line, Column: u.Column, SourceColumn: u.SourceColumn, SourceWidth: u.Span.End - u.Span.Start,
		})
	}
	view.Groups = []report.Group{group}
	return pretty.Render(w, view, source, color, width)
}

func Plain(w io.Writer, r Result) error {
	out := bufio.NewWriter(w)
	_, _ = fmt.Fprintf(out, "Font check: %q\n", r.File)
	for _, note := range r.notes() {
		_, _ = fmt.Fprintln(out, note)
	}
	for _, d := range r.Findings {
		u := d.Usages[0]
		_, _ = fmt.Fprintf(out, "INFO[%s] %d:%d %s\n", d.ID, u.Line, u.SourceColumn, d.Detail)
	}
	_, _ = fmt.Fprintf(out, "%d advisory findings.\n", len(r.Findings))
	return out.Flush()
}
