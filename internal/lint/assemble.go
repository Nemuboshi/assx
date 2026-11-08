package lint

import (
	"sort"
	"strings"

	"assx/internal/ass/renderer"
	"assx/internal/semantic"
)

// Assembler turns observations into one finding per (span, condition):
// renderer results merge, severity derives from the primary observation, and
// one span reports each condition once.

// statusForCondition is the observation status a fired rule implies.
func statusForCondition(condition string) Status {
	switch condition {
	case IssueUnknownTag:
		return StatusUnknown
	case IssueRendererUnresolved:
		return StatusUnmodeled
	case IssueNoEffect, IssueMatrixHeader, IssueLayoutRes, IssueStyleInteger,
		IssueStyleFloat, IssueRedundantStyleOverrides, IssueRepeatedOpenBrace,
		IssueEmptyOverrideBlock, IssueOverrideJunk, IssueRepeatedSlash:
		return StatusIgnored
	case IssueVSFilterModTag:
		return StatusAccepted
	default:
		return StatusRejected
	}
}

// stampPass gives rule findings the observation of the pass that produced them.
func stampPass(findings []Diagnostic, profile renderer.Profile, role Role) {
	for i := range findings {
		if len(findings[i].Renderers) != 0 {
			continue
		}
		findings[i].Renderers = []Observation{{
			Renderer: profile.Kind().String(),
			Role:     role,
			Status:   statusForCondition(findings[i].Condition),
		}}
		if role != RolePrimary {
			// Only the primary pass authorizes edits.
			findings[i].FixSafety = ""
			findings[i].Edits = nil
			findings[i].FixProof = nil
			findings[i].FixProofRef = 0
		}
	}
}

// stampPrimary is the primary-pass shorthand.
func stampPrimary(findings []Diagnostic, profile renderer.Profile) {
	stampPass(findings, profile, RolePrimary)
}

// observationStatusFrom resolves one renderer's match status.
func observationStatusFrom(status renderer.MatchStatus) Status {
	switch status {
	case renderer.Disabled:
		return StatusUnmodeled
	case renderer.Ignored:
		return StatusIgnored
	case renderer.UnknownName:
		return StatusUnknown
	default:
		return StatusAccepted
	}
}

// mergeObservations keeps one entry per renderer and puts the primary first.
func mergeObservations(dst, add []Observation, primary renderer.Profile) []Observation {
	seen := make(map[string]bool, len(dst)+len(add))
	out := make([]Observation, 0, len(dst)+len(add))
	primaryName := primary.Kind().String()
	for _, observation := range append(append([]Observation(nil), dst...), add...) {
		if observation.Renderer == primaryName {
			observation.Role = RolePrimary
		}
		if seen[observation.Renderer] {
			continue
		}
		seen[observation.Renderer] = true
		out = append(out, observation)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].Role == RolePrimary) != (out[j].Role == RolePrimary) {
			return out[i].Role == RolePrimary
		}
		return out[i].Renderer < out[j].Renderer
	})
	return out
}

func hasPrimaryObservation(observations []Observation) bool {
	for _, observation := range observations {
		if observation.Role == RolePrimary {
			return true
		}
	}
	return false
}

// deriveSeverity applies the one severity policy: an error-class condition is
// an error only when the primary renderer rejects it; everything else is a
// warning. Decorative conditions stay suggestions.
func deriveSeverity(condition string, observations []Observation) Severity {
	spec, ok := Rules[condition]
	if !ok {
		return Warning
	}
	switch spec.Severity {
	case Suggestion:
		return Suggestion
	case Error:
		for _, observation := range observations {
			if observation.Role == RolePrimary && observation.Status == StatusRejected {
				return Error
			}
		}
		return Warning
	default:
		return Warning
	}
}

type findingKey struct {
	start, end, line, column int
	condition                string
}

func keyOf(diagnostic Diagnostic) findingKey {
	key := findingKey{line: diagnostic.Line, column: diagnostic.Column, condition: diagnostic.Condition}
	if diagnostic.SourceSpan != nil {
		key.start = diagnostic.SourceSpan.Start
		key.end = diagnostic.SourceSpan.End
		key.line, key.column = 0, 0
	}
	return key
}

