package lint

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/edit"
	"assx/internal/semantic"
)

type FixEditProof struct {
	ID   string   `json:"id"`
	Edit TextEdit `json:"edit"`
}

type ProofCapabilities struct {
	VSFilterMod string `json:"vsfiltermod"`
	Lua         string `json:"lua"`
}

type FixTarget struct {
	Renderer     string            `json:"renderer"`
	Version      string            `json:"version"`
	Capabilities ProofCapabilities `json:"capabilities"`
}

type ProofDialogue struct {
	Line              int                  `json:"line"`
	BeforeTraceSHA256 string               `json:"before_trace_sha256"`
	AfterTraceSHA256  string               `json:"after_trace_sha256"`
	Before            []semantic.ProofStep `json:"before_interpretations"`
	After             []semantic.ProofStep `json:"after_interpretations"`
}

type ProofStyleField struct {
	Line   int    `json:"line"`
	Style  string `json:"style"`
	Field  string `json:"field"`
	Before string `json:"before"`
	After  string `json:"after"`
	Value  int64  `json:"renderer_value"`
}

type FixTargetProof struct {
	Target      FixTarget         `json:"target"`
	Dialogues   []ProofDialogue   `json:"dialogues,omitempty"`
	StyleFields []ProofStyleField `json:"style_fields,omitempty"`
}

type FixProof struct {
	ID                string           `json:"id"`
	SourceSHA256      string           `json:"source_sha256"`
	EditedSHA256      string           `json:"edited_sha256"`
	SourceEdits       []FixEditProof   `json:"source_edits"`
	Targets           []FixTarget      `json:"targets"`
	Interpretations   []FixTargetProof `json:"interpretations"`
	VerificationBasis []string         `json:"verification_basis"`
}

func defaultFixTargets() []renderer.Profile {
	libass, err := renderer.Standard(renderer.Libass)
	if err != nil {
		panic(err)
	}
	xy, err := renderer.Standard(renderer.XYVSFilter)
	if err != nil {
		panic(err)
	}
	return []renderer.Profile{libass, xy}
}

func fixTargets(profiles []renderer.Profile) []FixTarget {
	targets := make([]FixTarget, 0, len(profiles))
	for _, profile := range profiles {
		targets = append(targets, FixTarget{
			Renderer: profile.Kind().String(), Version: profile.Version(),
			Capabilities: proofCapabilities(profile.Kind(), profile.Build()),
		})
	}
	return targets
}

func proofCapabilities(kind renderer.Kind, build renderer.Build) ProofCapabilities {
	if kind != renderer.VSFilterMod {
		return ProofCapabilities{VSFilterMod: "not-applicable", Lua: "not-applicable"}
	}
	return ProofCapabilities{VSFilterMod: featureName(build.Mod), Lua: featureName(build.Lua)}
}

func featureName(feature renderer.Feature) string {
	switch feature {
	case renderer.FeatureDisabled:
		return "disabled"
	case renderer.FeatureEnabled:
		return "enabled"
	default:
		return "unknown"
	}
}

