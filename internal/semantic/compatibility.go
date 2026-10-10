package semantic

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/ass/spec"
)

// CompatibilityStatus applies only to the named dimension and source span.
// Even equivalent observations never promise pixel-identical rendering.
type CompatibilityStatus string

const (
	CompatibilityEquivalent CompatibilityStatus = "equivalent"
	CompatibilityDivergent  CompatibilityStatus = "divergent"
	CompatibilityIgnored    CompatibilityStatus = "ignored-or-unsupported"
	CompatibilityUnresolved CompatibilityStatus = "unresolved"
)

// Interpretation retains a detached observation from one independent run.
// Owners are original source spans, never profile-local operation indices.
type Interpretation struct {
	Profile     renderer.Profile
	Resolution  renderer.Result
	Event       TagEvent
	Owners      map[string]ass.ConcreteSpan
	ActiveStyle string
	Present     bool
}

// CompatibilityFinding keeps dimensions separate: one dispatch difference
// must not conceal a distinct application, ownership or evidence failure.
type CompatibilityFinding struct {
	Source      ass.ConcreteSpan
	Dimension   string
	Status      CompatibilityStatus
	Left, Right *Interpretation
	Detail      string
	Citations   []string
}

// CompareDialogue compares every pair of requested profiles. No profiles
// selects the frozen traditional compatibility scope: libass + xy-VSFilter.
// Input options supply Style context; caller observers and Profile are not used.
func CompareDialogue(tree ass.ConcreteDialogue, profiles []renderer.Profile, options EvaluationOptions) []CompatibilityFinding {
	if len(profiles) == 0 {
		libass, _ := renderer.Standard(renderer.Libass)
		xy, _ := renderer.Standard(renderer.XYVSFilter)
		profiles = []renderer.Profile{libass, xy}
	}
	// Duplicate targets cannot produce useful self-comparisons.
	unique := make([]renderer.Profile, 0, len(profiles))
	for _, p := range profiles {
		if !slices.Contains(unique, p) {
			unique = append(unique, p)
		}
	}
	if len(unique) < 2 || !tree.HasCandidates() {
		return nil
	}
	runs := make([]map[ass.ConcreteSpan]*Interpretation, len(unique))
	spans := make(map[ass.ConcreteSpan]bool)
	for i, p := range unique {
		runs[i] = observeInterpretation(tree, p, options)
		for span := range runs[i] {
			spans[span] = true
		}
	}
	order := make([]ass.ConcreteSpan, 0, len(spans))
	for span := range spans {
		order = append(order, span)
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].Start == order[j].Start {
			return order[i].End < order[j].End
		}
		return order[i].Start < order[j].Start
	})
	var out []CompatibilityFinding
	for _, span := range order {
		for i := range unique {
			for j := i + 1; j < len(unique); j++ {
				left, right := observationAt(runs[i], span, unique[i]), observationAt(runs[j], span, unique[j])
				out = append(out, compareInterpretations(span, left, right)...)
			}
		}
	}
	return out
}

func observationAt(run map[ass.ConcreteSpan]*Interpretation, span ass.ConcreteSpan, profile renderer.Profile) *Interpretation {
	if observation := run[span]; observation != nil {
		return observation
	}
	return &Interpretation{Profile: profile}
}

func observeInterpretation(tree ass.ConcreteDialogue, profile renderer.Profile, options EvaluationOptions) map[ass.ConcreteSpan]*Interpretation {
	out := make(map[ass.ConcreteSpan]*Interpretation)
	profile.WalkDialogue(tree, func(r renderer.Result, _ bool) bool {
		out[r.Source] = &Interpretation{Profile: profile, Resolution: r, Present: true}
		return true
	})
	var indexSpans []ass.ConcreteSpan
	options.Profile = nil
	options.Observer = Observer{Tag: func(event TagEvent, state StateView) {
		span := ass.ConcreteSpan{Start: event.Tag.Start, End: event.Tag.End}
		indexSpans = append(indexSpans, span)
		observation := out[span]
		event.Slots = slices.Clone(event.Slots)
		// Nested operations have their own source-aligned observations;
		// retaining recursive child views would duplicate the same subtree.
		event.Tag.Children = nil
		observation.Event = event
		observation.ActiveStyle = state.ActiveStyle()
		if len(event.Slots) > 0 {
			observation.Owners = make(map[string]ass.ConcreteSpan, len(event.Slots))
		}
		for _, slot := range event.Slots {
			if owner := state.Source(slot); owner >= 0 && owner < len(indexSpans) {
				observation.Owners[slot] = indexSpans[owner]
			}
		}
		out[span] = observation
	}}
	EvaluateResolved(tree, profile, options)
	return out
}