func mergeSources(dst, add []string) []string {
	seen := make(map[string]bool, len(dst)+len(add))
	out := make([]string, 0, len(dst)+len(add))
	for _, source := range append(append([]string(nil), dst...), add...) {
		if source == "" || seen[source] {
			continue
		}
		seen[source] = true
		out = append(out, source)
	}
	return out
}

// assemble merges observations and derives severity for one document analysis.
func assemble(findings []Diagnostic, primary renderer.Profile) []Diagnostic {
	stampPrimary(findings, primary)
	merged := make([]Diagnostic, 0, len(findings))
	index := make(map[findingKey]int, len(findings))
	for _, finding := range findings {
		key := keyOf(finding)
		if at, ok := index[key]; ok {
			merged[at].Renderers = mergeObservations(merged[at].Renderers, finding.Renderers, primary)
			if merged[at].Scenario == "" {
				merged[at].Scenario = finding.Scenario
			}
			if merged[at].Detail == "" {
				merged[at].Detail = finding.Detail
			}
			merged[at].Sources = mergeSources(merged[at].Sources, finding.Sources)
			continue
		}
		index[key] = len(merged)
		finding.Renderers = mergeObservations(nil, finding.Renderers, primary)
		merged = append(merged, finding)
	}
	for i := range merged {
		if !hasPrimaryObservation(merged[i].Renderers) {
			merged[i].Renderers = append(merged[i].Renderers, Observation{
				Renderer: primary.Kind().String(), Role: RolePrimary, Status: StatusAccepted,
			})
			merged[i].Renderers = mergeObservations(nil, merged[i].Renderers, primary)
		}
		merged[i].Severity = deriveSeverity(merged[i].Condition, merged[i].Renderers)
	}
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].Line == merged[j].Line {
			if merged[i].Column == merged[j].Column {
				return merged[i].Condition < merged[j].Condition
			}
			return merged[i].Column < merged[j].Column
		}
		return merged[i].Line < merged[j].Line
	})
	return merged
}

// isBehaviorCondition reports whether a condition comes from renderer comparison.
func isBehaviorCondition(condition string) bool {
	switch condition {
	case ConditionDispatch, ConditionArguments, ConditionSignature, ConditionState,
		ConditionOwnership, ConditionApplication, ConditionLayout:
		return true
	}
	return false
}

// IsComparisonCondition reports whether a condition exists only because two
// renderers were compared.
func IsComparisonCondition(condition string) bool { return isBehaviorCondition(condition) }

// behaviorCondition maps one comparison dimension to its condition.
func behaviorCondition(dimension string) string {
	switch {
	case strings.HasPrefix(dimension, "document/style-box"):
		return IssueStyleBoxGeometry
	case strings.HasPrefix(dimension, "document/style-border"):
		return IssueStyleBorderMode
	case strings.HasPrefix(dimension, "document/"):
		return ConditionLayout
	case dimension == "behavior/arg-form" || dimension == "arguments":
		return ConditionArguments
	case dimension == "dispatch":
		return ConditionDispatch
	case dimension == "signature":
		return ConditionSignature
	case dimension == "state":
		return ConditionState
	case dimension == "ownership":
		return ConditionOwnership
	case dimension == "application":
		return ConditionApplication
	default:
		return ConditionState
	}
}

// compatibilityObservations turns one pair finding into two observations.
// A document-level finding has no resolved command, so its status comes from
// the comparison outcome instead of a per-renderer match.
func compatibilityObservations(finding semantic.CompatibilityFinding) []Observation {
	documentLevelStatus := StatusAccepted
	switch finding.Status {
	case semantic.CompatibilityUnresolved:
		documentLevelStatus = StatusUnknown
	case semantic.CompatibilityIgnored:
		documentLevelStatus = StatusIgnored
	}
	statusFor := func(interpretation *semantic.Interpretation) Status {
		if interpretation.Resolution.Name == "" {
			return documentLevelStatus
		}
		return observationStatusFrom(interpretation.Resolution.Status)
	}
	return []Observation{
		{
			Renderer: finding.Left.Profile.Kind().String(),
			Role:     RoleCompared,
			Status:   statusFor(finding.Left),
			Citation: finding.Left.Resolution.Citation,
		},
		{
			Renderer: finding.Right.Profile.Kind().String(),
			Role:     RoleCompared,
			Status:   statusFor(finding.Right),
			Citation: finding.Right.Resolution.Citation,
		},
	}
}