func proveSafeFixes(doc ass.Document, diagnostics []Diagnostic, profiles []renderer.Profile) []Diagnostic {
	var candidates []int
	var selected []FixEditProof
	for i, diagnostic := range diagnostics {
		if diagnostic.FixSafety != SafeFix {
			continue
		}
		if len(diagnostic.Edits) == 0 {
			diagnostics[i].FixSafety = ""
			diagnostics[i].FixProof = nil
			continue
		}
		candidates = append(candidates, i)
		for _, sourceEdit := range diagnostic.Edits {
			selected = append(selected, FixEditProof{ID: diagnostic.ID, Edit: sourceEdit})
		}
	}
	if len(candidates) == 0 {
		return diagnostics
	}
	validEdits := fixEditIndex(diagnostics)
	proofDoc := proofDocument(doc.Text, selected)
	if proof, err := buildFixProofWithDocument(doc.Text, proofDoc, selected, profiles, validEdits); err == nil {
		attachProofReferences(diagnostics, candidates, proof)
		return diagnostics
	}

	// shortcut: stop after 64 proof builds; raise the cap if measured workloads leave valid SafeFix edits unproved.
	const maxProofAttempts = 64
	queue := make([][]int, 0, 2)
	if len(candidates) > 1 {
		queue = append(queue, candidates[:len(candidates)/2], candidates[len(candidates)/2:])
	}
	for attempts := 1; len(queue) > 0 && attempts < maxProofAttempts; attempts++ {
		indexes := queue[0]
		queue = queue[1:]
		if len(indexes) == 0 {
			continue
		}
		edits := fixEditsForDiagnostics(diagnostics, indexes)
		proof, err := buildFixProofWithDocument(doc.Text, proofDoc, edits, profiles, validEdits)
		if err == nil {
			attachProofReferences(diagnostics, indexes, proof)
			continue
		}
		if len(indexes) > 1 {
			middle := len(indexes) / 2
			queue = append(queue, indexes[:middle], indexes[middle:])
		}
	}
	for _, index := range candidates {
		if diagnostics[index].FixProof == nil {
			diagnostics[index].FixSafety = ""
			diagnostics[index].Edits = nil
			diagnostics[index].FixProof = nil
			diagnostics[index].FixProofRef = ""
		}
	}
	return diagnostics
}

func fixEditsForDiagnostics(diagnostics []Diagnostic, indexes []int) []FixEditProof {
	var edits []FixEditProof
	for _, index := range indexes {
		for _, sourceEdit := range diagnostics[index].Edits {
			edits = append(edits, FixEditProof{ID: diagnostics[index].ID, Edit: sourceEdit})
		}
	}
	return edits
}

func fixEditIndex(diagnostics []Diagnostic) map[FixEditProof]struct{} {
	valid := make(map[FixEditProof]struct{})
	for _, diagnostic := range diagnostics {
		if diagnostic.FixSafety != SafeFix {
			continue
		}
		for _, sourceEdit := range diagnostic.Edits {
			valid[FixEditProof{ID: diagnostic.ID, Edit: sourceEdit}] = struct{}{}
		}
	}
	return valid
}

func attachProofReferences(diagnostics []Diagnostic, indexes []int, proof *FixProof) {
	for i, index := range indexes {
		diagnostics[index].FixProof = proof
		diagnostics[index].FixProofRef = ""
		if i != 0 {
			diagnostics[index].FixProofRef = proof.ID
		}
	}
}

func buildFixProof(source string, edits []FixEditProof, profiles []renderer.Profile) (*FixProof, error) {
	doc := proofDocument(source, edits)
	return buildFixProofWithEditIndex(source, edits, profiles, fixEditIndex(analyzeDocumentUnproved(doc)))
}

func buildFixProofWithEditIndex(source string, edits []FixEditProof, profiles []renderer.Profile, validEdits map[FixEditProof]struct{}) (*FixProof, error) {
	return buildFixProofWithDocument(source, proofDocument(source, edits), edits, profiles, validEdits)
}

