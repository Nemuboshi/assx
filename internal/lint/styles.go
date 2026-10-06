package lint

import (
	"math"
	"strconv"
	"strings"

	"assx/internal/ass"
)

var integerStyleFields = map[string]bool{
	"bold": true, "italic": true, "underline": true, "strikeout": true,
	"borderstyle": true, "alignment": true,
	"marginl": true, "marginr": true,
	"marginv": true, "encoding": true, "alphalevel": true,
}

var floatStyleFields = map[string]bool{
	"fontsize": true, "scalex": true, "scaley": true, "spacing": true,
	"angle": true, "outline": true, "shadow": true,
}

func AnalyzeStyles(doc ass.Document) []Diagnostic {
	var out []Diagnostic
	for _, field := range doc.StyleFields {
		fieldName := field.StyleName + "." + field.Name
		if integerStyleFields[field.Name] {
			value := strings.TrimSpace(field.Value)
			match := integerPrefix.FindStringSubmatch(value)
			if !strings.Contains(value, ".") || len(match) == 0 || value == match[1] {
				continue
			}
			if _, err := strconv.ParseFloat(value, 64); err != nil {
				continue
			}
			replacement := match[1]
			if _, err := strconv.ParseInt(replacement, 10, 32); err == nil {
				out = append(out, styleDiagnostic(IssueStyleInteger, field, fieldName, replacement))
			} else {
				out = append(out, styleDiagnostic(IssueStyleInteger, field, fieldName, ""))
			}
			continue
		}
		if !floatStyleFields[field.Name] {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(field.Value), 64)
		if err != nil || significantDigits(field.Value) <= 7 {
			continue
		}
		f32 := float32(value)
		if math.IsInf(float64(f32), 0) {
			out = append(out, styleDiagnostic(IssueStyleFloat, field, fieldName, ""))
			continue
		}
		rounded := strconv.FormatFloat(float64(f32), 'f', -1, 32)
		roundedValue, err := strconv.ParseFloat(rounded, 64)
		if err == nil && roundedValue != value {
			out = append(out, styleDiagnostic(IssueStyleFloat, field, fieldName, rounded))
		}
	}
	return out
}

func styleDiagnostic(id string, field ass.StyleField, name, replacement string) Diagnostic {
	rule := Rules[id]
	diagnostic := Diagnostic{
		ID: rule.ID, Severity: rule.Severity, FixSafety: rule.FixSafety,
		Title: rule.Title, Description: rule.Description, Fix: rule.Fix,
		Line: field.Line, Column: field.ValueColumn, Field: name, Sources: rule.Sources,
	}
	if replacement != "" {
		diagnostic.Edits = []TextEdit{{Start: field.ValueStart, End: field.ValueEnd, Replacement: replacement}}
	}
	return diagnostic
}

func significantDigits(value string) int {
	mantissa := strings.SplitN(strings.ToLower(strings.TrimSpace(value)), "e", 2)[0]
	mantissa = strings.TrimLeft(mantissa, "+-")
	mantissa = strings.ReplaceAll(mantissa, ".", "")
	mantissa = strings.TrimLeft(mantissa, "0")
	mantissa = strings.TrimRight(mantissa, "0")
	return len(mantissa)
}
