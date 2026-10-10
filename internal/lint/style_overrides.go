package lint

import (
	"sort"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/semantic"
)

// AnalyzeRedundantStyleOverrides retains the standalone entry point while
// sharing the state transitions used by ASS006 and the font checker.
func AnalyzeRedundantStyleOverrides(doc ass.Document) []Diagnostic {
	return analyzeRedundantStyleOverrides(doc, nil)
}

// AnalyzeRedundantStyleOverridesForRenderer reports renderer-local redundancy.
// It deliberately provides no edits: a single-profile observation is not the
// all-target equivalence proof required by SafeFix (P08).
func AnalyzeRedundantStyleOverridesForRenderer(doc ass.Document, profile renderer.Profile) []Diagnostic {
	return scopeDiagnostics(analyzeRedundantStyleOverrides(doc, &profile), profile)
}

func analyzeRedundantStyleOverrides(doc ass.Document, profile *renderer.Profile) []Diagnostic {
	styles := semantic.StyleStatesByName(doc.StyleFields)
	var diagnostics []Diagnostic
	for _, dialogue := range doc.Dialogues {
		var collector *styleRunCollector
		if profile == nil {
			collector = newStyleRunCollector(dialogue, styles)
		} else {
			collector = newStyleRunCollectorForRenderer(dialogue, styles)
		}
		if collector == nil {
			continue
		}
		collector.allowEdits = profile == nil
		semantic.Evaluate(dialogue.ParsedText(), semantic.EvaluationOptions{
			Styles: styles, DialogueStyle: dialogue.Style, Profile: profile,
			Observer: semantic.Observer{Tag: collector.onTag, Text: collector.onText},
		})
		if diagnostic, ok := collector.diagnostic(); ok {
			diagnostics = append(diagnostics, diagnostic)
		}
	}
	return diagnostics
}

// styleRunCollector owns only the ASS013 lint policy (grouping removable tags).
// It never interprets tag values, renderer precedence, or reset semantics.
type styleRunCollector struct {
	dialogue   ass.Dialogue
	tree       ass.DialogueText
	styles     map[string]semantic.StyleState
	stopped    bool
	hasTags    bool
	candidates []ass.Tag
	runTags    []ass.Tag
	runStart   map[string]semantic.StateValue
	runTouched map[string]bool
	allowEdits bool
}

func newStyleRunCollector(dialogue ass.Dialogue, styles map[string]semantic.StyleState) *styleRunCollector {
	return makeStyleRunCollector(dialogue, styles, dialogue.ParsedText().HasTags())
}

func newStyleRunCollectorForRenderer(dialogue ass.Dialogue, styles map[string]semantic.StyleState) *styleRunCollector {
	return makeStyleRunCollector(dialogue, styles, ass.ParseConcreteDialogue(dialogue.Text).HasCandidates())
}

func makeStyleRunCollector(dialogue ass.Dialogue, styles map[string]semantic.StyleState, hasTags bool) *styleRunCollector {
	if !hasTags {
		return nil
	}
	name := semantic.DialogueStyleLookupName(dialogue.Style)
	if _, ok := styles[name]; !ok {
		return nil
	}
	return &styleRunCollector{
		dialogue: dialogue, tree: dialogue.ParsedText(), styles: styles,
		runStart:   make(map[string]semantic.StateValue),
		runTouched: make(map[string]bool),
		allowEdits: true,
	}
}

func (c *styleRunCollector) flush(atVisibleText bool, state semantic.StateView) {
	if len(c.runTags) == 0 {
		return
	}
	if atVisibleText {
		base, ok := c.styles[state.ActiveStyle()]
		redundant := ok
		for slot := range c.runTouched {
			baseValue := base.Values[slot]
			if !baseValue.Known || c.runStart[slot] != baseValue || state.Value(slot) != baseValue {
				redundant = false
				break
			}
		}
		if redundant {
			c.candidates = append(c.candidates, c.runTags...)
		}
	}
	c.runTags = nil
	clear(c.runStart)
	clear(c.runTouched)
}

func (c *styleRunCollector) onText(text string, _ int, state semantic.StateView) {
	if !c.stopped && text != "" {
		c.flush(true, state)
	}
}

func (c *styleRunCollector) onTag(event semantic.TagEvent, state semantic.StateView) {
	tag := event.Tag
	// A semantic proof barrier revokes candidates collected before it.
	// Check even after stopping ASS013's local run: unsupported tags can
	// occur inside later transforms, after earlier valid text boundaries.
	if event.Barrier {
		c.candidates = nil
		c.runTags = nil
		c.stopped = true
		return
	}
	if c.stopped {
		return
	}
	c.hasTags = true
	if tag.Name == "r" && !tag.InTransition {
		c.flush(false, state)
		if _, ok := c.styles[state.ActiveStyle()]; !ok {
			c.stopped = true
		}
		return
	}
	if event.Barrier || !semantic.SafeIndependentStyleEvent(event) {
		c.flush(false, state)
		c.stopped = true
		return
	}
	activeBase, ok := c.styles[state.ActiveStyle()]
	if !ok {
		c.flush(false, state)
		c.stopped = true
		return
	}
	var slots []string
	for i, slot := range event.Slots {
		if activeBase.Slots[slot] {
			slots = append(slots, slot)
			if !c.runTouched[slot] {
				c.runStart[slot] = event.Before[i]
			}
		}
	}
	if len(slots) == 0 {
		return
	}
	if len(slots) != len(event.Slots) || !event.Known {
		c.flush(false, state)
		c.stopped = true
		return
	}
	for _, slot := range slots {
		c.runTouched[slot] = true
	}
	c.runTags = append(c.runTags, tag)
}

func (c *styleRunCollector) diagnostic() (Diagnostic, bool) {
	if !c.hasTags || len(c.candidates) == 0 {
		return Diagnostic{}, false
	}
	var edits []TextEdit
	var safety FixSafety
	if c.allowEdits {
		edits = redundantStyleEdits(c.dialogue, c.tree, c.candidates)
		if len(edits) == 0 {
			return Diagnostic{}, false
		}
		safety = Rules[IssueRedundantStyleOverrides].FixSafety
	}
	first := c.candidates[0]
	rule := Rules[IssueRedundantStyleOverrides]
	return Diagnostic{
		ID: rule.ID, Severity: rule.Severity, FixSafety: safety,
		Title: rule.Title, Description: rule.Description, Fix: rule.Fix, Sources: rule.Sources,
		Line: c.dialogue.Line, Column: first.Column, Tag: first.Name,
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