func buildFixProofWithDocument(source string, beforeDoc ass.Document, edits []FixEditProof, profiles []renderer.Profile, validEdits map[FixEditProof]struct{}) (*FixProof, error) {
	if len(profiles) == 0 {
		return nil, fmt.Errorf("SafeFix proof has no renderer targets")
	}
	for _, sourceEdit := range edits {
		if !supportedProofRule(sourceEdit.ID) {
			return nil, fmt.Errorf("rule %s has no renderer proof", sourceEdit.ID)
		}
		if _, ok := validEdits[sourceEdit]; !ok {
			return nil, fmt.Errorf("rule %s edit does not match a current lint finding", sourceEdit.ID)
		}
	}
	targets := fixTargets(profiles)
	for i := range targets {
		if targets[i].Version == "" {
			return nil, fmt.Errorf("renderer %s has no pinned version", targets[i].Renderer)
		}
		for j := 0; j < i; j++ {
			if targets[i] == targets[j] {
				return nil, fmt.Errorf("duplicate renderer target %s", targets[i].Renderer)
			}
		}
	}
	changed, err := edit.Apply(source, proofTextEdits(edits))
	if err != nil {
		return nil, err
	}
	var afterDoc ass.Document
	if hasStyleFieldEdits(edits) {
		afterDoc = proofDocument(changed, edits)
	} else {
		afterDoc, err = editedDialogueDocument(beforeDoc, changed, edits)
		if err != nil {
			return nil, err
		}
		afterDoc.Text = changed
	}
	if len(beforeDoc.Dialogues) != len(afterDoc.Dialogues) || !reflect.DeepEqual(beforeDoc.EventFormat, afterDoc.EventFormat) {
		return nil, fmt.Errorf("edit changed document event parsing")
	}
	if !sameDocumentShape(beforeDoc, afterDoc, edits) {
		return nil, fmt.Errorf("edit changed document structure outside a verified style integer field")
	}
	stylesBefore := semantic.StyleStatesByName(beforeDoc.StyleFields)
	stylesAfter := semantic.StyleStatesByName(afterDoc.StyleFields)
	if !reflect.DeepEqual(stylesBefore, stylesAfter) {
		return nil, fmt.Errorf("edit changed modeled Style state")
	}
	styleEvidence, err := verifyStyleIntegerEdits(beforeDoc, afterDoc, edits)
	if err != nil {
		return nil, err
	}
	syntaxOnly := len(edits) != 0
	for _, sourceEdit := range edits {
		if sourceEdit.ID != IssueEmptyOverrideBlock {
			syntaxOnly = false
			break
		}
		if !isEmptyOverrideEdit(beforeDoc, sourceEdit.Edit) {
			return nil, fmt.Errorf("ASS018 edit does not match an empty override block")
		}
	}

	proof := &FixProof{
		SourceSHA256: hashSource(source), EditedSHA256: hashSource(changed),
		SourceEdits: slices.Clone(edits), Targets: targets,
		VerificationBasis: []string{
			"Compare interpretations within every pinned target renderer.",
			"Require verified signatures and citations for every command affected by a semantic edit.",
			"Compare the full trace when an edit removes only an empty override block.",
			"Compare visible text, tracked state values, and source provenance before and after the edit.",
			"Reject edits that change document parsing or an unresolved semantic operation.",
			"Compare integer Style fields by the exact integer consumed before and after the edit.",
			"Use libass rendercheck for pixel regression fixtures; use pinned-source and trace evidence for xy-VSFilter.",
		},
	}
	byDialogue := dialogueEditsByIndex(beforeDoc, edits)
	affected := affectedDialogues(beforeDoc, edits, byDialogue)
	for _, profile := range profiles {
		target := FixTargetProof{Target: FixTarget{
			Renderer: profile.Kind().String(), Version: profile.Version(),
			Capabilities: proofCapabilities(profile.Kind(), profile.Build()),
		}}
		for _, dialogueIndex := range affected {
			beforeDialogue := beforeDoc.Dialogues[dialogueIndex]
			afterDialogue := afterDoc.Dialogues[dialogueIndex]
			dialogueEdits := byDialogue[dialogueIndex]
			var comparison semantic.ProofComparison
			if syntaxOnly {
				comparison = semantic.CompareSyntaxProof(
					ass.ParseConcreteDialogue(beforeDialogue.Text),
					ass.ParseConcreteDialogue(afterDialogue.Text), profile,
					semantic.EvaluationOptions{Styles: stylesBefore, DialogueStyle: beforeDialogue.Style},
				)
			} else {
				comparison = semantic.CompareResolvedProof(
					ass.ParseConcreteDialogue(beforeDialogue.Text),
					ass.ParseConcreteDialogue(afterDialogue.Text),
					profile,
					semantic.EvaluationOptions{Styles: stylesBefore, DialogueStyle: beforeDialogue.Style},
					removedRanges(beforeDialogue, dialogueEdits),
				)
			}
			if !comparison.Equivalent {
				return nil, fmt.Errorf("%s dialogue line %d: %s", profile.Kind(), beforeDialogue.Line, comparison.Reason)
			}
			beforeHash, err := hashJSON(comparison.Before)
			if err != nil {
				return nil, err
			}
			afterHash, err := hashJSON(comparison.After)
			if err != nil {
				return nil, err
			}
			ranges := dialogueRanges(beforeDialogue, dialogueEdits)
			target.Dialogues = append(target.Dialogues, ProofDialogue{
				Line:              beforeDialogue.Line,
				BeforeTraceSHA256: beforeHash, AfterTraceSHA256: afterHash,
				Before: relevantSteps(comparison.Before.Steps, ranges),
				After:  relevantSteps(comparison.After.Steps, ranges),
			})
		}
		target.StyleFields = slices.Clone(styleEvidence)
		proof.Interpretations = append(proof.Interpretations, target)
	}
	identity := *proof
	identity.ID = ""
	proof.ID, err = hashJSON(identity)
	if err != nil {
		return nil, err
	}
	return proof, nil
}

