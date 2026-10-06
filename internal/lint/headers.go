package lint

import (
	"fmt"
	"strconv"
	"strings"

	"assx/internal/ass"
)

func AnalyzeHeaders(doc ass.Document) []Diagnostic {
	var out []Diagnostic
	playResY, playResYValid := headerInt(doc, "playresy")
	matrix, matrixSet := doc.Headers["ycbcr matrix"]
	normalMatrix := strings.ToUpper(strings.TrimSpace(matrix.Value))

	if !matrixSet || normalMatrix == "" {
		edit := headerValueEdit(doc, matrix, matrixSet, "None", "YCbCr Matrix: None")
		out = append(out, headerDiagnostic(doc, IssueMatrixHeader, matrix, matrixSet,
			"No YCbCr Matrix is specified; renderer defaults can differ.", edit))
	} else if playResY > 576 && (normalMatrix == "TV.601" || normalMatrix == "PC.601") {
		replacement := "TV.709"
		if normalMatrix == "PC.601" {
			replacement = "PC.709"
		}
		edit := TextEdit{Start: matrix.ValueStart, End: matrix.ValueEnd, Replacement: replacement}
		out = append(out, headerDiagnostic(doc, IssueMatrixHeader, matrix, true,
			fmt.Sprintf("PlayResY is %d and YCbCr Matrix is %s.", playResY, matrix.Value), []TextEdit{edit}))
	}

	playResX, playResXValid := headerInt(doc, "playresx")
	layoutX, layoutXExists := doc.Headers["layoutresx"]
	layoutY, layoutYExists := doc.Headers["layoutresy"]
	layoutXValid := layoutXExists && positiveInt(layoutX.Value)
	layoutYValid := layoutYExists && positiveInt(layoutY.Value)
	if !layoutXValid || !layoutYValid {
		var edits []TextEdit
		var lines []string
		hasValidPlayRes := playResXValid && playResYValid
		if hasValidPlayRes {
			if !layoutXValid {
				if layoutXExists {
					edits = append(edits, TextEdit{Start: layoutX.ValueStart, End: layoutX.ValueEnd, Replacement: strconv.Itoa(playResX)})
				} else {
					lines = append(lines, "LayoutResX: "+strconv.Itoa(playResX))
				}
			}
			if !layoutYValid {
				if layoutYExists {
					edits = append(edits, TextEdit{Start: layoutY.ValueStart, End: layoutY.ValueEnd, Replacement: strconv.Itoa(playResY)})
				} else {
					lines = append(lines, "LayoutResY: "+strconv.Itoa(playResY))
				}
			}
			if len(lines) > 0 {
				edits = append(edits, headerInsert(doc, lines))
			}
		}
		detail := "LayoutResX and LayoutResY must both be positive for libass to use the declared layout resolution."
		if !playResXValid || !playResYValid {
			detail += " Set valid PlayResX and PlayResY values before applying the suggested fix."
		}
		out = append(out, headerDiagnostic(doc, IssueLayoutRes, layoutX, layoutXExists, detail, edits))
	}
	return out
}

func headerDiagnostic(doc ass.Document, id string, field ass.HeaderField, exists bool, detail string, edits []TextEdit) Diagnostic {
	rule := Rules[id]
	line := doc.ScriptInfoLine
	if line == 0 {
		line = 1
	}
	column := 1
	if exists {
		line = field.Line
	}
	return Diagnostic{
		ID: rule.ID, Severity: rule.Severity, FixSafety: rule.FixSafety,
		Title: rule.Title, Description: rule.Description, Fix: rule.Fix,
		Line: line, Column: column, Detail: detail, Sources: rule.Sources, Edits: edits,
	}
}

func headerValueEdit(doc ass.Document, field ass.HeaderField, exists bool, value, insertion string) []TextEdit {
	if exists {
		return []TextEdit{{Start: field.ValueStart, End: field.ValueEnd, Replacement: value}}
	}
	return []TextEdit{headerInsert(doc, []string{insertion})}
}

func headerInsert(doc ass.Document, lines []string) TextEdit {
	joined := strings.Join(lines, doc.Newline) + doc.Newline
	if doc.ScriptInfoLine == 0 {
		joined = "[Script Info]" + doc.Newline + joined + doc.Newline
		return TextEdit{Start: 0, End: 0, Replacement: joined}
	}
	at := doc.ScriptInfoInsert
	if at > 0 && doc.Text[at-1] != '\n' {
		joined = doc.Newline + joined
	}
	return TextEdit{Start: at, End: at, Replacement: joined}
}

func headerInt(doc ass.Document, name string) (int, bool) {
	field, exists := doc.Headers[name]
	if !exists {
		return 0, false
	}
	value, err := strconv.Atoi(strings.TrimSpace(field.Value))
	return value, err == nil && value > 0
}

func positiveInt(value string) bool {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	return err == nil && parsed > 0
}
