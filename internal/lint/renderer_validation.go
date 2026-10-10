package lint

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/semantic"
)

// AnalyzeForRenderer validates a dialogue under exactly one pinned renderer.
// Findings are renderer-local observations, never all-target SafeFix proofs.
func AnalyzeForRenderer(dialogue ass.Dialogue, profile renderer.Profile) []Diagnostic {
	findings, _ := analyzeResolvedDialogue(dialogue, profile, nil, nil)
	return scopeDiagnostics(findings, profile)
}

// AnalyzeDocumentForRenderer is the opt-in P06 entry point. Shared document
// checks remain active; only override naming, argument forms, state, drawing,
// karaoke, and reset-style references use the selected renderer. CLI selection
// and the default multi-renderer compatibility policy belong to P10.
func AnalyzeDocumentForRenderer(doc ass.Document, profile renderer.Profile) []Diagnostic {
	diagnostics := AnalyzeHeaders(doc)
	diagnostics = append(diagnostics, AnalyzeKeywords(doc)...)
	diagnostics = append(diagnostics, AnalyzeStyles(doc)...)
	diagnostics = append(diagnostics, AnalyzeEventFieldsForRenderer(doc, profile)...)
	diagnostics = append(diagnostics, analyzeUndefinedStyleReferencesForRenderer(doc, profile)...)

	styles := semantic.StyleStatesByName(doc.StyleFields)
	var redundant []Diagnostic
	for _, dialogue := range doc.Dialogues {
		collector := newStyleRunCollectorForRenderer(dialogue, styles)
		if collector != nil {
			collector.allowEdits = false
		}
		findings, style := analyzeResolvedDialogue(dialogue, profile, styles, collector)
		diagnostics = append(diagnostics, findings...)
		if style != nil {
			redundant = append(redundant, *style)
		}
	}
	diagnostics = append(diagnostics, redundant...)
	return scopeDiagnostics(diagnostics, profile)
}

// scopeDiagnostics annotates the explicit scope and enforces the P08 proof
// boundary for ALL diagnostics, including shared header and Style checks.
func scopeDiagnostics(diagnostics []Diagnostic, profile renderer.Profile) []Diagnostic {
	for i := range diagnostics {
		diagnostics[i].Renderer = profile.Kind().String()
		diagnostics[i].FixSafety = ""
		diagnostics[i].Edits = nil
	}
	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].Line == diagnostics[j].Line {
			return diagnostics[i].Column < diagnostics[j].Column
		}
		return diagnostics[i].Line < diagnostics[j].Line
	})
	// A source location may yield different rules, but repeating the exact
	// same observation offers no value (e.g. overlapping parsed views).
	type key struct {
		id, tag, field, detail string
		line, column           int
	}
	seen := make(map[key]bool, len(diagnostics))
	out := diagnostics[:0]
	for _, d := range diagnostics {
		k := key{d.ID, d.Tag, d.Field, d.Detail, d.Line, d.Column}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, d)
	}
	return out
}

func analyzeResolvedDialogue(dialogue ass.Dialogue, profile renderer.Profile, styles map[string]semantic.StyleState, collector *styleRunCollector) ([]Diagnostic, *Diagnostic) {
	tree := ass.ParseConcreteDialogue(dialogue.Text)
	analyzer := dialogueAnalyzer{line: dialogue.Line, textStart: dialogue.TextStart}
	for _, node := range tree.Nodes {
		if node.Block == nil {
			continue
		}
		if strings.TrimSpace(node.Block.Content) == "" {
			rule := Rules[IssueEmptyOverrideBlock]
			analyzer.diagnostics = append(analyzer.diagnostics, Diagnostic{
				ID: rule.ID, Severity: rule.Severity, Title: rule.Title,
				Description: rule.Description, Fix: rule.Fix,
				Line: dialogue.Line, Column: node.Block.Span.Start + 1,
				Sources: rule.Sources,
			})
			continue
		}
		for _, item := range node.Block.Items {
			if item.Expression == nil {
				if junk := meaningfulRawJunk(item.Raw); junk != "" {
					analyzer.diagnostics = append(analyzer.diagnostics,
						overrideJunkDiagnostic(dialogue, item.Span.Start+1, junk))
				}
			}
		}
	}
	profile.WalkDialogue(tree, func(result renderer.Result, nested bool) bool {
		tag := resolvedLintTag(result, nested)
		analyzer.consumeResolved(tag, result, profile)
		return true
	})
	analyzer.diagnostics = append(analyzer.diagnostics, analyzeRepeatedOpenBraces(dialogue)...)
	analyzer.diagnostics = append(analyzer.diagnostics, analyzeUnterminatedBlocks(dialogue)...)

	drawing := resolvedDrawingCollector{dialogue: dialogue}
	options := semantic.EvaluationOptions{
		Styles: styles, DialogueStyle: dialogue.Style,
		Observer: semantic.Observer{
			Tag: func(event semantic.TagEvent, state semantic.StateView) {
				if collector != nil {
					collector.onTag(event, state)
				}
			},
			Text: func(text string, start int, state semantic.StateView) {
				drawing.onText(text, start, state)
				if collector != nil {
					collector.onText(text, start, state)
				}
			},
		},
	}
	evaluation := semantic.EvaluateResolved(tree, profile, options)
	for _, effect := range evaluation.NoEffects {
		analyzer.add(IssueNoEffect, effect.Tag, noEffectDetail(effect))
	}
	analyzer.diagnostics = append(analyzer.diagnostics, drawing.diagnostics()...)
	if collector != nil {
		if finding, ok := collector.diagnostic(); ok {
			return analyzer.diagnostics, &finding
		}
	}
	return analyzer.diagnostics, nil
}