func isEmptyOverrideEdit(doc ass.Document, edit TextEdit) bool {
	for _, dialogue := range doc.Dialogues {
		for _, node := range dialogue.ParsedText().Nodes {
			if node.Kind != ass.OverrideNode || node.Block == nil || !node.Block.IsEmpty() {
				continue
			}
			if edit.Start == dialogue.TextStart+node.Block.Start && edit.End == dialogue.TextStart+node.Block.End {
				return true
			}
		}
	}
	return false
}

func supportedProofRule(id string) bool {
	switch id {
	case IssueNoEffect, IssueRepeatedSlash, IssueStyleInteger, IssueRedundantStyleOverrides,
		IssueRepeatedOpenBrace, IssueEmptyOverrideBlock:
		return true
	default:
		return false
	}
}

func proofDocument(source string, edits []FixEditProof) ass.Document {
	doc := ass.Parse(source)
	if len(doc.Dialogues) != 0 {
		return doc
	}
	for _, candidate := range edits {
		switch candidate.ID {
		case IssueNoEffect, IssueRepeatedSlash, IssueRedundantStyleOverrides,
			IssueRepeatedOpenBrace, IssueEmptyOverrideBlock:
			doc.Dialogues = []ass.Dialogue{{Text: source, Line: 1}}
			return doc
		}
	}
	return doc
}

func sameDocumentShape(before, after ass.Document, edits []FixEditProof) bool {
	if len(before.StyleFields) != len(after.StyleFields) || len(before.Dialogues) != len(after.Dialogues) {
		return false
	}
	for i, field := range before.StyleFields {
		other := after.StyleFields[i]
		if field.StyleName != other.StyleName || field.Name != other.Name {
			return false
		}
		if field.Value != other.Value && !styleFieldEdited(field, edits, IssueStyleInteger) {
			return false
		}
	}
	for i, dialogue := range before.Dialogues {
		other := after.Dialogues[i]
		if dialogue.Style != other.Style || len(dialogue.Fields) != len(other.Fields) {
			return false
		}
		for j, field := range dialogue.Fields {
			otherField := other.Fields[j]
			if field.Name != otherField.Name {
				return false
			}
			if field.Name != "text" && field.Value != otherField.Value {
				return false
			}
		}
	}
	return true
}

func verifyStyleIntegerEdits(before, after ass.Document, edits []FixEditProof) ([]ProofStyleField, error) {
	var out []ProofStyleField
	for i, field := range before.StyleFields {
		other := after.StyleFields[i]
		if field.Value == other.Value {
			continue
		}
		if !styleFieldEdited(field, edits, IssueStyleInteger) {
			return nil, fmt.Errorf("style field %s.%s changed without an integer proof", field.StyleName, field.Name)
		}
		oldValue := ass.DecodeInteger(strings.TrimSpace(field.Value))
		newValue := ass.DecodeInteger(strings.TrimSpace(other.Value))
		if oldValue.Status != ass.ValueValid || newValue.Status != ass.ValueValid ||
			oldValue.Consumed == 0 || newValue.Consumed == 0 || oldValue.Integer != newValue.Integer {
			return nil, fmt.Errorf("style field %s.%s changes its consumed integer", field.StyleName, field.Name)
		}
		out = append(out, ProofStyleField{
			Line: field.Line, Style: field.StyleName, Field: field.Name,
			Before: field.Value, After: other.Value, Value: oldValue.Integer,
		})
	}
	return out, nil
}

