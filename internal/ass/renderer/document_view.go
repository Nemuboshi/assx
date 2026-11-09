package renderer

import (
	"context"
	"slices"
	"strings"

	"assx/internal/ass"
)

// View is one renderer's document view. The embedded Document holds the
// fields that renderer reads. Profile and Layouts name the interpretation that
// produced it. A View is not the source document, and its Text spans can differ
// from another View of the same source.
type View struct {
	Document
	Profile Profile
	Layouts []RecordLayout
}

// View builds this renderer's view from one source parse.
func (p Profile) View(ctx context.Context, source ass.Parsed) (View, error) {
	return p.ViewFromConcrete(ctx, DefaultDocument(source), source, p.ResolveDocumentLayouts(source.Concrete))
}

// ViewOf builds this renderer's view for a document without a source parse.
// It parses the text again. Prefer View with a Parsed value on hot paths.
func (p Profile) ViewOf(ctx context.Context, doc Document) (View, error) {
	tree := ass.ParseConcreteDocument(doc.Text)
	return p.viewFrom(ctx, doc, tree, p.ResolveDocumentLayouts(tree))
}

// ViewFromConcrete builds this renderer's view from one default view, one
// source parse, and that profile's resolved layouts. The caller can share all
// three across every consumer of one source version, so the default view is
// built once per request. A canceled context returns no partial view; the same
// inputs stay usable for a later request.
func (p Profile) ViewFromConcrete(ctx context.Context, doc Document, source ass.Parsed, layouts []RecordLayout) (View, error) {
	return p.viewFrom(ctx, doc, source.Concrete, layouts)
}

func (p Profile) viewFrom(ctx context.Context, doc Document, tree ass.ConcreteDocument, layouts []RecordLayout) (View, error) {
	view, err := p.projectDocument(ctx, doc, tree, layouts)
	if err != nil {
		return View{}, err
	}
	return View{Document: view, Profile: p, Layouts: layouts}, nil
}

// skipsProjection reports the guard cases that need no concrete document.
func (p Profile) skipsProjection(doc Document) bool {
	return p.kind == Libass || len(doc.Lines) == 0 || len(doc.Dialogues) == 1 && len(doc.Dialogues[0].Fields) == 0
}

func (p Profile) projectDocument(ctx context.Context, doc Document, tree ass.ConcreteDocument, layouts []RecordLayout) (Document, error) {
	if err := ctx.Err(); err != nil {
		return Document{}, err
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
				view.StyleFields = append(view.StyleFields, StyleField{
					StyleName: name, Name: row.Columns[i], Value: value, Line: row.Line,
					ValueStart: start, ValueEnd: start + len(value), ValueColumn: start - line.ContentSpan.Start + 1,
				})
			}
		}
		if row.Kind != "dialogue" || !projectDialogues || section != "events" {
			continue
		}
		dialogue := Dialogue{Line: row.Line, LineStart: line.ContentSpan.Start}
		for i, cell := range row.Cells {
			name := row.Columns[i]
			dialogue.Fields = append(dialogue.Fields, EventField{Name: name, Value: cell.Raw, Start: cell.Span.Start, End: cell.Span.End})
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
