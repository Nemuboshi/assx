package semantic

import (
	"fmt"
	"slices"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

// compareStyleBorders compares declared Style behavior, not final Dialogue pixels.
func compareStyleBorders(a, b renderer.RecordLayout, left, right *Interpretation) []CompatibilityFinding {
	if a.Kind != "style" || b.Kind != "style" || !a.Known || !b.Known {
		return nil
	}
	for _, row := range [2]renderer.RecordLayout{a, b} {
		for _, cell := range row.Cells {
			if !cell.Present {
				return nil
			}
		}
	}
	cell, ok := styleComparisonCell(a, b, "borderstyle")
	if !ok {
		return nil
	}
	raw := strings.Trim(cell.Raw, " \t\r\n\v\f")
	value := ass.DecodeExactInteger(raw)
	if value.Status != ass.ValueValid || value.Raw != raw {
		return nil
	}
	lk, rk := left.Profile.Kind(), right.Profile.Kind()
	dimension, detail := "", ""
	switch {
	case value.Integer == 4 && (lk == renderer.Libass) != (rk == renderer.Libass):
		dimension = "document/style-border-mode"
		detail = "BorderStyle 4 selects a whole-event background in libass and a normal glyph outline in VSFilter."
	case value.Integer == 3 && (lk == renderer.VSFilterMod) != (rk == renderer.VSFilterMod):
		x, xok := styleComparisonNumber(a, b, "scalex")
		y, yok := styleComparisonNumber(a, b, "scaley")
		outline, ook := styleComparisonNumber(a, b, "outline")
		if !xok || !yok || !ook || x <= 0 || y <= 0 || outline <= 0 || x == 100 && y == 100 {
			return nil
		}
		dimension = "document/style-box-scaling"
		detail = fmt.Sprintf("BorderStyle 3 declares ScaleX=%g, ScaleY=%g, and Outline=%g. VSFilterMod skips the extra box font scaling used by libass and xy-VSFilter; box dimensions can differ.", x, y, outline)
	default:
		return nil
	}
	start := cell.Span.Start + strings.Index(cell.Raw, raw)
	return []CompatibilityFinding{{
		Source:    ass.ConcreteSpan{Start: start, End: start + len(raw)},
		Dimension: dimension, Status: CompatibilityDivergent, Left: left, Right: right,
		Detail: detail, Citations: []string{styleBorderCitation(left.Profile), styleBorderCitation(right.Profile)},
	}}
}

func styleComparisonCell(a, b renderer.RecordLayout, name string) (ass.ConcreteCell, bool) {
	var cell ass.ConcreteCell
	for n, row := range [2]renderer.RecordLayout{a, b} {
		i := slices.Index(row.Columns, name)
		if i < 0 || i >= len(row.Cells) || !row.Cells[i].Present || slices.Contains(row.Columns[i+1:], name) {
			return ass.ConcreteCell{}, false
		}
		if n == 0 {
			cell = row.Cells[i]
		} else if cell.Span != row.Cells[i].Span || cell.Raw != row.Cells[i].Raw {
			return ass.ConcreteCell{}, false
		}
	}
	return cell, true
}

func styleComparisonNumber(a, b renderer.RecordLayout, name string) (float64, bool) {
	cell, ok := styleComparisonCell(a, b, name)
	if !ok {
		return 0, false
	}
	raw := strings.Trim(cell.Raw, " \t\r\n\v\f")
	value := ass.DecodeExactNumber(raw)
	// Keep float precision differences separate from box scaling.
	return value.Number, value.Status == ass.ValueValid && value.Raw == raw && float64(float32(value.Number)) == value.Number
}

func styleBorderCitation(profile renderer.Profile) string {
	var path string
	switch profile.Kind() {
	case renderer.Libass:
		path = "libass/ass.c:709;libass/ass_render.c:829,1433-1451,2759-2790,3009-3010"
	case renderer.XYVSFilter:
		path = "src/subtitles/STS.cpp:1519,1540;src/subtitles/RTS.cpp:428-431,561-578,1259-1261"
	case renderer.VSFilterMod:
		path = "src/subtitles/STS.cpp:1873,1903;src/subtitles/RTS.cpp:286-287,662-685,1427"
	}
	return profile.Kind().String() + "@" + profile.Version() + ":" + path
}
