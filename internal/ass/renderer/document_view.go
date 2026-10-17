package renderer

import (
	"context"
	"slices"
	"strings"

	"assx/internal/ass"
)

// DocumentView projects known ASS v4+ fixed-position VSFilter record groups for lint consumers.
// Groups with unknown layouts retain the parser's original fields.
func (p Profile) DocumentView(doc ass.Document) ass.Document {
	if p.skipsProjection(doc) {
		return doc
	}
	tree := ass.ParseConcreteDocument(doc.Text)
	view, err := p.projectDocument(context.Background(), doc, tree, p.ResolveDocumentLayouts(tree))
	if err != nil {
		return doc
	}
	return view
}

// skipsProjection reports the guard cases that need no concrete document.
func (p Profile) skipsProjection(doc ass.Document) bool {
	return p.kind == Libass || len(doc.Lines) == 0 || len(doc.Dialogues) == 1 && len(doc.Dialogues[0].Fields) == 0
}

// ProjectsDocument reports whether projection can change this document.
// Callers use it to skip concrete parsing and layout resolution that
// DocumentViewFromConcrete would ignore anyway.
func (p Profile) ProjectsDocument(doc ass.Document) bool { return !p.skipsProjection(doc) }

// DocumentViewFromConcrete is the context-aware projection core. The caller
// supplies one concrete document and that profile's resolved layouts, so a
// request can share both across every consumer of one source version. A
// canceled context returns no partial view; the same inputs stay usable for a
// later request.
func (p Profile) DocumentViewFromConcrete(ctx context.Context, doc ass.Document, tree ass.ConcreteDocument, layouts []RecordLayout) (ass.Document, error) {
	return p.projectDocument(ctx, doc, tree, layouts)
}

func (p Profile) projectDocument(ctx context.Context, doc ass.Document, tree ass.ConcreteDocument, layouts []RecordLayout) (ass.Document, error) {
	if err := ctx.Err(); err != nil {
		return ass.Document{}, err
	}
	if p.skipsProjection(doc) {
		return doc, nil
	}
	stylesKnown, dialoguesKnown := true, true
	hasStyles, hasDialogues := false, false
	for _, row := range layouts {
		switch row.Kind {
		case "style":
			hasStyles = true
			stylesKnown = stylesKnown && row.Known
		case "dialogue":
			hasDialogues = true
			dialoguesKnown = dialoguesKnown && row.Known
		}
	}
	projectStyles, projectDialogues := hasStyles && stylesKnown, hasDialogues && dialoguesKnown
	if !projectStyles && !projectDialogues {
		return doc, nil
	}
	view := doc
	if projectDialogues {
		view.Dialogues = nil
		view.EventFormat = slices.Clone(assEventColumns)
	}
	if projectStyles {
		view.StyleFields = nil
	}
	sections := make([]string, len(tree.Lines))
	section := ""
	for i, line := range tree.Lines {
		if line.Header != nil {
			section = strings.ToLower(strings.TrimSpace(line.Header.RawName))
		}
		sections[i] = section
	}
	for _, row := range layouts {
		line := tree.Lines[row.Line-1]
		section := sections[row.Line-1]
		if projectStyles && row.Kind == "style" && section == "v4+ styles" {
			name := ""
			if len(row.Cells) != 0 {
				name = strings.TrimSpace(row.Cells[0].Raw)
			}
			for i, cell := range row.Cells {
				if !cell.Present {
					continue
				}
				value := strings.TrimSpace(cell.Raw)
				start := cell.Span.Start + len(cell.Raw) - len(strings.TrimLeft(cell.Raw, " \t"))
				view.StyleFields = append(view.StyleFields, ass.StyleField{
					StyleName: name, Name: row.Columns[i], Value: value, Line: row.Line,
					ValueStart: start, ValueEnd: start + len(value), ValueColumn: start - line.ContentSpan.Start + 1,
				})
			}
		}
		if row.Kind != "dialogue" || !projectDialogues || section != "events" {
			continue
		}
		dialogue := ass.Dialogue{Line: row.Line, LineStart: line.ContentSpan.Start}
		for i, cell := range row.Cells {
			name := row.Columns[i]
			dialogue.Fields = append(dialogue.Fields, ass.EventField{Name: name, Value: cell.Raw, Start: cell.Span.Start, End: cell.Span.End})
			dialogue.MissingFields = dialogue.MissingFields || !cell.Present
			if name == "text" {
				dialogue.Text, dialogue.TextStart = cell.Raw, cell.Span.Start
			}
			if name == "style" {
				dialogue.Style = strings.TrimSpace(cell.Raw)
				dialogue.StyleColumn = cell.Span.Start - line.ContentSpan.Start + len(cell.Raw) - len(strings.TrimLeft(cell.Raw, " \t")) + 1
			}
		}
		view.Dialogues = append(view.Dialogues, dialogue)
	}
	return view, nil
}