// Convert only a resolved command into the existing diagnostic/decoder view;
// never re-infer its identity or arguments via the global tag registry.
func resolvedLintTag(result renderer.Result, nested bool) ass.Tag {
	name := result.Name
	if name == "" {
		name = strings.TrimSpace(result.Head)
	}
	slashes := len(result.Raw) - len(strings.TrimLeft(result.Raw, "\\"))
	tag := ass.Tag{
		Name: name, Raw: result.Raw, Start: result.Source.Start,
		End: result.Source.End, Column: result.Source.Start + slashes + 1,
		Paren: result.Form == renderer.Paren, InTransition: nested,
		RepeatedSlashes: max(slashes-1, 0),
	}
	for _, arg := range result.Args {
		tag.Args = append(tag.Args, strings.TrimSpace(arg.Raw))
	}
	return tag
}

func (a *dialogueAnalyzer) consumeResolved(tag ass.Tag, result renderer.Result, profile renderer.Profile) {
	if tag.RepeatedSlashes != 0 {
		a.add(IssueRepeatedSlash, tag, fmt.Sprintf("Found %d extra backslash(es) before the tag.", tag.RepeatedSlashes))
		return
	}
	if !result.Closed {
		a.add(IssueRendererUnresolved, tag, "Unterminated parenthesized expression has no verified interpretation.")
		return
	}
	switch result.Status {
	case renderer.UnknownName:
		detail := "Unknown override tag for the selected renderer."
		if tag.Name == "N" || tag.Name == "n" || tag.Name == "h" {
			detail = "This text escape is only valid outside an override block."
		}
		a.add(IssueUnknownTag, tag, detail)
		return
	case renderer.Conditional:
		a.add(IssueRendererUnresolved, tag, "Command dispatch depends on unselected renderer build capabilities; validity cannot be established.")
		return
	case renderer.Disabled:
		a.add(IssueRendererUnresolved, tag, "Command is disabled by the selected renderer build.")
		return
	case renderer.Ignored:
		a.add(IssueRendererUnresolved, tag, "The renderer recognizes this prefix but ignores the full command.")
		return
	}
	if result.Signature == renderer.SignatureRejected {
		a.add(IssueArgumentCount, tag, fmt.Sprintf("Found %d arguments; expected %s for %s.", len(result.Args), verifiedCounts(profile, result.Name), profile.Kind()))
		return
	}
	if !result.HasPolicy {
		// Recognized extensions can still lack a verified state model.
		a.add(IssueRendererUnresolved, tag, "Renderer dispatch recognizes this command, but its effects are not yet modeled.")
		return
	}
	if result.Signature == renderer.SignatureInferred {
		a.add(IssueRendererUnresolved, tag, "This invocation is inferred from source evidence but is not verified for this renderer.")
		return
	}
	ir := ass.DecodeTagWithSpec(tag, result.Policy, true)
	// The frozen traditional syntax retains its source-checked structural
	// constraints even where the evidence matrix is not exhaustive.
	// Rect/vector clip forms are source-pinned in all three renderers.
	enforceCounts := result.Signature == renderer.SignatureUnknown &&
		(profile.Kind() != renderer.VSFilterMod || result.Name == "clip" || result.Name == "iclip")
	if result.Signature == renderer.SignatureUnknown &&
		!enforceCounts && result.Policy.Counts != nil && !ir.HasExpectedArity() {
		a.add(IssueRendererUnresolved, tag, "This argument shape is not verified for the selected renderer build.")
		return
	}
	a.validateIR(tag, ir, enforceCounts, false)
	if tail := ignoredTagTailIR(tag, ir); tail != "" {
		a.diagnostics = append(a.diagnostics, overrideJunkDiagnostic(
			ass.Dialogue{Line: a.line}, tag.Column, tail))
	}
}

func verifiedCounts(profile renderer.Profile, name string) string {
	var counts []int
	for _, s := range profile.Signatures(name) {
		if s.Availability != renderer.SignatureAvailable || s.Evidence != renderer.SignatureVerified {
			continue
		}
		found := false
		for _, c := range counts {
			if c == s.Count {
				found = true
				break
			}
		}
		if !found {
			counts = append(counts, s.Count)
		}
	}
	if len(counts) == 0 {
		return "a verified signature"
	}
	sort.Ints(counts)
	return countsText(counts)
}

// Renderer-specific drawing checks follow effective drawing state, including
// resets and unknown operations; they never trust a legacy raw \\p token.
type resolvedDrawingCollector struct {
	dialogue  ass.Dialogue
	fragments []ass.DrawingFragment
	findings  []Diagnostic
}

func (d *resolvedDrawingCollector) flush() {
	if len(d.fragments) == 0 {
		return
	}
	rule := Rules[IssueMalformedDrawing]
	for _, problem := range ass.ParseDrawing(d.fragments).Issues {
		d.findings = append(d.findings, Diagnostic{
			ID: rule.ID, Severity: rule.Severity, Title: rule.Title,
			Description: rule.Description, Fix: rule.Fix,
			Line: d.dialogue.Line, Column: problem.Start + 1,
			Detail: problem.Detail, Sources: rule.Sources,
		})
	}
	d.fragments = nil
}

func (d *resolvedDrawingCollector) onText(text string, start int, state semantic.StateView) {
	scale := state.Value("drawing_scale")
	value, err := strconv.ParseInt(scale.Value, 10, 32)
	if !scale.Known || err != nil || value <= 0 {
		d.flush()
		return
	}
	if text != "" {
		d.fragments = append(d.fragments, ass.DrawingFragment{
			Text: text, Start: start, End: start + len(text),
		})
	}
}

func (d *resolvedDrawingCollector) diagnostics() []Diagnostic {
	d.flush()
	return d.findings
}
