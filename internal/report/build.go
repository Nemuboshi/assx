package report

import (
	"strings"

	"assx/internal/ass"
	"assx/internal/lint"
)

// Build assembles the presenter view from final diagnostics. It preserves the
// public diagnostic order and normalizes positions with document byte spans.
func Build(path string, diagnostics []lint.Diagnostic, document ass.Document) Summary {
	var summary Summary
	summary.Total = len(diagnostics)
	group := Group{File: path}
	textStarts := make(map[int]int, len(document.Dialogues))
	tagSpans := make(map[tagKey]sourcePosition)
	fields := make(map[fieldKey]sourcePosition)
	lineOffsets := make(map[int]int, len(document.Lines))
	for _, line := range document.Lines {
		lineOffsets[line.Line] = line.Offset
	}
	for _, dialogue := range document.Dialogues {
		textOffset := dialogue.TextStart - dialogue.LineStart
		textStarts[dialogue.Line] = textOffset
		dialogue.ParsedText().WalkTokens(func(token ass.TokenView) bool {
			if !token.HasTag {
				return true
			}
			name := token.Tag.Name
			span := token.Tag.End - token.Tag.SlashStart
			column := textOffset + token.Tag.SlashStart + 1
			if token.Tag.RepeatedSlashes > 0 {
				if name == "" {
					span = token.Tag.End - token.Tag.Start
					column = textOffset + token.Tag.Start + 1
				} else {
					span = token.Tag.RepeatedSlashes
				}
			}
			tagSpans[tagKey{dialogue.Line, token.Tag.Column, token.Tag.Name}] = sourcePosition{
				column: column,
				width:  span,
			}
			return true
		})
		for _, field := range dialogue.Fields {
			fields[fieldKey{dialogue.Line, strings.ToLower(field.Name)}] = sourcePosition{
				column: field.Start - dialogue.LineStart + 1,
				width:  field.End - field.Start,
			}
		}
	}
	for _, field := range document.StyleFields {
		if lineOffset, ok := lineOffsets[field.Line]; ok {
			fields[fieldKey{field.Line, strings.ToLower(field.Name)}] = sourcePosition{
				column: field.ValueStart - lineOffset + 1,
				width:  field.ValueEnd - field.ValueStart,
			}
		}
	}
	for i := range diagnostics {
		d := &diagnostics[i]
		position := locate(d, textStarts, fields, tagSpans)
		view := Diagnostic{
			Line:         d.Line,
			Column:       d.Column,
			SourceColumn: position.column,
			SourceWidth:  position.width,
			ID:           d.ID,
			Severity:     d.Severity,
			Title:        d.Title,
			Description:  d.Description,
			Detail:       d.Detail,
			Fix:          d.Fix,
			Tag:          d.Tag,
			Field:        d.Field,
			FixSafety:    d.FixSafety,
			Edits:        append([]lint.TextEdit(nil), d.Edits...),
			Sources:      append([]string(nil), d.Sources...),
		}
		view.Outcome = outcomeOf(d)
		switch d.Severity {
		case lint.Error:
			group.Errors++
		case lint.Warning:
			group.Warnings++
		case lint.Suggestion:
			group.Suggestions++
		}
		switch view.Outcome {
		case FixSafe:
			group.FixAvailable.Safe++
			summary.Remaining.Safe++
		case FixUnsafe:
			group.FixAvailable.Unsafe++
			summary.Remaining.Unsafe++
		case FixNone:
			group.FixAvailable.Unfixable++
			summary.Remaining.Unfixable++
		}
		group.Diagnostics = append(group.Diagnostics, view)
	}
	summary.Groups = append(summary.Groups, group)
	return summary
}

type fieldKey struct {
	line int
	name string
}

type tagKey struct {
	line   int
	column int
	name   string
}

type sourcePosition struct {
	column int
	width  int
}

func locate(d *lint.Diagnostic, textStarts map[int]int, fields map[fieldKey]sourcePosition, tagSpans map[tagKey]sourcePosition) sourcePosition {
	if d.Field != "" {
		if position, ok := fields[fieldKey{d.Line, strings.ToLower(d.Field)}]; ok {
			return position
		}
		return sourcePosition{column: d.Column, width: 1}
	}
	position := sourcePosition{column: d.Column, width: 1}
	if offset, ok := textStarts[d.Line]; ok {
		position.column += offset
	}
	if d.Tag != "" {
		if tagPosition, ok := tagSpans[tagKey{d.Line, d.Column, d.Tag}]; ok {
			return tagPosition
		}
	}
	return position
}

// outcomeOf derives the fix badge only from concrete edits and validated
// per-diagnostic safety metadata.
func outcomeOf(d *lint.Diagnostic) FixOutcome {
	if len(d.Edits) == 0 {
		return FixNone
	}
	switch d.FixSafety {
	case lint.SafeFix:
		return FixSafe
	case lint.UnsafeFix:
		return FixUnsafe
	default:
		return FixNone
	}
}