func compareInterpretations(span ass.ConcreteSpan, a, b *Interpretation) []CompatibilityFinding {
	out := make([]CompatibilityFinding, 0, 8)
	add := func(dimension string, status CompatibilityStatus, detail string, citations ...string) {
		sources := slices.Clone(citations)
		sources = slices.DeleteFunc(sources, func(s string) bool { return s == "" })
		slices.Sort(sources)
		sources = slices.Compact(sources)
		out = append(out, CompatibilityFinding{Source: span, Dimension: dimension, Status: status, Left: a, Right: b, Detail: detail, Citations: sources})
	}
	if !a.Present || !b.Present {
		add("dispatch", CompatibilityUnresolved, "This source span is visited by only one independently interpreted operation stream.")
		return out
	}
	ar, br := a.Resolution, b.Resolution
	dispatchSources := []string{dispatchCitation(a.Profile), dispatchCitation(b.Profile)}
	dispatch := CompatibilityEquivalent
	if !ar.Closed || !br.Closed || a.Event.Tag.RepeatedSlashes != 0 || b.Event.Tag.RepeatedSlashes != 0 || ar.Status == renderer.Conditional || br.Status == renderer.Conditional {
		dispatch = CompatibilityUnresolved
	} else if ar.Name != br.Name || ar.Status != br.Status {
		dispatch = CompatibilityDivergent
	} else if ar.Status != renderer.Matched {
		dispatch = CompatibilityIgnored
	}
	add("dispatch", dispatch, fmt.Sprintf("%s selects %s; %s selects %s.", a.Profile.Kind(), dispatchDescription(ar), b.Profile.Kind(), dispatchDescription(br)), dispatchSources...)
	if dispatch == CompatibilityUnresolved || dispatch == CompatibilityIgnored {
		return out
	}
	// Inferred shapes and unmodeled parameter scans cannot establish common
	// consumption. Bare suffix differences follow verified dispatch directly.
	args := CompatibilityEquivalent
	if ar.Signature == renderer.SignatureUnknown || br.Signature == renderer.SignatureUnknown || ar.Signature == renderer.SignatureInferred || br.Signature == renderer.SignatureInferred {
		args = CompatibilityUnresolved
	}
	if !slices.Equal(ar.Args, br.Args) && (ar.Form == renderer.Bare && br.Form == renderer.Bare || verifiedSignature(ar.Signature) && verifiedSignature(br.Signature)) {
		args = CompatibilityDivergent
	}
	add("arguments", args, fmt.Sprintf("%s resolves %s; %s resolves %s.", a.Profile.Kind(), argumentDescription(ar), b.Profile.Kind(), argumentDescription(br)), ar.Citation, br.Citation)
	signature := CompatibilityUnresolved
	if verifiedSignature(ar.Signature) && verifiedSignature(br.Signature) {
		signature = CompatibilityEquivalent
		if ar.Signature != br.Signature {
			signature = CompatibilityDivergent
		} else if ar.Signature == renderer.SignatureRejected {
			signature = CompatibilityIgnored
		}
	}
	add("signature", signature, fmt.Sprintf("%s: %s; %s: %s.", a.Profile.Kind(), signatureDescription(ar.Signature), b.Profile.Kind(), signatureDescription(br.Signature)), ar.Citation, br.Citation)
	ae, be := a.Event, b.Event
	// Application and ownership are observations, not broader proof. Require
	// source-verified accepted/rejected forms and certain evaluator transitions.
	certain := verifiedSignature(ar.Signature) && verifiedSignature(br.Signature) && transitionCertain(ae) && transitionCertain(be) && firstWinsEvidence(a) && firstWinsEvidence(b)
	application := CompatibilityUnresolved
	if certain {
		application = CompatibilityEquivalent
		if ae.Applied != be.Applied || ae.Ignored != be.Ignored {
			application = CompatibilityDivergent
		}
	}
	add("application", application, fmt.Sprintf("%s: applied=%t, ignored=%t; %s: applied=%t, ignored=%t.", a.Profile.Kind(), ae.Applied, ae.Ignored, b.Profile.Kind(), be.Applied, be.Ignored), ar.Citation, br.Citation)
	if ae.Policy == spec.FirstWins || be.Policy == spec.FirstWins {
		ownership := CompatibilityUnresolved
		if certain {
			ownership = CompatibilityEquivalent
			if !sameOwners(a.Owners, b.Owners) {
				ownership = CompatibilityDivergent
			}
		}
		add("ownership", ownership, fmt.Sprintf("%s owners: %s; %s owners: %s.", a.Profile.Kind(), ownersDescription(a.Owners), b.Profile.Kind(), ownersDescription(b.Owners)), ar.Citation, br.Citation)
	}
	if ar.Name == br.Name && ar.Status == renderer.Matched && br.Status == renderer.Matched {
		as, bs := candidateScenarios(a), candidateScenarios(b)
		scenarios := append(slices.Clone(as), bs...)
		slices.Sort(scenarios)
		for _, scenario := range slices.Compact(scenarios) {
			av, bv := scenarioEvidence(a, scenario, as), scenarioEvidence(b, scenario, bs)
			status := compareScenarioOutcomes(ar.Name, scenario, av, bv)
			if scenario == "coord-round" && !slices.Equal(ar.Args, br.Args) {
				status = CompatibilityUnresolved
			}
			add("behavior/"+scenario, status, fmt.Sprintf("%s: %s; %s: %s.", a.Profile.Kind(), behaviorDescription(av), b.Profile.Kind(), behaviorDescription(bv)), av.Citation, bv.Citation)
		}
	}
	// Identical tracked slots are insufficient evidence: the engine is partial
	// and shared canonical values are deliberately conservative across profiles.
	stateStatus := CompatibilityUnresolved
	if certain && knownStateDifference(ae, be) {
		stateStatus = CompatibilityDivergent
	}
	add("state", stateStatus, fmt.Sprintf("%s: %s, active Style %q; %s: %s, active Style %q. Tracked state alone cannot prove rendering compatibility.", a.Profile.Kind(), stateDescription(ae), a.ActiveStyle, b.Profile.Kind(), stateDescription(be), b.ActiveStyle))
	return out
}

