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
	program    *program
	operations map[int]resolvedOperation
	entries    []resolvedEntry
}

// PrepareDialogue resolves every expression in the tree under the profile.
// A canceled context returns no prepared value.
func PrepareDialogue(ctx context.Context, tree ass.ConcreteDialogue, profile renderer.Profile) (*PreparedDialogue, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return prepareDialogue(tree, profile, buildProgram(tree)), nil
}

func prepareDialogue(tree ass.ConcreteDialogue, profile renderer.Profile, program *program) *PreparedDialogue {
	operations, entries := resolveOperations(tree, profile, true)
	return &PreparedDialogue{tree: tree, profile: profile, program: program, operations: operations, entries: entries}
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
	if profile.ModStyleTransform() {
		options.Styles = modStyleStates(options.Styles)
	}
	return evaluateProgram(prepared.program, profile, prepared.operations, options)
}

// DialoguePreparations caches prepared dialogue operations for one analysis
// request. Validation, compatibility, and repeated evaluations of equal
// dialogue text under exact profiles share one resolution walk per pair.
// A nil cache is valid and resolves every request independently.
type DialoguePreparations struct {
	byProfile map[renderer.Profile]map[string]*PreparedDialogue
	programs  map[string]*program
}

func NewDialoguePreparations() *DialoguePreparations {
	return &DialoguePreparations{
		byProfile: make(map[renderer.Profile]map[string]*PreparedDialogue),
		programs:  make(map[string]*program),
	}
}

// Prepare returns operations resolved from the given tree under the profile,
// reusing an earlier value for equal source text.
func (d *DialoguePreparations) Prepare(profile renderer.Profile, tree ass.ConcreteDialogue) *PreparedDialogue {
	if prepared := d.lookup(profile, tree.Source); prepared != nil {
		return prepared
	}
	prepared := prepareDialogue(tree, profile, d.programFor(tree.Source, tree))
	d.store(profile, tree.Source, prepared)
	return prepared
}

// PrepareText returns operations resolved from a fresh parse of text under
// the profile, reusing an earlier value for equal text.
func (d *DialoguePreparations) PrepareText(profile renderer.Profile, text string) *PreparedDialogue {
	if prepared := d.lookup(profile, text); prepared != nil {
		return prepared
	}
	tree := ass.ParseConcreteDialogue(text)
	prepared := prepareDialogue(tree, profile, d.programFor(text, tree))
	d.store(profile, text, prepared)
	return prepared
}

func (d *DialoguePreparations) lookup(profile renderer.Profile, text string) *PreparedDialogue {
	if d == nil {
		return nil
	}
	return d.byProfile[profile][text]
}

// programFor returns the shared neutral program for one dialogue text. Every
// profile that prepares the same text walks the same token sequence.
func (d *DialoguePreparations) programFor(text string, tree ass.ConcreteDialogue) *program {
	if d == nil {
		return buildProgram(tree)
	}
	if built := d.programs[text]; built != nil {
		return built
	}
	built := buildProgram(tree)
	d.programs[text] = built
	return built
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

// EvaluatePreparedSet folds one neutral program once with one state per
// prepared dialogue. The result reports false when the entries do not share a
// program, for example after a projected text rewrite; the caller then keeps
// separate evaluations.
func EvaluatePreparedSet(prepared []*PreparedDialogue, options []EvaluationOptions) ([]Evaluation, bool) {
	if len(prepared) == 0 || len(options) != len(prepared) {
		return nil, false
	}
	for _, entry := range prepared[1:] {
		if entry.program != prepared[0].program {
			return nil, false
		}
	}
	if options[0].Observer.Text == nil && !prepared[0].tree.HasCandidates() {
		return make([]Evaluation, len(prepared)), true
	}
	states := make([]evaluationState, len(prepared))
	for i := range prepared {
		states[i] = evaluationState{profile: prepared[i].profile, operations: prepared[i].operations, options: options[i]}
	}
	return evaluateProgramSet(prepared[0].program, states), true
}
