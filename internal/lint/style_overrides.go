package lint

import (
	"maps"
	"sort"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/spec"
	"assx/internal/semantic"
)

// AnalyzeRedundantStyleOverrides checks Style-backed override state for each dialogue.
func AnalyzeRedundantStyleOverrides(doc ass.Document) []Diagnostic {
	styles := semantic.StyleDefinitionsByName(doc.StyleFields)
	var diagnostics []Diagnostic
	for _, dialogue := range doc.Dialogues {
		originalName := semantic.DialogueStyleLookupName(dialogue.Style)
		if _, ok := styles[originalName]; !ok {
			continue
		}
		diagnostic, ok := analyzeRedundantStyleDialogue(dialogue, originalName, styles)
		if ok {
			diagnostics = append(diagnostics, diagnostic)
		}
	}
	return diagnostics
}

func analyzeRedundantStyleDialogue(dialogue ass.Dialogue, originalStyle string, styles map[string]map[string]string) (Diagnostic, bool) {
	styleStates := make(map[string]semantic.StyleState, len(styles))
	for name, fields := range styles {
		styleStates[name] = semantic.CanonicalStyleState(fields)
	}
	activeBase, ok := styleStates[originalStyle]
	if !ok {
		return Diagnostic{}, false
	}
	state := maps.Clone(activeBase.Values)

	tree := dialogue.ParsedText()
	var candidates, runTags []ass.Tag
	runStart := make(map[string]semantic.StateValue)
	runTouched := make(map[string]bool)

	flushRun := func(atVisibleText bool) {
		if len(runTags) == 0 {
			return
		}
		if atVisibleText {
			redundant := true
			for slot := range runTouched {
				baseValue := activeBase.Values[slot]
				if runStart[slot] != baseValue || state[slot] != baseValue {
					redundant = false
					break
				}
			}
			if redundant {
				candidates = append(candidates, runTags...)
			}
		}
		runTags = nil
		runStart = make(map[string]semantic.StateValue)
		runTouched = make(map[string]bool)
	}

	tree.WalkTokens(func(token ass.TokenView) bool {
		if !token.HasTag {
			if token.Text != "" {
				flushRun(true)
			}
			return true
		}

		tag := token.Tag
		tagSpec, known := spec.TagSpecs[tag.Name]
		if !known || tagSpec.VSFilterModOnly || tag.InTransition || tag.RepeatedSlashes > 0 {
			flushRun(false)
			return false
		}

		if tagSpec.Behavior == spec.StyleReset {
			flushRun(false)
			target := originalStyle
			if len(tag.Args) > 0 && strings.TrimSpace(tag.Args[0]) != "" {
				target = strings.TrimSpace(tag.Args[0])
			}
			nextBase, exists := styleStates[target]
			if !exists {
				return false
			}
			activeBase = nextBase
			state = maps.Clone(activeBase.Values)
			return true
		}

		if !semantic.SafeIndependentStyleTag(tag) {
			flushRun(false)
			return false
		}

		var slots []string
		for _, slot := range tagSpec.Slots {
			if activeBase.Slots[slot] {
				slots = append(slots, slot)
			}
		}
		if len(slots) == 0 {
			return true
		}
		if len(slots) != len(tagSpec.Slots) {
			flushRun(false)
			return false
		}

		values, valueOK := semantic.StyleTagState(tag, tagSpec, slots, activeBase.Values)
		if !valueOK {
			flushRun(false)
			return false
		}
		for _, slot := range slots {
			if !runTouched[slot] {
				runStart[slot] = state[slot]
			}
			runTouched[slot] = true
		}
		runTags = append(runTags, tag)
		for i, slot := range slots {
			state[slot] = semantic.KnownValue(values[i])
		}
		return true
	})
	flushRun(false)

	if len(candidates) == 0 {
		return Diagnostic{}, false
	}
	edits := redundantStyleEdits(dialogue, tree, candidates)
	if len(edits) == 0 {
		return Diagnostic{}, false
	}
	first := candidates[0]
	return Diagnostic{
		ID: IssueRedundantStyleOverrides, Severity: Suggestion, FixSafety: SafeFix,
		Line: dialogue.Line, Column: first.Column, Tag: first.Name,
		Detail: "The override tags leave the active Style properties unchanged across dialogue text.",
		Edits:  edits,
	}, true
}

func redundantStyleEdits(dialogue ass.Dialogue, tree ass.DialogueText, styleTags []ass.Tag) []TextEdit {
	candidate := make(map[[2]int]bool, len(styleTags))
	for _, tag := range styleTags {
		candidate[[2]int{tag.Start, tag.End}] = true
	}

	var edits []TextEdit
	for _, node := range tree.Nodes {
		block := node.Block
		if node.Kind != ass.OverrideNode || block == nil {
			continue
		}
		hasCandidate := false
		for _, item := range block.Items {
			if item.Tag != nil && candidate[[2]int{item.Tag.Start, item.Tag.End}] {
				hasCandidate = true
				break
			}
		}
		if !hasCandidate {
			continue
		}
		if blockContainsOnlyCandidateTags(block, candidate) {
			edits = append(edits, TextEdit{
				Start: dialogue.TextStart + block.Start,
				End:   dialogue.TextStart + block.End,
			})
			continue
		}
		for _, item := range block.Items {
			if item.Tag != nil && candidate[[2]int{item.Tag.Start, item.Tag.End}] {
				edits = append(edits, TextEdit{
					Start: dialogue.TextStart + item.Tag.Start,
					End:   dialogue.TextStart + item.Tag.End,
				})
			}
		}
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].Start < edits[j].Start })
	return edits
}

func blockContainsOnlyCandidateTags(block *ass.OverrideBlock, candidates map[[2]int]bool) bool {
	if block == nil {
		return false
	}
	for _, item := range block.Items {
		if item.Tag != nil {
			if !candidates[[2]int{item.Tag.Start, item.Tag.End}] {
				return false
			}
			continue
		}
		for _, b := range item.Raw {
			switch b {
			case ' ', '\t', '\r', '\n', '\f', '\v':
			default:
				return false
			}
		}
	}
	return true
}