func firstWinsEvidence(o *Interpretation) bool {
	if o.Resolution.Signature == renderer.SignatureRejected || o.Event.Policy != spec.FirstWins {
		return true
	}
	return o.Profile.Behavior(o.Resolution.Name, "repeat").Verified
}

func knownStateDifference(a, b TagEvent) bool {
	for i, slot := range a.Slots {
		j := slices.Index(b.Slots, slot)
		if i < len(a.After) && j >= 0 && j < len(b.After) && a.After[i].Known && b.After[j].Known && a.After[i] != b.After[j] {
			return true
		}
	}
	return false
}

func verifiedSignature(s renderer.SignatureStatus) bool {
	return s == renderer.SignatureVerified || s == renderer.SignatureRejected
}
func transitionCertain(e TagEvent) bool {
	return !e.Barrier && (e.Uncertainty == UncertaintyNone || e.Ignored && e.Signature == renderer.SignatureRejected)
}
func sameOwners(a, b map[string]ass.ConcreteSpan) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if other, ok := b[k]; !ok || v != other {
			return false
		}
	}
	return true
}
func ownersDescription(owners map[string]ass.ConcreteSpan) string {
	keys := make([]string, 0, len(owners))
	for key := range owners {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		span := owners[key]
		parts = append(parts, fmt.Sprintf("%s=[%d,%d)", key, span.Start, span.End))
	}
	if len(parts) == 0 {
		return "none proven"
	}
	return strings.Join(parts, ", ")
}
func dispatchDescription(r renderer.Result) string {
	statuses := [...]string{"unknown name", "matched", "build-conditional", "disabled", "ignored"}
	name := r.Name
	if name == "" {
		name = r.Head
	}
	detail := fmt.Sprintf("\\%s (%s)", name, statuses[r.Status])
	if r.Fallback != "" {
		detail += ", build-off fallback \\" + r.Fallback
	}
	if r.Shadowed != "" {
		detail += ", shadows \\" + r.Shadowed
	}
	return detail
}
func argumentDescription(r renderer.Result) string {
	parts := make([]string, 0, len(r.Args))
	for _, arg := range r.Args {
		parts = append(parts, fmt.Sprintf("%q at [%d,%d)", arg.Raw, arg.Span.Start, arg.Span.End))
	}
	return fmt.Sprintf("%d argument(s): %s", len(parts), strings.Join(parts, ", "))
}
func signatureDescription(s renderer.SignatureStatus) string {
	return [...]string{"unresolved argument shape", "verified acceptance", "inferred acceptance (unresolved)", "verified rejection"}[s]
}
func behaviorDescription(e renderer.BehaviorEvidence) string {
	if !e.Verified {
		return "unresolved (no verified scenario outcome)"
	}
	return e.Outcome
}
func stateDescription(e TagEvent) string {
	parts := make([]string, 0, len(e.Slots))
	for i, slot := range e.Slots {
		if i >= len(e.After) {
			break
		}
		value := e.After[i]
		if value.Known {
			parts = append(parts, slot+"="+value.Value)
		} else {
			parts = append(parts, slot+"=unresolved")
		}
	}
	if len(parts) == 0 {
		return "no modeled slot transition"
	}
	return strings.Join(parts, ", ")
}
func dispatchCitation(p renderer.Profile) string {
	switch p.Kind() {
	case renderer.Libass:
		return "libass@" + p.Version() + ":libass/ass_parse.c:68-78,355-916"
	case renderer.XYVSFilter:
		return "xy-VSFilter@" + p.Version() + ":src/subtitles/RTS.cpp:1716-1769,2155-2163"
	default:
		return "VSFilterMod@" + p.Version() + ":src/subtitles/RTS.cpp:2524-2669"
	}
}
