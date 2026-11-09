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

// Prepared holds one layout set and one view per requested profile. It is the
// shared view preparation for one source version.
type Prepared struct {
	Views   map[Profile]View
	Layouts map[Profile][]RecordLayout
}

// Prepare builds the default view once, then every requested renderer view
// from it. A canceled context returns no partial preparation.
func Prepare(ctx context.Context, source ass.Parsed, profiles []Profile) (Prepared, error) {
	return PrepareFrom(ctx, DefaultDocument(source), source, profiles)
}

// PrepareFrom builds every requested renderer view from an existing default
// view. Use it when the caller already has that view, for example after an
// edit.
func PrepareFrom(ctx context.Context, doc Document, source ass.Parsed, profiles []Profile) (Prepared, error) {
	prepared := Prepared{
		Views:   make(map[Profile]View, len(profiles)),
		Layouts: make(map[Profile][]RecordLayout, len(profiles)),
	}
	for _, profile := range profiles {
		if err := ctx.Err(); err != nil {
			return Prepared{}, err
		}
		layouts := profile.ResolveDocumentLayouts(source.Concrete)
		view, err := profile.viewFrom(ctx, doc, source.Concrete, layouts)
		if err != nil {
			return Prepared{}, err
		}
		prepared.Layouts[profile] = layouts
		prepared.Views[profile] = view
	}
	return prepared, nil
}

// View builds this renderer's view from one source parse.
func (p Profile) View(ctx context.Context, source ass.Parsed) (View, error) {
	return p.viewFrom(ctx, DefaultDocument(source), source.Concrete, p.ResolveDocumentLayouts(source.Concrete))
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
	var cursor documentCursor
	for i, line := range tree.Lines {
		cursor.enter(line)
		sections[i] = cursor.Section()
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
