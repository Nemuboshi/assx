package semantic

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

// CompareDocument retains absolute decoded-source spans. Record layouts and
// Style input are independently derived for each profile, then the ordinary
// evaluator observes each renderer's own Text field. Layout agreement never
// proves Style value interpretation or whole-document rendering equivalence.
func CompareDocument(tree ass.ConcreteDocument, profiles []renderer.Profile) []CompatibilityFinding {
	if len(profiles) == 0 {
		libass, _ := renderer.Standard(renderer.Libass)
		xy, _ := renderer.Standard(renderer.XYVSFilter)
		profiles = []renderer.Profile{libass, xy}
	}
	unique := make([]renderer.Profile, 0, len(profiles))
	for _, p := range profiles {
		if !slices.Contains(unique, p) {
			unique = append(unique, p)
		}
	}
	if len(unique) < 2 {
		return nil
	}
	layouts := make([]map[ass.ConcreteSpan]renderer.RecordLayout, len(unique))
	runs := make([]map[ass.ConcreteSpan]*Interpretation, len(unique))
	spans := make(map[ass.ConcreteSpan]bool)
	for i, p := range unique {
		rows := p.ResolveDocumentLayouts(tree)
		layouts[i] = make(map[ass.ConcreteSpan]renderer.RecordLayout, len(rows))
		runs[i] = make(map[ass.ConcreteSpan]*Interpretation)
		var fields []ass.StyleField
		for _, row := range rows {
			layouts[i][row.Source] = row
			spans[row.Source] = true
			if row.Kind != "style" || !row.Known {
				continue
			}
			name := layoutValue(row, "name")
			for j, column := range row.Columns {
				if j >= len(row.Cells) || !row.Cells[j].Present {
					continue
				}
				fields = append(fields, ass.StyleField{StyleName: name, Name: column, Value: strings.TrimSpace(row.Cells[j].Raw), Line: row.Line})
			}
		}
		styles := StyleStatesByName(fields)
		for _, row := range rows {
			if row.Kind != "dialogue" || !row.Known {
				continue
			}
			text := slices.Index(row.Columns, "text")
			if text < 0 || text >= len(row.Cells) || !row.Cells[text].Present {
				continue
			}
			cell := row.Cells[text]
			observations := observeInterpretation(ass.ParseConcreteDialogue(cell.Raw), p, EvaluationOptions{Styles: styles, DialogueStyle: layoutValue(row, "style")})
			for span, o := range observations {
				offset := cell.Span.Start
				absolute := ass.ConcreteSpan{Start: span.Start + offset, End: span.End + offset}
				o.Resolution.Source = absolute
				for k := range o.Resolution.Args {
					o.Resolution.Args[k].Span.Start += offset
					o.Resolution.Args[k].Span.End += offset
				}
				o.Event.Tag.Start += offset
				o.Event.Tag.End += offset
				for slot, owner := range o.Owners {
					owner.Start += offset
					owner.End += offset
					o.Owners[slot] = owner
				}
				runs[i][absolute] = o
			}
		}
	}
	var out []CompatibilityFinding
	order := make([]ass.ConcreteSpan, 0, len(spans))
	for span := range spans {
		order = append(order, span)
	}
	sort.Slice(order, func(i, j int) bool { return order[i].Start < order[j].Start })
	for _, span := range order {
		for i := range unique {
			for j := i + 1; j < len(unique); j++ {
				a, b := layouts[i][span], layouts[j][span]
				status := CompatibilityUnresolved
				if a.Known && b.Known {
					status = CompatibilityEquivalent
					if !slices.Equal(a.Columns, b.Columns) || !slices.Equal(a.Cells, b.Cells) {
						status = CompatibilityDivergent
					}
				}
				left, right := &Interpretation{Profile: unique[i], Present: a.Known}, &Interpretation{Profile: unique[j], Present: b.Known}
				out = append(out, CompatibilityFinding{Source: span, Dimension: "document/" + a.Kind + "-layout", Status: status, Left: left, Right: right, Detail: fmt.Sprintf("%s: %s; %s: %s.", unique[i].Kind(), layoutDescription(a), unique[j].Kind(), layoutDescription(b)), Citations: []string{a.Citation, b.Citation}})
				if a.Kind == "style" {
					out = append(out, CompatibilityFinding{Source: span, Dimension: "document/style-values", Status: CompatibilityUnresolved, Left: left, Right: right, Detail: "Style field layout is compared independently; renderer-specific value conversion and fallback compatibility remain unresolved."})
				}
			}
		}
	}
	tagSpans := make(map[ass.ConcreteSpan]bool)
	for _, run := range runs {
		for span := range run {
			tagSpans[span] = true
		}
	}
	order = order[:0]
	for span := range tagSpans {
		order = append(order, span)
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].Start == order[j].Start {
			return order[i].End < order[j].End
		}
		return order[i].Start < order[j].Start
	})
	for _, span := range order {
		for i := range unique {
			for j := i + 1; j < len(unique); j++ {
				a, b := observationAt(runs[i], span, unique[i]), observationAt(runs[j], span, unique[j])
				out = append(out, compareInterpretations(span, a, b)...)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Source.Start < out[j].Source.Start })
	return out
}

func layoutValue(row renderer.RecordLayout, name string) string {
	index := slices.Index(row.Columns, name)
	if index < 0 || index >= len(row.Cells) || !row.Cells[index].Present {
		return ""
	}
	return strings.TrimSpace(row.Cells[index].Raw)
}
func layoutDescription(row renderer.RecordLayout) string {
	if !row.Known {
		return "unresolved record dialect or framing"
	}
	parts := make([]string, 0, len(row.Columns))
	for i, column := range row.Columns {
		cell := row.Cells[i]
		parts = append(parts, fmt.Sprintf("%s=%q at [%d,%d)", column, cell.Raw, cell.Span.Start, cell.Span.End))
	}
	return strings.Join(parts, ", ")
}
