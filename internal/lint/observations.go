package lint

import (
	"context"
	"slices"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/semantic"
)

type resolutionKey struct {
	start, end int
}

// observationStatus is the compared renderer's status for one resolution.
func observationStatus(result renderer.Result) Status {
	switch {
	case !result.Closed, result.Status == renderer.UnknownName:
		return StatusUnknown
	case result.Status == renderer.Disabled:
		return StatusUnmodeled
	case result.Status == renderer.Ignored:
		return StatusIgnored
	case result.Signature == renderer.SignatureRejected:
		return StatusRejected
	case !result.HasPolicy:
		return StatusUnmodeled
	case result.Signature == renderer.SignatureInferred:
		return StatusInferred
	}
	return StatusAccepted
}

// resolutionObservable reports whether a condition's compared status comes
// from the renderer's resolution rather than from a lint-side check.
func resolutionObservable(condition string) bool {
	switch condition {
	case IssueArgumentCount, IssueUnknownTag, IssueRendererUnresolved:
		return true
	}
	return false
}

// deepCheckable reports whether a condition depends on lint-side value
// decoding. Its compared verdict comes from the same rule cascade.
func deepCheckable(condition string) bool {
	switch condition {
	case IssueInvalidValue, IssueFontComma:
		return true
	}
	return false
}

// sameResolution reports whether two profiles decoded the same expression the
// same way. Equal input and equal policy imply an equal lint verdict.
func sameResolution(a, b renderer.Result) bool {
	if a.Status != b.Status || a.Signature != b.Signature || a.Closed != b.Closed ||
		a.HasPolicy != b.HasPolicy || a.Shadowed != b.Shadowed || a.Form != b.Form ||
		a.Name != b.Name || len(a.Args) != len(b.Args) ||
		a.Policy.Value != b.Policy.Value || a.Policy.Min != b.Policy.Min || a.Policy.Max != b.Policy.Max ||
		len(a.Policy.Counts) != len(b.Policy.Counts) {
		return false
	}
	for i := range a.Args {
		if a.Args[i].Raw != b.Args[i].Raw {
			return false
		}
	}
	for i := range a.Policy.Counts {
		if a.Policy.Counts[i] != b.Policy.Counts[i] {
			return false
		}
	}
	return true
}

// lintResolved runs the tag cascade for one compared resolution. The helper
// keeps creation and observation filling on one rule path.
func lintResolved(tag ass.Tag, result renderer.Result, profile renderer.Profile, line, textStart int) []Diagnostic {
	analyzer := dialogueAnalyzer{line: line, textStart: textStart}
	analyzer.consumeResolved(tag, result, profile)
	return analyzer.diagnostics
}

// resolvedView returns the prepared view for one profile, or builds it when
// the caller has no shared preparation.
func resolvedView(ctx context.Context, source ass.Parsed, profile renderer.Profile, shared *DocumentAnalysis) (renderer.View, bool) {
	if shared != nil {
		if view, ok := shared.viewFor(source, profile); ok {
			return view, true
		}
	}
	view, err := profile.View(ctx, source)
	return view, err == nil
}

// primaryResolutions records how the primary renderer decoded every
// expression. Compared checks run only where the compared decode differs.
func primaryResolutions(ctx context.Context, source ass.Parsed, targets AnalysisTargets, shared *DocumentAnalysis) (map[resolutionKey]renderer.Result, renderer.View) {
	results := make(map[resolutionKey]renderer.Result)
	view, ok := resolvedView(ctx, source, targets.Primary, shared)
	if !ok {
		return results, view
	}
	for _, dialogue := range view.Dialogues {
		if ctx.Err() != nil {
			return results, view
		}
		var prepared *semantic.PreparedDialogue
		if shared != nil {
			prepared = shared.preparations.PrepareText(targets.Primary, dialogue.Text)
		} else {
			prepared, _ = semantic.PrepareDialogue(ctx, ass.ParseConcreteDialogue(dialogue.Text), targets.Primary)
		}
		if prepared == nil {
			continue
		}
		start := dialogue.TextStart
		prepared.EachOperation(func(result renderer.Result, nested bool) bool {
			span := result.Source.AtDocumentOffset(start)
			results[resolutionKey{span.Start, span.End}] = result
			return true
		})
	}
	return results, view
}

