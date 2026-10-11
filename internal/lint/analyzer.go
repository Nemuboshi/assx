package lint

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/spec"
	"assx/internal/edit"
	"assx/internal/semantic"
)

type TextEdit = edit.TextEdit

type Diagnostic struct {
	File        string            `json:"file"`
	ID          string            `json:"id"`
	Severity    Severity          `json:"severity"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Fix         string            `json:"suggested_fix"`
	FixSafety   FixSafety         `json:"fix_safety,omitempty"`
	FixProof    *FixProof         `json:"fix_proof,omitempty"`
	FixProofRef string            `json:"fix_proof_ref,omitempty"`
	Edits       []TextEdit        `json:"edits,omitempty"`
	Line        int               `json:"line"`
	Column      int               `json:"column"`
	Tag         string            `json:"tag,omitempty"`
	Field       string            `json:"field,omitempty"`
	Detail      string            `json:"detail,omitempty"`
	Renderer    string            `json:"renderer,omitempty"`
	SourceSpan  *ass.ConcreteSpan `json:"source_span,omitempty"`
	Sources     []string          `json:"sources"`
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
	line        int
	textStart   int
	diagnostics []Diagnostic
}

func (d Diagnostic) MarshalJSON() ([]byte, error) {
	type diagnosticJSON Diagnostic
	value := diagnosticJSON(d)
	if d.FixProofRef != "" {
		value.FixProof = nil
	}
	return json.Marshal(value)
}

func Analyze(dialogue ass.Dialogue) []Diagnostic {
	targets := DefaultAnalysisTargets()
	findings, _ := analyzeResolvedDialogue(dialogue, targets.Primary, nil, nil, nil)
	findings = append(findings, portabilityDiagnostics(dialogue, targets)...)
	findings = append(findings, AnalyzeCompatibility(dialogue, targets.Profiles())...)
	for i := range findings {
		if findings[i].Renderer == "" {
			findings[i].Renderer = ProfileScope(targets.Primary)
		}
	}
	proofDialogue := dialogue
	if proofDialogue.Line == 0 {
		proofDialogue.Line = 1
	}
	doc := ass.Document{Text: dialogue.Text, Dialogues: []ass.Dialogue{proofDialogue}}
	findings = proveSafeFixes(doc, findings, targets.Profiles())
	if dialogue.TextStart != 0 {
		return withoutSafeFixes(findings)
	}
	return findings
}

func withoutSafeFixes(diagnostics []Diagnostic) []Diagnostic {
	for i := range diagnostics {
		if diagnostics[i].FixSafety == SafeFix {
			diagnostics[i].FixSafety = ""
			diagnostics[i].Edits = nil
			diagnostics[i].FixProof = nil
			diagnostics[i].FixProofRef = ""
		}
	}
	return diagnostics
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

// validateIR checks values under the resolved policy. Differential checks
// belong only to the semantic compatibility engine.
func (a *dialogueAnalyzer) validateIR(tag ass.Tag, ir ass.TagIR, policyArity bool) {
	if !ir.Known {
		return
	}
	tagSpec := ir.Spec
	if tag.Paren && len(tag.Args) == 0 {
		return // Empty parenthesized expressions are ignored by the renderers.
	}
	if policyArity && tagSpec.Counts != nil && !ir.HasExpectedArity() {
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

		}
	case spec.NumberValue:
		if decoded.Consumed == 0 {
			invalid(fmt.Sprintf("Expected a number, found %q.", arg))
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
			for i, value := range tag.Args {
				component := ir.Argument(i)
				if component.Consumed == 0 {
					invalid(fmt.Sprintf("Expected numeric rectangle arguments; found %q.", value))
					break
				}
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
