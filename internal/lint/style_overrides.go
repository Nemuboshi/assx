package lint

import (
	"sort"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/spec"
	"assx/internal/semantic"
)

type overrideBlockRange struct {
	start int
	end   int
}

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
	state := cloneStyleValues(activeBase.Values)

	tree := dialogue.ParsedText()
	var allTags, candidates, runTags []ass.Tag
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

	for _, token := range tree.Tokens() {
		if token.Tag == nil {
			if token.Text != "" {
				flushRun(true)
			}
			continue
		}

		tag := *token.Tag
		allTags = append(allTags, tag)
		tagSpec, known := spec.TagSpecs[tag.Name]
		if !known || tagSpec.VSFilterModOnly || tag.InTransition || tag.RepeatedSlashes > 0 {
			flushRun(false)
			break
		}

		if tagSpec.Behavior == spec.StyleReset {
			flushRun(false)
			target := originalStyle
			if len(tag.Args) > 0 && strings.TrimSpace(tag.Args[0]) != "" {
				target = strings.TrimSpace(tag.Args[0])
			}
			nextBase, exists := styleStates[target]
			if !exists {
				break
			}
			activeBase = nextBase
			state = cloneStyleValues(activeBase.Values)
			continue
		}

		if !semantic.SafeIndependentStyleTag(tag) {
			flushRun(false)
			break
		}

		var slots []string
		for _, slot := range tagSpec.Slots {
			if activeBase.Slots[slot] {
				slots = append(slots, slot)
			}
		}
		if len(slots) == 0 {
			continue
		}
		if len(slots) != len(tagSpec.Slots) {
			flushRun(false)
			break
		}

		values, valueOK := semantic.StyleTagState(tag, tagSpec, slots, activeBase.Values)
		if !valueOK {
			flushRun(false)
			break
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
	}
	flushRun(false)

	if len(candidates) == 0 {
		return Diagnostic{}, false
	}
	edits := redundantStyleEdits(dialogue, tree, allTags, candidates)
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

func cloneStyleValues(values map[string]semantic.StateValue) map[string]semantic.StateValue {
	cloned := make(map[string]semantic.StateValue, len(values))
	for slot, value := range values {
		cloned[slot] = value
	}
	return cloned
}

func redundantStyleEdits(dialogue ass.Dialogue, tree ass.DialogueText, allTags, styleTags []ass.Tag) []TextEdit {
	candidate := make(map[[2]int]bool, len(styleTags))
	for _, tag := range styleTags {
		candidate[[2]int{tag.Start, tag.End}] = true
	}

	blockByTag := make(map[[2]int]overrideBlockRange)
	blockNodes := make(map[overrideBlockRange]*ass.OverrideBlock)
	for i := range tree.Nodes {
		node := &tree.Nodes[i]
		if node.Block == nil {
			continue
		}
		block := overrideBlockRange{start: node.Block.Start, end: node.Block.End}
		blockNodes[block] = node.Block
		for _, tag := range node.Block.Tags() {
			blockByTag[[2]int{tag.Start, tag.End}] = block
		}
	}

	allByBlock := make(map[overrideBlockRange][]ass.Tag)
	styleByBlock := make(map[overrideBlockRange][]ass.Tag)
	for _, tag := range allTags {
		if block, ok := blockByTag[[2]int{tag.Start, tag.End}]; ok {
			allByBlock[block] = append(allByBlock[block], tag)
		}
	}
	for _, tag := range styleTags {
		if block, ok := blockByTag[[2]int{tag.Start, tag.End}]; ok {
			styleByBlock[block] = append(styleByBlock[block], tag)
		}
	}

	var edits []TextEdit
	for block, tags := range styleByBlock {
		all := allByBlock[block]
		if len(all) == len(tags) && blockContainsOnlyCandidateTags(blockNodes[block], candidate) {
			edits = append(edits, TextEdit{
				Start: dialogue.TextStart + block.start,
				End:   dialogue.TextStart + block.end,
			})
			continue
		}
		for _, tag := range tags {
			edits = append(edits, TextEdit{
				Start: dialogue.TextStart + tag.Start,
				End:   dialogue.TextStart + tag.End,
			})
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