// applyResolutions fills compared observations. A compared expression whose
// decode equals the primary's copies the primary verdict; a different decode
// runs the rule cascade once and can create a finding.
func applyResolutions(ctx context.Context, source ass.Parsed, targets AnalysisTargets, shared *DocumentAnalysis, findings []Diagnostic) []Diagnostic {
	bySpan := make(map[resolutionKey][]int, len(findings))
	for i := range findings {
		if span := findings[i].SourceSpan; span != nil {
			key := resolutionKey{span.Start, span.End}
			bySpan[key] = append(bySpan[key], i)
		}
	}
	primary, primaryView := primaryResolutions(ctx, source, targets, shared)
	primaryText := make(map[int]string, len(primaryView.Dialogues))
	for _, dialogue := range primaryView.Dialogues {
		primaryText[dialogue.Line] = dialogue.Text
	}
	created := make(map[resolutionKey]map[string]int)
	var added []Diagnostic
	attach := func(d Diagnostic, observation Observation) {
		key := resolutionKey{d.SourceSpan.Start, d.SourceSpan.End}
		for _, at := range bySpan[key] {
			if findings[at].Condition == d.Condition {
				// The finding loop adds this observation once.
				return
			}
		}
		for existing := range created[key] {
			if existing == d.Condition {
				return
			}
		}
		rule := RuleFor(d.Condition)
		if created[key] == nil {
			created[key] = make(map[string]int)
		}
		created[key][d.Condition] = len(added)
		added = append(added, Diagnostic{
			Condition: d.Condition, Title: rule.Title, Description: rule.Description,
			Fix: rule.Fix, FixSafety: rule.FixSafety, Sources: rule.Sources,
			Line: d.Line, Column: d.Column, Tag: d.Tag,
			Renderers:  []Observation{observation},
			SourceSpan: &ass.ConcreteSpan{Start: d.SourceSpan.Start, End: d.SourceSpan.End},
		})
	}
	requests := make(map[int][]evaluationRequest)
	for _, profile := range targets.Profiles() {
		if profile == targets.Primary {
			continue
		}
		view, ok := resolvedView(ctx, source, profile, shared)
		if !ok {
			return findings
		}
		styles := semantic.StyleStatesByName(view.StyleFields)
		for _, dialogue := range view.Dialogues {
			if err := ctx.Err(); err != nil {
				return findings
			}
			// Different Text spans have no shared tag stream. The layout
			// finding reports that condition instead of a tag comparison.
			if text, ok := primaryText[dialogue.Line]; !ok || text != dialogue.Text {
				continue
			}
			var prepared *semantic.PreparedDialogue
			if shared != nil {
				prepared = shared.preparations.PrepareText(profile, dialogue.Text)
			} else {
				prepared, _ = semantic.PrepareDialogue(ctx, ass.ParseConcreteDialogue(dialogue.Text), profile)
			}
			if prepared == nil {
				continue
			}
			start := dialogue.TextStart
			needsEvaluation := false
			prepared.EachOperation(func(result renderer.Result, nested bool) bool {
				if result.Status == renderer.Matched && evaluatedTag(result.Name) {
					needsEvaluation = true
				}
				span := result.Source.AtDocumentOffset(start)
				key := resolutionKey{span.Start, span.End}
				observation := Observation{
					Renderer: profile.Kind().String(),
					Role:     RoleCompared,
					Status:   observationStatus(result),
				}
				primaryResult, hasPrimary := primary[key]
				equal := hasPrimary && sameResolution(primaryResult, result)
				var scratch []Diagnostic
				if !equal {
					tag := resolvedLintTag(result, nested)
					scratch = lintResolved(tag, result, profile, dialogue.Line, start)
				}
				scratchAt := make(map[string]Status, len(scratch))
				for _, diagnostic := range scratch {
					if diagnostic.SourceSpan == nil {
						continue
					}
					if diagnostic.SourceSpan.Start == span.Start && diagnostic.SourceSpan.End == span.End {
						scratchAt[diagnostic.Condition] = statusForCondition(diagnostic.Condition)
					}
					attach(diagnostic, Observation{
						Renderer: profile.Kind().String(), Role: RoleCompared, Status: statusForCondition(diagnostic.Condition),
					})
				}
				for _, at := range bySpan[key] {
					finding := &findings[at]
					switch {
					case scratchAt[finding.Condition] != "":
						finding.Renderers = append(finding.Renderers, Observation{
							Renderer: profile.Kind().String(), Role: RoleCompared,
							Status: scratchAt[finding.Condition],
						})
					case resolutionObservable(finding.Condition):
						if observation.Status != StatusAccepted && finding.Condition != observationCondition(result) {
							continue
						}
						finding.Renderers = append(finding.Renderers, observation)
					case deepCheckable(finding.Condition):
						status := StatusAccepted
						if equal {
							status = statusForCondition(finding.Condition)
						} else if len(scratchAt) != 0 {
							status = StatusUnknown
						}
						finding.Renderers = append(finding.Renderers, Observation{
							Renderer: profile.Kind().String(), Role: RoleCompared, Status: status,
						})
					}
				}
				return true
			})
			if needsEvaluation {
				requests[dialogue.Line] = append(requests[dialogue.Line], evaluationRequest{
					dialogue: dialogue, profile: profile, styles: styles, prepared: prepared,
				})
			}
		}
	}
	lines := make([]int, 0, len(requests))
	for line := range requests {
		lines = append(lines, line)
	}
	slices.Sort(lines)
	for _, line := range lines {
		findings = append(findings, comparedEvaluationSet(requests[line])...)
	}
	return append(findings, added...)
}

