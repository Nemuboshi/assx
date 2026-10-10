package renderer

import (
	"slices"
	"strings"

	"assx/internal/ass"
)

var assEventColumns = strings.Fields("layer start end style name marginl marginr marginv effect text")
var assStyleColumns = strings.Fields("name fontname fontsize primarycolour secondarycolour outlinecolour backcolour bold italic underline strikeout scalex scaley spacing angle borderstyle outline shadow alignment marginl marginr marginv encoding")

// RecordLayout describes source partitioning, not field validity or rendering.
// Unknown dialects retain their source with Known=false rather than borrowing
// the legacy document parser's field interpretation.
type RecordLayout struct {
	Line     int
	Source   ass.ConcreteSpan
	Kind     string
	Columns  []string
	Cells    []ass.ConcreteCell
	Known    bool
	Citation string
}

// ResolveDocumentLayouts independently partitions ASS v4+ Style and Dialogue
// records. libass honors Format names; both VSFilter parsers read fixed slots.
// SSA/v4++ and malformed framing remain unresolved until explicitly modeled.
func (p Profile) ResolveDocumentLayouts(tree ass.ConcreteDocument) []RecordLayout {
	eventColumns, styleColumns := slices.Clone(assEventColumns), slices.Clone(assStyleColumns)
	scriptASS := false
	section := ""
	var out []RecordLayout
	for _, line := range tree.Lines {
		if line.Header != nil {
			section = strings.ToLower(strings.TrimSpace(line.Header.RawName))
			continue
		}
		if line.Record == nil {
			continue
		}
		record := *line.Record
		key := strings.ToLower(strings.TrimSpace(record.Key))
		exactRecordKey := strings.TrimLeft(record.Key, " \t")
		if key == "scripttype" && (p.kind != Libass || section == "script info" && exactRecordKey == "ScriptType") {
			scriptASS = strings.EqualFold(strings.TrimSpace(record.Body), "v4.00+")
		}
		if key == "format" && p.kind == Libass && exactRecordKey == "Format" {
			columns := ass.ParseFormat(record.Body)
			for i, column := range columns {
				if column == "actor" {
					columns[i] = "name"
				}
			}
			if section == "events" {
				eventColumns = columns
			}
			if section == "v4+ styles" {
				styleColumns = columns
			}
		}
		if key != "dialogue" && key != "style" {
			continue
		}
		columns := styleColumns
		known := section == "v4+ styles"
		tail := -1
		if key == "dialogue" {
			columns = eventColumns
			known = section == "events" && scriptASS
			tail = slices.Index(columns, "text")
		}
		// libass requires the exact record keyword; case-insensitive VSFilter
		// acceptance is documented separately, never inferred from the CST.
		exactKey := "Style"
		if key == "dialogue" {
			exactKey = "Dialogue"
		}
		if p.kind == Libass && exactRecordKey != exactKey {
			known = false
		}
		out = append(out, RecordLayout{Line: line.Number, Source: line.ContentSpan, Kind: key, Columns: slices.Clone(columns), Cells: record.Partition(tree.Source, len(columns), tail), Known: known, Citation: p.documentCitation()})
	}
	return out
}

func (p Profile) documentCitation() string {
	switch p.kind {
	case Libass:
		return "libass@" + p.Version() + ":libass/ass.c:478-529,640-762,827-857"
	case XYVSFilter:
		return "xy-VSFilter@" + p.Version() + ":src/subtitles/STS.cpp:1425-1558"
	default:
		return "VSFilterMod@" + p.Version() + ":src/subtitles/STS.cpp:1675-1940,1980-2029"
	}
}