func styleFieldEdited(field ass.StyleField, edits []FixEditProof, id string) bool {
	for _, candidate := range edits {
		if candidate.ID == id && candidate.Edit.Start < field.ValueEnd && field.ValueStart < candidate.Edit.End {
			return true
		}
	}
	return false
}

func hasStyleFieldEdits(edits []FixEditProof) bool {
	for _, candidate := range edits {
		if candidate.ID == IssueStyleInteger {
			return true
		}
	}
	return false
}

func editedDialogueDocument(before ass.Document, changed string, edits []FixEditProof) (ass.Document, error) {
	byDialogue := dialogueEditsByIndex(before, edits)
	var mapped int
	for _, dialogueEdits := range byDialogue {
		mapped += len(dialogueEdits)
	}
	if mapped != len(edits) {
		return ass.Document{}, fmt.Errorf("edit is outside a dialogue text field")
	}

	// Text-column edits keep event fields intact; renderer semantics reparse each edited dialogue.
	after := before
	after.Dialogues = slices.Clone(before.Dialogues)
	var offsetDelta int
	for i := range before.Dialogues {
		original := before.Dialogues[i]
		afterStart := original.TextStart + offsetDelta
		after.Dialogues[i].TextStart = afterStart
		dialogueEdits := byDialogue[i]
		if len(dialogueEdits) == 0 {
			if afterStart < 0 || afterStart+len(original.Text) > len(changed) || changed[afterStart:afterStart+len(original.Text)] != original.Text {
				return ass.Document{}, fmt.Errorf("unchanged dialogue text does not match the edited source")
			}
			continue
		}
		localEdits := make([]edit.TextEdit, 0, len(dialogueEdits))
		for _, candidate := range dialogueEdits {
			localEdits = append(localEdits, edit.TextEdit{
				Start:       candidate.Edit.Start - original.TextStart,
				End:         candidate.Edit.End - original.TextStart,
				Replacement: candidate.Edit.Replacement,
			})
		}
		fixedText, err := edit.Apply(original.Text, localEdits)
		if err != nil {
			return ass.Document{}, err
		}
		if afterStart < 0 || afterStart+len(fixedText) > len(changed) || changed[afterStart:afterStart+len(fixedText)] != fixedText {
			return ass.Document{}, fmt.Errorf("edited dialogue text does not match the edited source")
		}
		after.Dialogues[i].Text = fixedText
		after.Dialogues[i].Fields = slices.Clone(original.Fields)
		for fieldIndex := range after.Dialogues[i].Fields {
			if after.Dialogues[i].Fields[fieldIndex].Name == "text" {
				after.Dialogues[i].Fields[fieldIndex].Value = fixedText
			}
		}
		offsetDelta += len(fixedText) - len(original.Text)
	}
	return after, nil
}

func dialogueEditsByIndex(doc ass.Document, edits []FixEditProof) map[int][]FixEditProof {
	byDialogue := make(map[int][]FixEditProof)
	for _, candidate := range edits {
		index := sort.Search(len(doc.Dialogues), func(i int) bool {
			dialogue := doc.Dialogues[i]
			return dialogue.TextStart+len(dialogue.Text) > candidate.Edit.Start
		})
		if index == len(doc.Dialogues) {
			continue
		}
		dialogue := doc.Dialogues[index]
		if candidate.Edit.Start < dialogue.TextStart || candidate.Edit.End > dialogue.TextStart+len(dialogue.Text) {
			continue
		}
		byDialogue[index] = append(byDialogue[index], candidate)
	}
	return byDialogue
}