// observationCondition maps a resolution to the condition it would report.
func observationCondition(result renderer.Result) string {
	switch observationStatus(result) {
	case StatusUnknown:
		if result.Closed {
			return IssueUnknownTag
		}
		return IssueRendererUnresolved
	case StatusIgnored, StatusInferred:
		return IssueRendererUnresolved
	case StatusRejected:
		return IssueArgumentCount
	case StatusUnmodeled:
		return IssueRendererUnresolved
	}
	return ""
}

// evaluatedTag reports whether a tag can change karaoke or drawing state.
func evaluatedTag(name string) bool {
	switch name {
	case "k", "K", "kf", "ko", "kt", "p":
		return true
	}
	return false
}

type evaluationRequest struct {
	dialogue ass.Dialogue
	profile  renderer.Profile
	styles   map[string]semantic.StyleState
	prepared *semantic.PreparedDialogue
}

// comparedEvaluationSet folds one neutral program per projected dialogue text
// for every compared renderer that shares it, then converts each state into
// observations. A custom event Format can select a different text span for
// each renderer, so requests from different texts never share one fold.
func comparedEvaluationSet(requests []evaluationRequest) []Diagnostic {
	prepared := make([]*semantic.PreparedDialogue, len(requests))
	options := make([]semantic.EvaluationOptions, len(requests))
	drawings := make([]resolvedDrawingCollector, len(requests))
	karaokes := make([]resolvedKaraokeCursor, len(requests))
	for i := range requests {
		prepared[i] = requests[i].prepared
		drawings[i] = resolvedDrawingCollector{dialogue: requests[i].dialogue}
		options[i] = semantic.EvaluationOptions{
			Styles: requests[i].styles, DialogueStyle: requests[i].dialogue.Style,
			Observer: semantic.Observer{
				Tag: func(event semantic.TagEvent, state semantic.StateView) {
					karaokes[i].onTag(event, state)
				},
				Text: func(text string, start int, state semantic.StateView) {
					karaokes[i].onText(text, start, state)
					drawings[i].onText(text, start, state)
				},
			},
		}
	}
	semantic.EvaluatePreparedSet(prepared, options)
	var findings []Diagnostic
	for i := range requests {
		current := drawings[i].diagnostics()
		if !requests[i].dialogue.MissingFields {
			current = append(current, analyzeKaraokeWithCursor(requests[i].dialogue, karaokes[i].duration)...)
		}
		stampPass(current, requests[i].profile, RoleCompared)
		findings = append(findings, current...)
	}
	return findings
}
