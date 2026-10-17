package lint

import (
	"fmt"
	"reflect"
	"slices"
	"sync/atomic"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/edit"
	"assx/internal/semantic"
)

// Counters for proof preparation work. They count preparations that actually
// ran (cache misses and one-off builds); proofBatchReuse counts reuse of
// prepared before work. Tests watch them to prove before-side work runs once
// per batch, not once per proof attempt.
var (
	proofBatchViews       atomic.Int64
	proofBatchTrees       atomic.Int64
	proofBatchComparisons atomic.Int64
	proofBatchHashes      atomic.Int64
	proofBatchReuse       atomic.Int64
)

func resetProofBatchCounters() {
	proofBatchViews.Store(0)
	proofBatchTrees.Store(0)
	proofBatchComparisons.Store(0)
	proofBatchHashes.Store(0)
	proofBatchReuse.Store(0)
}

// proofBatch owns the request's proof work items. Every proof build in one
// fallback run — the aggregate plus each halved retry — shares it, so the
// before-side preparation no edit can change runs at most once per profile:
// the projected document view, its interpreted Style state, the dialogue
// line index, each affected dialogue's parsed tree, and each accepted
// dialogue's before-trace digest. The changed source, the after document,
// the per-profile after view, and every comparison run per attempt because
// they depend on the exact edit set.
//
// A before entry is stored only after its dialogue-line mapping checks pass,
// so a mismatched Dialogue layout or an uncertain comparison never caches
// work under inputs the gate has not verified, and Style work stays
// document-wide: a Style edit marks every dialogue affected and each
// dialogue keeps its own keyed comparison.
//
// The batch is request-owned: it is reused only while the source, document
// shape, and profile set match exactly, and it never crosses requests.
type proofBatch struct {
	source    string
	beforeDoc ass.Document
	profiles  []renderer.Profile
	rawStyles map[string]semantic.StyleState
	seeded    map[renderer.Profile]ass.Document
	targets   []FixTarget
	preps     map[renderer.Profile]*proofProfilePrep
}

func newProofBatch(source string, beforeDoc ass.Document, profiles []renderer.Profile) *proofBatch {
	return &proofBatch{
		source: source, beforeDoc: beforeDoc, profiles: profiles,
		preps: make(map[renderer.Profile]*proofProfilePrep, len(profiles)),
	}
}

// newSessionProofBatch builds the batch a Discover call attaches to its
// session, seeded with the views discovery already prepared for the source.
func newSessionProofBatch(doc ass.Document, profiles []renderer.Profile, views map[renderer.Profile]ass.Document) *proofBatch {
	batch := newProofBatch(doc.Text, doc, profiles)
	batch.seedViews(views)
	return batch
}

// seedViews adopts projected documents prepared earlier for the exact same
// source, so a session's discovery views feed its proofs without re-parsing.
func (b *proofBatch) seedViews(views map[renderer.Profile]ass.Document) {
	if b != nil && len(views) != 0 {
		b.seeded = views
	}
}

// usable reports whether this batch was built for exactly this source,
// document shape, and profile set. A mismatch gives the caller a fresh
// batch; no work is shared across sources.
func (b *proofBatch) usable(source string, beforeDoc ass.Document, profiles []renderer.Profile) bool {
	if b == nil || b.source != source || b.beforeDoc.Text != beforeDoc.Text || !slices.Equal(b.profiles, profiles) {
		return false
	}
	return len(b.beforeDoc.Lines) == len(beforeDoc.Lines) &&
		len(b.beforeDoc.Dialogues) == len(beforeDoc.Dialogues) &&
		reflect.DeepEqual(b.beforeDoc.EventFormat, beforeDoc.EventFormat)
}

// proofProfilePrep is one profile's before-side work items for the batch.
type proofProfilePrep struct {
	view   ass.Document
	styles map[string]semantic.StyleState
	byLine map[int]ass.Dialogue
	before map[int]*proofBeforeDialogue
	after  map[string]*proofAfterDialogue
}

