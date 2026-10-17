package semantic

import (
	"context"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

// PreparedDialogue resolves one dialogue tree once for one exact renderer
// profile. Its operations feed validation, compatibility, and evaluation
// without a second resolution walk. The value belongs to one concrete tree:
// projected text, Style layouts, and edited sources each need their own
// prepared dialogue.
type PreparedDialogue struct {
	tree       ass.ConcreteDialogue
	profile    renderer.Profile
	tokens     []semanticToken
	operations map[int]resolvedOperation
	entries    []resolvedEntry
}

// PrepareDialogue resolves every expression in the tree under the profile.
// A canceled context returns no prepared value.
func PrepareDialogue(ctx context.Context, tree ass.ConcreteDialogue, profile renderer.Profile) (*PreparedDialogue, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tokens, operations, entries := operationsFromTokens(tree, profile, true)
	return &PreparedDialogue{
		tree:       tree,
		profile:    profile,
		tokens:     tokens,
		operations: operations,
		entries:    entries,
	}, nil
}

// Evaluate runs the state engine over the prepared operations under the
// profile used for preparation. One dialogue is the atomic cancellation unit;
// request loops check their context between dialogues.
func (p *PreparedDialogue) Evaluate(options EvaluationOptions) Evaluation {
	return evaluatePrepared(p, options)
}

// Tree returns the concrete dialogue the prepared operations belong to.
func (p *PreparedDialogue) Tree() ass.ConcreteDialogue { return p.tree }

// Profile returns the exact renderer profile used for resolution.
func (p *PreparedDialogue) Profile() renderer.Profile { return p.profile }

// EachOperation replays the emission order of renderer.Profile.WalkDialogue.
// Results and argument slices are shared read-only state: consumers that
// rewrite spans in place must clone first. Returning false stops the replay.
func (p *PreparedDialogue) EachOperation(visit func(renderer.Result, bool) bool) {
	for _, entry := range p.entries {
		if !visit(entry.result, entry.nested) {
			return
		}
	}
}

// EvaluatePrepared runs the state engine over prepared resolution output.
// One dialogue is the atomic unit: the context is checked at entry only, and
// a canceled evaluation returns no partial observation.
func EvaluatePrepared(ctx context.Context, prepared *PreparedDialogue, options EvaluationOptions) (Evaluation, error) {
	if err := ctx.Err(); err != nil {
		return Evaluation{}, err
	}
	return evaluatePrepared(prepared, options), nil
}

// evaluatePrepared is the context-free core for callers that already check
// cancellation at their request boundaries.
func evaluatePrepared(prepared *PreparedDialogue, options EvaluationOptions) Evaluation {
	if options.Observer.Text == nil && !prepared.tree.HasCandidates() {
		return Evaluation{}
	}
	profile := prepared.profile
	if profile.Kind() == renderer.VSFilterMod && profile.Build().Mod == renderer.FeatureEnabled {
		options.Styles = modStyleStates(options.Styles)
	}
	return evaluateResolvedCore(prepared.tokens, prepared.operations, profile, options)
}

// DialoguePreparations caches prepared dialogue operations for one analysis
// request. Validation, compatibility, and repeated evaluations of equal
// dialogue text under exact profiles share one resolution walk per pair.
// A nil cache is valid and resolves every request independently.
type DialoguePreparations struct {
	byProfile map[renderer.Profile]map[string]*PreparedDialogue
}

func NewDialoguePreparations() *DialoguePreparations {
	return &DialoguePreparations{byProfile: make(map[renderer.Profile]map[string]*PreparedDialogue)}
}

// Prepare returns operations resolved from the given tree under the profile,
// reusing an earlier value for equal source text.
func (d *DialoguePreparations) Prepare(profile renderer.Profile, tree ass.ConcreteDialogue) *PreparedDialogue {
	if prepared := d.lookup(profile, tree.Source); prepared != nil {
		return prepared
	}
	prepared, _ := PrepareDialogue(context.Background(), tree, profile)
	d.store(profile, tree.Source, prepared)
	return prepared
}

// PrepareText returns operations resolved from a fresh parse of text under
// the profile, reusing an earlier value for equal text.
func (d *DialoguePreparations) PrepareText(profile renderer.Profile, text string) *PreparedDialogue {
	if prepared := d.lookup(profile, text); prepared != nil {
		return prepared
	}
	prepared, _ := PrepareDialogue(context.Background(), ass.ParseConcreteDialogue(text), profile)
	d.store(profile, text, prepared)
	return prepared
}

func (d *DialoguePreparations) lookup(profile renderer.Profile, text string) *PreparedDialogue {
	if d == nil {
		return nil
	}
	return d.byProfile[profile][text]
}

func (d *DialoguePreparations) store(profile renderer.Profile, text string, prepared *PreparedDialogue) {
	if d == nil {
		return
	}
	byText := d.byProfile[profile]
	if byText == nil {
		byText = make(map[string]*PreparedDialogue)
		d.byProfile[profile] = byText
	}
	byText[text] = prepared
}