func affectedDialogues(doc ass.Document, edits []FixEditProof, byDialogue map[int][]FixEditProof) []int {
	styleChanged := false
	for _, candidate := range edits {
		if candidate.ID != IssueStyleInteger {
			continue
		}
		for _, field := range doc.StyleFields {
			if candidate.Edit.Start < field.ValueEnd && field.ValueStart < candidate.Edit.End {
				styleChanged = true
				break
			}
		}
	}
	var out []int
	for i := range doc.Dialogues {
		if styleChanged || len(byDialogue[i]) != 0 {
			out = append(out, i)
		}
	}
	return out
}

func removedRanges(dialogue ass.Dialogue, edits []FixEditProof) []ass.ConcreteSpan {
	var out []ass.ConcreteSpan
	for _, candidate := range edits {
		if candidate.ID != IssueNoEffect && candidate.ID != IssueRedundantStyleOverrides {
			continue
		}
		start := max(candidate.Edit.Start-dialogue.TextStart, 0)
		end := min(candidate.Edit.End-dialogue.TextStart, len(dialogue.Text))
		if start < end {
			out = append(out, ass.ConcreteSpan{Start: start, End: end})
		}
	}
	return out
}

func dialogueRanges(dialogue ass.Dialogue, edits []FixEditProof) []ass.ConcreteSpan {
	var out []ass.ConcreteSpan
	for _, candidate := range edits {
		start := candidate.Edit.Start - dialogue.TextStart
		end := candidate.Edit.End - dialogue.TextStart
		if start < 0 || end > len(dialogue.Text) || start > end {
			continue
		}
		out = append(out, ass.ConcreteSpan{Start: start, End: end})
	}
	return out
}

func relevantSteps(steps []semantic.ProofStep, ranges []ass.ConcreteSpan) []semantic.ProofStep {
	if len(steps) == 0 {
		return nil
	}
	selected := make(map[int]bool)
	for _, span := range ranges {
		before, after := -1, -1
		for i, step := range steps {
			if step.Source.End <= span.Start {
				before = i
			}
			if after < 0 && step.Source.Start >= span.End {
				after = i
			}
			if step.Source.Start < span.End && span.Start < step.Source.End {
				selected[i] = true
			}
		}
		if before >= 0 {
			selected[before] = true
		}
		if after >= 0 {
			selected[after] = true
		}
	}
	var slots []string
	for i, step := range steps {
		if !selected[i] {
			continue
		}
		slots = append(slots, step.Slots...)
	}
	out := make([]semantic.ProofStep, 0, len(selected))
	for i, step := range steps {
		if !selected[i] {
			continue
		}
		step.State.Values = proofValues(step.State.Values, slots)
		step.State.Sources = proofSources(step.State.Sources, slots)
		out = append(out, step)
	}
	return out
}

func proofValues(values map[string]semantic.ProofValue, slots []string) map[string]semantic.ProofValue {
	out := make(map[string]semantic.ProofValue, len(slots))
	for _, slot := range slots {
		if value, ok := values[slot]; ok {
			out[slot] = value
		}
	}
	return out
}

func proofSources(sources map[string]int, slots []string) map[string]int {
	out := make(map[string]int, len(slots))
	for _, slot := range slots {
		if source, ok := sources[slot]; ok {
			out[slot] = source
		}
	}
	return out
}

func proofTextEdits(proofs []FixEditProof) []edit.TextEdit {
	out := make([]edit.TextEdit, 0, len(proofs))
	for _, proof := range proofs {
		out = append(out, proof.Edit)
	}
	return out
}

func hashSource(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

func hashJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode proof trace: %w", err)
	}
	return hashSource(string(encoded)), nil
}

