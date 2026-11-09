package lint

import (
	"assx/internal/ass/renderer"
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

// Status is one renderer's outcome for one condition at one span.
type Status string

const (
	StatusAccepted      Status = "accepted"
	StatusRejected      Status = "rejected"
	StatusIgnored       Status = "ignored"
	StatusUnknown       Status = "unknown"
	StatusUnmodeled     Status = "unmodeled"
	StatusInferred      Status = "inferred"
	StatusNotComparable Status = "not-comparable"
)

// Role tells which selected renderer produced an observation.
type Role string

const (
	RolePrimary  Role = "primary"
	RoleCompared Role = "compared"
)

// Observation keeps one renderer's result next to the condition it observed.
type Observation struct {
	Renderer string `json:"renderer"`
	Role     Role   `json:"role"`
	Status   Status `json:"status"`
	Citation string `json:"citation,omitempty"`
}

type Diagnostic struct {
	File        string            `json:"file"`
	Condition   string            `json:"condition"`
	Scenario    string            `json:"scenario,omitempty"`
	Severity    Severity          `json:"severity"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Fix         string            `json:"suggested_fix"`
	FixSafety   FixSafety         `json:"fix_safety,omitempty"`
	FixProof    *FixProof         `json:"fix_proof,omitempty"`
	FixProofRef int               `json:"fix_proof_ref,omitempty"`
	Edits       []TextEdit        `json:"edits,omitempty"`
	Line        int               `json:"line"`
	Column      int               `json:"column"`
	Tag         string            `json:"tag,omitempty"`
	Field       string            `json:"field,omitempty"`
	Detail      string            `json:"detail,omitempty"`
	Renderers   []Observation     `json:"renderers,omitempty"`
	SourceSpan  *ass.ConcreteSpan `json:"source_span,omitempty"`
	Sources     []string          `json:"sources"`
	spanJSON    bool
}

func analyzeUnterminatedBlocks(dialogue renderer.Dialogue) []Diagnostic {
	text := dialogue.Text
	var diagnostics []Diagnostic
	for offset := 0; offset < len(text); {
		open := nextUnescapedOpenBrace(text, offset)
		if open < 0 {
			break
		}
		close := strings.IndexByte(text[open+1:], '}')
		if close < 0 {
			rule := RuleFor(IssueUnterminatedBlock)
			diagnostics = append(diagnostics, Diagnostic{
				Condition: rule.Condition, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
				Fix: rule.Fix, Line: dialogue.Line, Column: open + 1,
				Detail:  fmt.Sprintf("No closing brace follows the '{' at column %d; the rest of the line renders as literal text.", open+1),
				Sources: rule.Sources, SourceSpan: &ass.ConcreteSpan{Start: dialogue.TextStart + open, End: dialogue.TextStart + open + 1},
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

// RendererNames joins observations for display, primary first.
func (d Diagnostic) RendererNames() string {
	names := make([]string, 0, len(d.Renderers))
	for _, observation := range d.Renderers {
		names = append(names, observation.Renderer)
	}
	return strings.Join(names, " / ")
}

func (d Diagnostic) MarshalJSON() ([]byte, error) {
	type diagnosticJSON Diagnostic
	value := diagnosticJSON(d)
	if !d.spanJSON {
		value.SourceSpan = nil
	}
	return json.Marshal(value)
}

func (d *Diagnostic) UnmarshalJSON(data []byte) error {
	type diagnosticJSON Diagnostic
	var value diagnosticJSON
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*d = Diagnostic(value)
	d.spanJSON = d.SourceSpan != nil
	return nil
}

func Analyze(dialogue renderer.Dialogue) []Diagnostic {
	targets := DefaultAnalysisTargets()
	var findings []Diagnostic
	for _, profile := range targets.Profiles() {
		role := RoleCompared
		if profile == targets.Primary {
			role = RolePrimary
		}
		pass, _ := analyzeResolvedDialogue(dialogue, profile, nil, nil, nil)
		for i := range pass {
			if len(pass[i].Renderers) == 0 {
				pass[i].Renderers = []Observation{{
					Renderer: profile.Kind().String(), Role: role,
					Status: statusForCondition(pass[i].Condition),
				}}
			}
		}
		findings = append(findings, pass...)
	}
	findings = append(findings, portabilityDiagnostics(dialogue, targets)...)
	findings = append(findings, AnalyzeCompatibility(dialogue, targets.Profiles())...)
	findings = assemble(findings, targets.Primary)
	proofDialogue := dialogue
	if proofDialogue.Line == 0 {
		proofDialogue.Line = 1
	}
	doc := renderer.Document{Text: dialogue.Text, Dialogues: []renderer.Dialogue{proofDialogue}}
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
			diagnostics[i].FixProofRef = 0
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
	rule := RuleFor(id)
	span := ass.ConcreteSpan{Start: a.textStart + tag.Start, End: a.textStart + tag.End}
	a.diagnostics = append(a.diagnostics, Diagnostic{
		Condition: rule.Condition, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
		Fix: rule.Fix, FixSafety: rule.FixSafety, Line: a.line, Column: tag.Column, Tag: tag.Name,
		Detail: detail, Sources: rule.Sources, SourceSpan: &span,
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

func analyzeRepeatedOpenBraces(dialogue renderer.Dialogue) []Diagnostic {
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
			rule := RuleFor(IssueRepeatedOpenBrace)
			diagnostics = append(diagnostics, Diagnostic{
				Condition: rule.Condition, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
				Fix: rule.Fix, FixSafety: rule.FixSafety, Line: dialogue.Line, Column: open + 2,
				Edits:   []TextEdit{{Start: dialogue.TextStart + open + 1, End: dialogue.TextStart + runEnd}},
				Sources: rule.Sources, SourceSpan: &ass.ConcreteSpan{Start: dialogue.TextStart + open + 1, End: dialogue.TextStart + runEnd},
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
