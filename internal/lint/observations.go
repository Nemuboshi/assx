package lint

import (
	"context"
	"fmt"
	"strings"

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

// conditionFromResult names the condition a resolution implies. An accepted
// resolution has no condition: it only becomes an observation on a finding.
func conditionFromResult(result renderer.Result, profile renderer.Profile, tag string) (string, Status, string) {
	switch status := observationStatus(result); {
	case status == StatusAccepted:
		return "", status, ""
	case status == StatusUnknown && result.Closed:
		detail := "Unknown override tag for the selected renderer."
		if tag == "N" || tag == "n" || tag == "h" {
			detail = "This text escape is only valid outside an override block."
		}
		return IssueUnknownTag, status, detail
	case status == StatusUnknown:
		return IssueRendererUnresolved, status, "Unterminated parenthesized expression has no verified interpretation."
	case status == StatusIgnored:
		return IssueRendererUnresolved, status, "The renderer recognizes this prefix but ignores the full command."
	case status == StatusRejected:
		return IssueArgumentCount, status, fmt.Sprintf("Found %d arguments; expected %s for %s.",
			len(result.Args), verifiedCounts(profile, result.Name), profile.Kind())
	case status == StatusInferred:
		return IssueRendererUnresolved, status, "This invocation is inferred from source evidence but is not verified for this renderer."
	case result.Status == renderer.Disabled:
		return IssueRendererUnresolved, status, "Command is disabled by the selected renderer build."
	default:
		return IssueRendererUnresolved, StatusUnmodeled, "Renderer dispatch recognizes this command, but its effects are not yet modeled."
	}
}

// applyResolutions reads each compared renderer's resolved operations and
// turns them into observations on the findings at the same source span. It
// replaces a second rule pass: a compared renderer resolves the expression once
// and its status becomes the observation, instead of re-running every check.
// When the compared renderer rejects a span that carries no finding yet, it
// creates one; assembly fills the primary observation.
func applyResolutions(ctx context.Context, doc ass.Document, targets AnalysisTargets, shared *DocumentAnalysis, findings []Diagnostic) []Diagnostic {
	bySpan := make(map[resolutionKey][]int, len(findings))
	for i := range findings {
		if span := findings[i].SourceSpan; span != nil {
			key := resolutionKey{span.Start, span.End}
			bySpan[key] = append(bySpan[key], i)
		}
	}
	type createdKey struct {
		span      resolutionKey
		condition string
	}
	created := make(map[createdKey]int)
	var added []Diagnostic
	for _, profile := range targets.Profiles() {
		if profile == targets.Primary {
			continue
		}
		view := profile.DocumentView(doc)
		if shared != nil {
			if preparedView, ok := shared.viewFor(doc, profile); ok {
				view = preparedView
			}
		}
		for _, dialogue := range view.Dialogues {
			if err := ctx.Err(); err != nil {
				return findings
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
			prepared.EachOperation(func(result renderer.Result, nested bool) bool {
				span := result.Source.AtDocumentOffset(start)
				key := resolutionKey{span.Start, span.End}
				status := observationStatus(result)
				observation := Observation{
					Renderer: profile.Kind().String(),
					Role:     RoleCompared,
					Status:   status,
				}
				name := result.Name
				if name == "" {
					name = strings.TrimSpace(result.Head)
				}
				condition, _, detail := conditionFromResult(result, profile, name)
				for _, at := range bySpan[key] {
					// The compared renderer only observes resolution-level
					// conditions. Other conditions stay primary-only rather
					// than claim an accepted status this pass never checked.
					if !resolutionObservable(findings[at].Condition) {
						continue
					}
					if status != StatusAccepted && findings[at].Condition != condition {
						continue
					}
					findings[at].Renderers = append(findings[at].Renderers, observation)
				}
				if condition == "" {
					return true
				}
				for _, at := range bySpan[key] {
					if findings[at].Condition == condition {
						return true
					}
				}
				createdKey := createdKey{span: key, condition: condition}
				if at, ok := created[createdKey]; ok {
					added[at].Renderers = append(added[at].Renderers, observation)
					return true
				}
				rule := RuleFor(condition)
				slashes := len(result.Raw) - len(strings.TrimLeft(result.Raw, "\\"))
				created[createdKey] = len(added)
				added = append(added, Diagnostic{
					Condition: condition, Title: rule.Title, Description: rule.Description,
					Fix: rule.Fix, FixSafety: rule.FixSafety, Sources: rule.Sources,
					Line: dialogue.Line, Column: result.Source.Start + slashes + 1, Tag: name,
					Detail: detail, Renderers: []Observation{observation},
					SourceSpan: &ass.ConcreteSpan{Start: span.Start, End: span.End},
				})
				return true
			})
		}
	}
	return append(findings, added...)
}
