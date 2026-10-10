package lint

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/spec"
	"assx/internal/edit"
	"assx/internal/semantic"
)

type TextEdit = edit.TextEdit

type Diagnostic struct {
	File        string     `json:"file"`
	ID          string     `json:"id"`
	Severity    Severity   `json:"severity"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Fix         string     `json:"suggested_fix"`
	FixSafety   FixSafety  `json:"fix_safety,omitempty"`
	Edits       []TextEdit `json:"edits,omitempty"`
	Line        int        `json:"line"`
	Column      int        `json:"column"`
	Tag         string     `json:"tag,omitempty"`
	Field       string     `json:"field,omitempty"`
	Detail      string     `json:"detail,omitempty"`
	Sources     []string   `json:"sources"`
}

func analyzeUnterminatedBlocks(dialogue ass.Dialogue) []Diagnostic {
	text := dialogue.Text
	var diagnostics []Diagnostic
	for offset := 0; offset < len(text); {
		open := nextUnescapedOpenBrace(text, offset)
		if open < 0 {
			break
		}
		close := strings.IndexByte(text[open+1:], '}')
		if close < 0 {
			rule := Rules[IssueUnterminatedBlock]
			diagnostics = append(diagnostics, Diagnostic{
				ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
				Fix: rule.Fix, Line: dialogue.Line, Column: open + 1,
				Detail:  fmt.Sprintf("No closing brace follows the '{' at column %d; the rest of the line renders as literal text.", open+1),
				Sources: rule.Sources,
			})
			break
		}
		offset = open + 1 + close + 1
	}
	return diagnostics
}

type dialogueAnalyzer struct {
	line              int
	textStart         int
	diagnostics       []Diagnostic
	hasVSFilterModTag bool
}

func Analyze(dialogue ass.Dialogue) []Diagnostic {
	tree := dialogue.ParsedText()
	analyzer := dialogueAnalyzer{line: dialogue.Line, textStart: dialogue.TextStart}

	tree.WalkTokens(func(token ass.TokenView) bool {
		if token.HasTag {
			analyzer.consumeTag(token.Tag)
		}
		return true
	})

	analyzer.diagnostics = append(analyzer.diagnostics, analyzeOverrideSyntax(dialogue, tree)...)
	analyzer.diagnostics = append(analyzer.diagnostics, analyzeDrawings(dialogue, tree)...)
	analyzer.diagnostics = append(analyzer.diagnostics, analyzeUnterminatedBlocks(dialogue)...)

	for _, effect := range semantic.EvaluateDialogue(tree) {
		analyzer.add(IssueNoEffect, effect.Tag, noEffectDetail(effect))
		analyzer.diagnostics[len(analyzer.diagnostics)-1].Edits = []TextEdit{
			noEffectRemovalEdit(tree, effect.Tag, analyzer.textStart),
		}
	}

	analyzer.diagnostics = append(analyzer.diagnostics, analyzeRepeatedOpenBraces(dialogue)...)
	if analyzer.hasVSFilterModTag {
		for i := range analyzer.diagnostics {
			analyzer.diagnostics[i].FixSafety = ""
			analyzer.diagnostics[i].Edits = nil
		}
	}
	return analyzer.diagnostics
}

func noEffectDetail(effect semantic.NoEffect) string {
	switch effect.Reason {
	case semantic.SameValue:
		return "Assigns the value already active in every affected state slot."
	case semantic.FirstWinsIgnored:
		return fmt.Sprintf("Ignored because an earlier tag with index %d owns this first-wins slot.", effect.OwnerIndex)
	case semantic.OverwrittenBeforeUse:
		return "Overwritten before any dialogue text used it."
	case semantic.ResetBeforeUse:
		return "Reset before any dialogue text used it."
	case semantic.TransitionNoEffect:
		return "The transform cannot change tracked render state, and collision handling is already disabled."
	default:
		return "The override has no effect."
	}
}

func (a *dialogueAnalyzer) add(id string, tag ass.Tag, detail string) {
	rule := Rules[id]
	a.diagnostics = append(a.diagnostics, Diagnostic{
		ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
		Fix: rule.Fix, FixSafety: rule.FixSafety, Line: a.line, Column: tag.Column, Tag: tag.Name,
		Detail: detail, Sources: rule.Sources,
	})
}

func (a *dialogueAnalyzer) consumeTag(tag ass.Tag) {
	if tag.RepeatedSlashes > 0 {
		a.add(IssueRepeatedSlash, tag, fmt.Sprintf("Found %d extra backslash(es) before the tag.", tag.RepeatedSlashes))
		a.diagnostics[len(a.diagnostics)-1].Edits = []TextEdit{{
			Start: a.textStart + tag.Start,
			End:   a.textStart + tag.Start + tag.RepeatedSlashes,
		}}
		return
	}

	a.validate(tag)
	tagSpec, known := spec.TagSpecs[tag.Name]
	if !known {
		a.add(IssueUnknownTag, tag, "Unknown override tag.")
		return
	}
	if tag.Name == "N" || tag.Name == "n" || tag.Name == "h" {
		a.add(IssueUnknownTag, tag, "This text escape is only valid in dialogue text, outside an override block.")
		return
	}
	if tagSpec.VSFilterModOnly {
		a.hasVSFilterModTag = true
		a.add(IssueVSFilterModTag, tag, "This tag is specific to VSFilterMod and is not shared by libass and VSFilter. Automatic fixes are disabled for this dialogue.")
	}
}

func (a *dialogueAnalyzer) validate(tag ass.Tag) {
	ir := ass.DecodeTag(tag)
	if !ir.Known {
		return
	}
	tagSpec := ir.Spec
	if tag.Paren && len(tag.Args) == 0 {
		return // Empty parenthesized expressions are ignored by the renderers.
	}
	if tagSpec.Counts != nil && !ir.HasExpectedArity() {
		a.add(IssueArgumentCount, tag, fmt.Sprintf("Found %d arguments; expected %s.", len(tag.Args), countsText(tagSpec.Counts)))
		return
	}
	if len(tag.Args) == 0 {
		return
	}

	arg := tag.Args[0]
	decoded := ir.Argument(0)
	usable := decoded.Status == ass.ValueValid || decoded.Status == ass.ValueAmbiguous
	invalid := func(detail string) { a.add(IssueInvalidValue, tag, detail) }

	switch tagSpec.Value {
	case spec.IntegerValue:
		if !usable {
			invalid(fmt.Sprintf("Expected an integer, found %q.", arg))
		} else if (tagSpec.Min != 0 || tagSpec.Max != 0) && (decoded.Integer < int64(tagSpec.Min) || decoded.Integer > int64(tagSpec.Max)) {
			invalid(fmt.Sprintf("Value %d is outside the accepted range %d..%d.", decoded.Integer, tagSpec.Min, tagSpec.Max))
		} else if tag.Name == "a" && (decoded.Integer == 4 || decoded.Integer == 8) {
			a.add(IssueRendererDiff, tag, fmt.Sprintf("libass treats \\a%d as middle-center like \\a5; xy-VSFilter and VSFilterMod bit-map it to a different alignment.", decoded.Integer))
		}
	case spec.NumberValue:
		if decoded.Consumed == 0 {
			invalid(fmt.Sprintf("Expected a number, found %q.", arg))
		}
		if tag.Name == "blur" && usable && decoded.Number > 100 {
			a.add(IssueRendererDiff, tag, "Values above 100 are clamped by libass but have no matching upper clamp in VSFilter.")
		}
	case spec.BoldValue:
		value := decoded.Integer
		if !usable || (value != 0 && value != 1 && value < 100) {
			invalid(fmt.Sprintf("Bold value %q must be 0, 1, or at least 100.", arg))
		}
	case spec.FontNameValue:
		if tag.Paren && len(tag.Args) > 1 {
			a.add(IssueFontComma, tag, "A comma inside parenthesized \\fn syntax separates arguments.")
		}
	case spec.HexValue:
		if decoded.Consumed == 0 {
			invalid(fmt.Sprintf("Expected a hexadecimal value, found %q.", arg))
		} else if tag.Paren && strings.HasPrefix(strings.ToUpper(arg), "&H") {
			a.add(IssueRendererDiff, tag, "VSFilter and libass treat an &H prefix in parenthesized color/alpha arguments differently.")
		}
	case spec.NumberListValue:
		for i, value := range tag.Args {
			if ir.Argument(i).Consumed == 0 {
				invalid(fmt.Sprintf("Expected numeric arguments; found %q.", value))
				break
			}
		}
	case spec.RectValue:
		if len(tag.Args) == 4 {
			rendererDiff := false
			for i, value := range tag.Args {
				component := ir.Argument(i)
				if component.Consumed == 0 {
					invalid(fmt.Sprintf("Expected numeric rectangle arguments; found %q.", value))
					break
				}
				if usableFloat := component.Status == ass.ValueValid || component.Status == ass.ValueAmbiguous; usableFloat && math.Trunc(component.Number) != math.Trunc(component.Number+0.5) {
					rendererDiff = true
				}
			}
			if rendererDiff {
				a.add(IssueRendererDiff, tag, "Fractional rectangular clip coordinates can round differently: xy-VSFilter adds 0.5 before integer conversion, while libass and VSFilterMod consume integer values.")
			}
		}
	}
}

func analyzeRepeatedOpenBraces(dialogue ass.Dialogue) []Diagnostic {
	text := dialogue.Text
	var diagnostics []Diagnostic
	for offset := 0; offset < len(text); {
		open := nextUnescapedOpenBrace(text, offset)
		if open < 0 {
			break
		}
		close := strings.IndexByte(text[open+1:], '}')
		if close < 0 {
			break
		}
		close += open + 1
		runEnd := open + 1
		for runEnd < len(text) && text[runEnd] == '{' {
			runEnd++
		}
		if runEnd > open+1 {
			rule := Rules[IssueRepeatedOpenBrace]
			diagnostics = append(diagnostics, Diagnostic{
				ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
				Fix: rule.Fix, FixSafety: rule.FixSafety, Line: dialogue.Line, Column: open + 2,
				Edits:   []TextEdit{{Start: dialogue.TextStart + open + 1, End: dialogue.TextStart + runEnd}},
				Sources: rule.Sources,
			})
		}
		offset = close + 1
	}
	return diagnostics
}

func nextUnescapedOpenBrace(text string, start int) int {
	for i := start; i < len(text); i++ {
		if text[i] == '\\' && i+1 < len(text) && text[i+1] == '{' {
			i++
			continue
		}
		if text[i] == '{' {
			return i
		}
	}
	return -1
}

func countsText(values []int) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = strconv.Itoa(value)
	}
	return strings.Join(parts, " or ")
}
