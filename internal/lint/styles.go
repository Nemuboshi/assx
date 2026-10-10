package lint

import (
	"fmt"
	"math"
	"regexp"
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

// colourFields are the colour style fields consumed by ASS/SSA style rows.
var colourFields = map[string]bool{
	"primarycolour": true, "secondarycolour": true,
	"outlinecolour": true, "backcolour": true, "tertiarycolour": true,
}

// libass parse_int_header reads hex after an "&h" or "0x" prefix, else
// decimal; a value with no digit in the selected base silently becomes 0.
// xy-VSFilter GetInt throws on the same input and can abort the style line.
var (
	styleColourHexPrefixDigit = regexp.MustCompile(`(?i)^(?:&h|0x)[0-9a-f]`)
	styleColourDecimalDigit   = regexp.MustCompile(`^[+-]?[0-9]`)
)

func AnalyzeStyles(doc ass.Document) []Diagnostic {
	return proveSafeFixes(doc, analyzeStyles(doc), defaultFixTargets())
}

func analyzeStyles(doc ass.Document) []Diagnostic {
	var out []Diagnostic
	for _, field := range doc.StyleFields {
		fieldName := field.StyleName + "." + field.Name
		if integerStyleFields[field.Name] {
			value := strings.TrimSpace(field.Value)
			decoded := ass.DecodeInteger(value)
			replacement := strings.TrimSpace(value[:decoded.Consumed])
			if !strings.Contains(value, ".") || decoded.Consumed == 0 || value == replacement {
				continue
			}
			if _, err := strconv.ParseFloat(value, 64); err != nil {
				continue
			}
			if _, err := strconv.ParseInt(replacement, 10, 32); err == nil {
				out = append(out, styleDiagnostic(IssueStyleInteger, field, fieldName, replacement))
			} else {
				out = append(out, styleDiagnostic(IssueStyleInteger, field, fieldName, ""))
			}
			continue
		}
		if colourFields[field.Name] {
			value := strings.TrimSpace(field.Value)
			if styleColourMalformed(value) {
				diagnostic := styleDiagnostic(IssueMalformedStyleColour, field, fieldName, "")
				if value == "" {
					diagnostic.Detail = "The colour field is empty."
				} else {
					diagnostic.Detail = fmt.Sprintf("Value %q has no digit in its selected base.", value)
				}
				out = append(out, diagnostic)
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

func styleColourMalformed(value string) bool {
	if value == "" {
		return true
	}
	if strings.HasPrefix(strings.ToLower(value), "&h") || strings.HasPrefix(strings.ToLower(value), "0x") {
		return !styleColourHexPrefixDigit.MatchString(value)
	}
	return !styleColourDecimalDigit.MatchString(value)
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