func profilesFromProof(targets []FixTarget) ([]renderer.Profile, error) {
	profiles := make([]renderer.Profile, 0, len(targets))
	for _, target := range targets {
		var kind renderer.Kind
		switch target.Renderer {
		case renderer.Libass.String():
			kind = renderer.Libass
		case renderer.XYVSFilter.String():
			kind = renderer.XYVSFilter
		case renderer.VSFilterMod.String():
			kind = renderer.VSFilterMod
		default:
			return nil, fmt.Errorf("unknown renderer in SafeFix proof: %s", target.Renderer)
		}
		build, err := parseProofCapabilities(kind, target.Capabilities)
		if err != nil {
			return nil, fmt.Errorf("renderer %s capabilities: %w", target.Renderer, err)
		}
		profile, err := renderer.New(kind, build)
		if err != nil {
			return nil, err
		}
		if profile.Version() != target.Version {
			return nil, fmt.Errorf("renderer version changed for %s", target.Renderer)
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

func parseProofCapabilities(kind renderer.Kind, capabilities ProofCapabilities) (renderer.Build, error) {
	if kind != renderer.VSFilterMod {
		if capabilities != (ProofCapabilities{VSFilterMod: "not-applicable", Lua: "not-applicable"}) {
			return renderer.Build{}, fmt.Errorf("features do not apply to this renderer")
		}
		return renderer.Build{}, nil
	}
	mod, err := parseFeatureName(capabilities.VSFilterMod)
	if err != nil {
		return renderer.Build{}, err
	}
	lua, err := parseFeatureName(capabilities.Lua)
	if err != nil {
		return renderer.Build{}, err
	}
	return renderer.Build{Mod: mod, Lua: lua}, nil
}

func parseFeatureName(name string) (renderer.Feature, error) {
	switch name {
	case "unknown":
		return renderer.FeatureUnknown, nil
	case "disabled":
		return renderer.FeatureDisabled, nil
	case "enabled":
		return renderer.FeatureEnabled, nil
	default:
		return renderer.FeatureUnknown, fmt.Errorf("invalid feature state %q", name)
	}
}

func sameFixTargets(a, b []FixTarget) bool {
	return reflect.DeepEqual(a, b)
}

func sameFixEditSet(a, b []FixEditProof) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[FixEditProof]int, len(a))
	for _, candidate := range a {
		counts[candidate]++
	}
	for _, candidate := range b {
		if counts[candidate] == 0 {
			return false
		}
		counts[candidate]--
	}
	return true
}

func proofIdentityMatches(proof *FixProof) bool {
	if proof == nil || proof.ID == "" {
		return false
	}
	identity := *proof
	identity.ID = ""
	id, err := hashJSON(identity)
	return err == nil && id == proof.ID
}

func verifyFixProofs(source string, beforeDoc ass.Document, selected []FixEditProof, proof *FixProof, validEdits map[FixEditProof]struct{}) error {
	if proof == nil || proof.ID == "" || proof.SourceSHA256 != hashSource(source) {
		return fmt.Errorf("SafeFix proof does not match the current source")
	}
	if !sameFixEditSet(selected, proof.SourceEdits) {
		return fmt.Errorf("selected SafeFix edits do not match the proof")
	}
	if !sameFixTargets(proof.Targets, fixTargets(defaultFixTargets())) {
		return fmt.Errorf("default SafeFix application requires libass and xy-VSFilter proofs")
	}
	if !proofIdentityMatches(proof) {
		return fmt.Errorf("SafeFix proof identity does not match its evidence")
	}
	profiles, err := profilesFromProof(proof.Targets)
	if err != nil {
		return err
	}
	verified, err := buildFixProofWithDocument(source, beforeDoc, proof.SourceEdits, profiles, validEdits)
	if err != nil {
		return err
	}
	if verified.ID != proof.ID || verified.EditedSHA256 != proof.EditedSHA256 ||
		!reflect.DeepEqual(verified.SourceEdits, proof.SourceEdits) || !sameFixTargets(verified.Targets, proof.Targets) {
		return fmt.Errorf("SafeFix proof identity does not match the recomputed evidence")
	}
	return nil
}
