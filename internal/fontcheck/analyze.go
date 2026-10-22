package fontcheck

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

type Kind string

const (
	FamilyAbsent      Kind = "family_absent"
	MissingCharacters Kind = "missing_characters"
)

// Face identifies a selected face, not a renderer fallback decision.
type Face struct {
	Path   string `json:"path"`
	Index  int    `json:"index"`
	Bold   bool   `json:"bold"`
	Italic bool   `json:"italic"`
}

// Finding groups equal requests on one dialogue line, in source order.
type Finding struct {
	Kind    Kind    `json:"kind"`
	Usages  []Usage `json:"usages"`
	Missing string  `json:"missing_characters,omitempty"`
	Matched []Face  `json:"matched_faces"`
}

// Analyze checks known visible requests against a complete catalog.
func Analyze(doc ass.Document, catalog *Catalog, profile renderer.Profile) ([]Finding, error) {
	return AnalyzeUsages(ExtractUsages(doc, profile), catalog)
}

// AnalyzeUsages checks extracted requests without evaluating the source again.
func AnalyzeUsages(usages []Usage, catalog *Catalog) ([]Finding, error) {
	if len(catalog.Issues) != 0 {
		return nil, fmt.Errorf("font catalog is incomplete: %s: %s", catalog.Issues[0].Path, catalog.Issues[0].Detail)
	}
	type key struct {
		line    int
		context fontContext
	}
	var groups []Finding
	indexes := make(map[key]int)
	for _, usage := range usages {
		if !usage.Known || usage.Drawing || usage.Text == "" {
			continue
		}
		k := key{usage.Line, fontContext{usage.Request.Family, usage.Request.Bold, usage.Request.Italic}}
		index, exists := indexes[k]
		if !exists {
			index = len(groups)
			indexes[k] = index
			groups = append(groups, Finding{})
		}
		groups[index].Usages = append(groups[index].Usages, usage)
	}
	var findings []Finding
	for _, group := range groups {
		request := group.Usages[0].Request
		context := fontContext{request.Family, request.Bold, request.Italic}
		var text strings.Builder
		for _, usage := range group.Usages {
			text.WriteString(usage.Text)
		}
		missing, found, err := catalog.missingRunes(context, text.String())
		if err != nil {
			return nil, err
		}
		if !found {
			group.Kind = FamilyAbsent
		} else if len(missing) > 0 {
			group.Kind = MissingCharacters
			group.Missing = string(missing)
			group.Matched = catalog.selectedFaces(context)
		} else {
			continue
		}
		findings = append(findings, group)
	}
	slices.SortStableFunc(findings, func(a, b Finding) int {
		return cmp.Or(cmp.Compare(a.Usages[0].Line, b.Usages[0].Line), cmp.Compare(a.Usages[0].Column, b.Usages[0].Column))
	})
	return findings, nil
}

func (c *Catalog) selectedFaces(context fontContext) []Face {
	candidates := c.families[normalizeFontFamily(context.family)]
	if len(candidates) == 0 {
		return nil
	}
	best := candidates[0]
	for _, face := range candidates[1:] {
		if styleDistance(face, context) < styleDistance(best, context) {
			best = face
		}
	}
	var selected []Face
	for _, face := range candidates {
		if face.path == best.path && styleDistance(face, context) == styleDistance(best, context) {
			selected = append(selected, Face{Path: face.path, Index: face.index, Bold: face.bold, Italic: face.italic})
		}
	}
	return slices.Compact(selected)
}