// proofBeforeDialogue is one verified before dialogue: its view row, parsed
// tree, and accepted before-trace digest. The digest depends only on the
// pinned profile, Style state, and original text, so it is edit-independent.
type proofBeforeDialogue struct {
	dialogue ass.Dialogue
	tree     ass.ConcreteDialogue
	hash     string
}

type proofAfterDialogue struct {
	tree ass.ConcreteDialogue
	hash string
}

// build runs the full gate and evidence pipeline for one exact edit set,
// reusing the batch's prepared before work. validEdits names the current
// findings of this build's request and is checked, never cached: it can
// differ between calls that share a session batch. Gate order and error text
// match the original single-call pipeline, so rejected attempts report
// identical failures and accepted proofs keep identical identity bytes.
func (b *proofBatch) build(edits []FixEditProof, validEdits map[FixEditProof]struct{}) (*FixProof, error) {
	for _, profile := range b.profiles {
		if profile.Kind() == renderer.VSFilterMod && (profile.Build().Mod == renderer.FeatureUnknown || profile.Build().Lua != renderer.FeatureDisabled) {
			return nil, fmt.Errorf("VSFilterMod proof requires a known Mod capability and disabled Lua")
		}
	}
	if len(b.profiles) == 0 {
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
	targets, err := b.fixTargetsChecked()
	if err != nil {
		return nil, err
	}
	changed, err := edit.Apply(b.source, proofTextEdits(edits))
	if err != nil {
		return nil, err
	}
	var afterDoc ass.Document
	if hasStyleFieldEdits(edits) {
		afterDoc = proofDocument(changed, edits)
		if len(b.beforeDoc.Dialogues) != len(afterDoc.Dialogues) || !reflect.DeepEqual(b.beforeDoc.EventFormat, afterDoc.EventFormat) {
			for _, profile := range b.profiles {
				view := profile.DocumentView(afterDoc)
				if len(view.Dialogues) == len(b.beforeDoc.Dialogues) && reflect.DeepEqual(view.EventFormat, b.beforeDoc.EventFormat) {
					afterDoc = view
					break
				}
			}
		}
	} else {
		afterDoc, err = editedDialogueDocument(b.beforeDoc, changed, edits)
		if err != nil {
			return nil, err
		}
		afterDoc.Text = changed
	}
	if len(b.beforeDoc.Dialogues) != len(afterDoc.Dialogues) || !reflect.DeepEqual(b.beforeDoc.EventFormat, afterDoc.EventFormat) {
		return nil, fmt.Errorf("edit changed document event parsing")
	}
	if !sameDocumentShape(b.beforeDoc, afterDoc, edits) {
		return nil, fmt.Errorf("edit changed document structure outside a verified style integer field")
	}
	if b.rawStyles == nil {
		b.rawStyles = semantic.StyleStatesByName(b.beforeDoc.StyleFields)
	}
	if !reflect.DeepEqual(b.rawStyles, semantic.StyleStatesByName(afterDoc.StyleFields)) {
		return nil, fmt.Errorf("edit changed modeled Style state")
	}
	styleEvidence, err := verifyStyleIntegerEdits(b.beforeDoc, afterDoc, edits)
	if err != nil {
		return nil, err
	}
	syntaxOnly := len(edits) != 0
	for _, sourceEdit := range edits {
		if sourceEdit.ID != IssueEmptyOverrideBlock {
			syntaxOnly = false
			break
		}
		if !isEmptyOverrideEdit(b.beforeDoc, sourceEdit.Edit) {
			return nil, fmt.Errorf("ASS018 edit does not match an empty override block")
		}
	}

	proof := &FixProof{
		SourceSHA256: hashSource(b.source), EditedSHA256: hashSource(changed),
		SourceEdits: slices.Clone(edits), Targets: slices.Clone(targets),
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
	byDialogue := dialogueEditsByIndex(b.beforeDoc, edits)
	affected := affectedDialogues(b.beforeDoc, edits, byDialogue)
	for _, profile := range b.profiles {
		prep := b.prep(profile)
		profileAfter := profile.DocumentView(afterDoc)
		if len(b.beforeDoc.Lines) == 0 {
			profileAfter = afterDoc
		}
		if !reflect.DeepEqual(prep.styles, semantic.StyleStatesByName(profileAfter.StyleFields)) {
			return nil, fmt.Errorf("%s edit changes interpreted Style state", profile.Kind())
		}
		profileStyleEvidence, err := verifyStyleIntegerEdits(prep.view, profileAfter, edits)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", profile.Kind(), err)
		}
		if len(profileStyleEvidence) != len(styleEvidence) {
			return nil, fmt.Errorf("%s cannot verify every edited Style field", profile.Kind())
		}
		byLineAfter := make(map[int]ass.Dialogue, len(profileAfter.Dialogues))
		for _, dialogue := range profileAfter.Dialogues {
			byLineAfter[dialogue.Line] = dialogue
		}
		target := FixTargetProof{Target: FixTarget{
			Renderer: profile.Kind().String(), Version: profile.Version(),
			Capabilities: proofCapabilities(profile.Kind(), profile.Build()),
		}}
		for _, dialogueIndex := range affected {
			baseDialogue := b.beforeDoc.Dialogues[dialogueIndex]
			beforeEntry, beforePresent := prep.beforeDialogue(baseDialogue.Line)
			afterDialogue, afterPresent := byLineAfter[baseDialogue.Line]
			if !beforePresent || !afterPresent || beforeEntry.dialogue.TextStart != baseDialogue.TextStart || beforeEntry.dialogue.Style != baseDialogue.Style || beforeEntry.dialogue.MissingFields || afterDialogue.MissingFields {
				return nil, fmt.Errorf("%s cannot verify the edited Dialogue layout", profile.Kind())
			}
			dialogueEdits := byDialogue[dialogueIndex]
			proofBatchComparisons.Add(1)
			afterTree := prep.afterDialogue(afterDialogue.Text, beforeEntry)
			var comparison semantic.ProofComparison
			if syntaxOnly {
				comparison = semantic.CompareSyntaxProof(beforeEntry.tree, afterTree, profile,
					semantic.EvaluationOptions{Styles: prep.styles, DialogueStyle: beforeEntry.dialogue.Style})
			} else {
				comparison = semantic.CompareResolvedProof(beforeEntry.tree, afterTree, profile,
					semantic.EvaluationOptions{Styles: prep.styles, DialogueStyle: beforeEntry.dialogue.Style},
					removedRanges(beforeEntry.dialogue, dialogueEdits))
			}
			if !comparison.Equivalent {
				return nil, fmt.Errorf("%s dialogue line %d: %s", profile.Kind(), beforeEntry.dialogue.Line, comparison.Reason)
			}
			beforeHash, err := prep.beforeHash(beforeEntry, comparison.Before)
			if err != nil {
				return nil, err
			}
			afterHash, err := prep.afterHash(afterDialogue.Text, comparison.After)
			if err != nil {
				return nil, err
			}
			ranges := dialogueRanges(beforeEntry.dialogue, dialogueEdits)
			target.Dialogues = append(target.Dialogues, ProofDialogue{
				Line:              beforeEntry.dialogue.Line,
				BeforeTraceSHA256: beforeHash, AfterTraceSHA256: afterHash,
				Before: relevantSteps(comparison.Before.Steps, ranges),
				After:  relevantSteps(comparison.After.Steps, ranges),
			})
		}
		target.StyleFields = slices.Clone(profileStyleEvidence)
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

// fixTargetsChecked builds the target list once per batch and re-applies the
// pinned-version and duplicate checks on first use. The list depends only on
// the fixed profile set, so every attempt gets identical target bytes.
func (b *proofBatch) fixTargetsChecked() ([]FixTarget, error) {
	if b.targets != nil {
		return b.targets, nil
	}
	targets := fixTargets(b.profiles)
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
	b.targets = targets
	return targets, nil
}

// prep returns one profile's before view, interpreted Style state, and line
// index, prepared at most once per batch.
func (b *proofBatch) prep(profile renderer.Profile) *proofProfilePrep {
	if prep, ok := b.preps[profile]; ok {
		proofBatchReuse.Add(1)
		return prep
	}
	var view ass.Document
	if seeded, ok := b.seeded[profile]; ok && seeded.Text == b.beforeDoc.Text && len(seeded.Dialogues) == len(b.beforeDoc.Dialogues) {
		view = seeded
		proofBatchReuse.Add(1)
	} else {
		proofBatchViews.Add(1)
		view = profile.DocumentView(b.beforeDoc)
		if len(b.beforeDoc.Lines) == 0 {
			view = b.beforeDoc
		}
	}
	prep := &proofProfilePrep{
		view:   view,
		styles: semantic.StyleStatesByName(view.StyleFields),
		byLine: make(map[int]ass.Dialogue, len(view.Dialogues)),
		before: make(map[int]*proofBeforeDialogue),
		after:  make(map[string]*proofAfterDialogue),
	}
	for _, dialogue := range view.Dialogues {
		prep.byLine[dialogue.Line] = dialogue
	}
	b.preps[profile] = prep
	return prep
}

// beforeDialogue returns one affected dialogue's verified before entry,
// parsed at most once per batch. An unmapped line yields false so the caller
// reports the same layout error as the original loop.
func (p *proofProfilePrep) beforeDialogue(line int) (*proofBeforeDialogue, bool) {
	if entry, ok := p.before[line]; ok {
		return entry, true
	}
	dialogue, ok := p.byLine[line]
	if !ok {
		return nil, false
	}
	proofBatchTrees.Add(1)
	entry := &proofBeforeDialogue{dialogue: dialogue, tree: ass.ParseConcreteDialogue(dialogue.Text)}
	p.before[line] = entry
	return entry, true
}

// afterDialogue returns one after dialogue's parsed tree: the before tree
// when the text is unchanged, else a per-text parse kept once per batch. The
// map belongs to one profile of one request, so no tree outlives the source.
func (p *proofProfilePrep) afterDialogue(text string, before *proofBeforeDialogue) ass.ConcreteDialogue {
	if before != nil && before.dialogue.Text == text {
		return before.tree
	}
	if cached, ok := p.after[text]; ok {
		proofBatchReuse.Add(1)
		return cached.tree
	}
	proofBatchTrees.Add(1)
	tree := ass.ParseConcreteDialogue(text)
	p.after[text] = &proofAfterDialogue{tree: tree}
	return tree
}

// beforeHash returns the accepted comparison's before-trace digest, computed
// once per dialogue per batch: the trace depends only on the pinned profile,
// the interpreted Style state, and the original dialogue text, never on the
// edit set.
func (p *proofProfilePrep) beforeHash(before *proofBeforeDialogue, trace semantic.ProofTrace) (string, error) {
	if before.hash != "" {
		proofBatchReuse.Add(1)
		return before.hash, nil
	}
	proofBatchHashes.Add(1)
	hash, err := hashJSON(trace)
	if err != nil {
		return "", err
	}
	before.hash = hash
	return hash, nil
}

// afterHash returns one after trace's digest, computed once per distinct
// after text per batch for the same reason.
func (p *proofProfilePrep) afterHash(text string, trace semantic.ProofTrace) (string, error) {
	entry := p.after[text]
	if entry == nil {
		entry = &proofAfterDialogue{}
		p.after[text] = entry
	}
	if entry.hash != "" {
		proofBatchReuse.Add(1)
		return entry.hash, nil
	}
	proofBatchHashes.Add(1)
	hash, err := hashJSON(trace)
	if err != nil {
		return "", err
	}
	entry.hash = hash
	return hash, nil
}
