package semantic

import (
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

var compactCorpus = []string{
	`plain text`,
	`{\b1}bold {\b0}off`,
	`{\bord1\shad2}alias {\bord0}off`,
	`{\alpha&H0000FF&}four alphas`,
	`{\pos(1,2)}latched {\pos(3,4)}ignored`,
	`{\an7}first {\an5}second`,
	`{\fnTest}named {\r}reset {\rMissing}unknown name`,
	`{\fscx120}scaled`,
	`{\t(0,100,\fscx150)}transform`,
	`{\t(0,100,\blur5)}invalid transform child`,
	`{\p1}m1{\p0}m0`,
	`{\kf10}ka{\k20}ra`,
	`{\kt100}absolute kt`,
	`{\fn}{\fnarial}empty then named`,
	`{\be1}{\be0}blur {\blur101}ambiguous`,
	`{\c&HFFFFFF&}color {\1a&H55&}alpha`,
	`{\clip(1,2,30,40)}rect {\clip(0)}vector`,
	`{\pos(1,2)}{\move(1,2,3,4,5,6)}same latch`,
	`{\frz45}rot {\fr30}same slot`,
	`{q1}wrap {\p2}{\q0}mixed`,
	`{\i1\s3}box`,
	`{\fad(10,20)}{\fade(10,200,300,400,500,600,700)}arities`,
	`{\an0}malformed latch`,
	`{\pos(}unclosed`,
	`{\move(1,2,3,4,5}wrong arity`,
	`{\fn}text{\fnArial}font{\fn}reset`,
	`{\b1}x{\fscx50}y{\b1}z`,
	`{\\\\b1}escaped`,
	`{\r}at boundary`,
	`text before {\blur4}after`,
	`{\fscx120}{\fscy120}both scales`,
	`{\1a&H00&}{\2a&HFF&}{\3a&H7F&}{\4a&H00&}alphas`,
	`{\p1}{\p0}drawing off`,
	`{\kf0}zero karaoke`,
	`{\t(0,100,\b1)}transform with tag child`,
}

func compactCorpusStyles() map[string]StyleState {
	fields := []ass.StyleField{
		{StyleName: "Default", Name: "Fontname", Value: "Arial"},
		{StyleName: "Default", Name: "Bold", Value: "-1"},
		{StyleName: "Default", Name: "Fontsize", Value: "48"},
		{StyleName: "Default", Name: "PrimaryColour", Value: "&H00FFFFFF"},
		{StyleName: "Default", Name: "Outline", Value: "3"},
		{StyleName: "Default", Name: "ScaleX", Value: "120"},
		{StyleName: "Test", Name: "Fontname", Value: "Times"},
		{StyleName: "Test", Name: "Italic", Value: "-1"},
	}
	return StyleStatesByName(fields)
}

func compactLibassProfile(t *testing.T) renderer.Profile {
	t.Helper()
	profile, err := renderer.Standard(renderer.Libass)
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

// referenceDecision runs the wire pipeline that shipped before the compact
// traces: full map-based ProofSteps. It is the differential oracle.
func referenceDecision(before, after ass.ConcreteDialogue, profile renderer.Profile, options EvaluationOptions, removed []ass.ConcreteSpan) (bool, string) {
	comparison := referenceCompareResolvedProof(before, after, profile, options, removed)
	return comparison.Equivalent, comparison.Reason
}

// compactEdits are candidate SafeFix rewrites: removed blocks plus one
// same-length substitution each, so spans on both sides stay realistic.
func compactEdits(source string) (string, []ass.ConcreteSpan) {
	type edit struct {
		at       int
		from, to string
	}
	candidates := []edit{
		{from: `{\b0}`, to: `{}`}, {from: `{\pos(3,4)}`, to: `{}`}, {from: `{\an5}`, to: `{}`},
		{from: `{\fscx120}`, to: `{}`}, {from: `{\be0}`, to: `{}`}, {from: `{\p0}`, to: `{}`},
		{from: `{\rMissing}`, to: `{}`}, {from: `{\blur101}`, to: `{}`}, {from: `{\fnarial}`, to: `{}`},
		{from: `{\1a&H55&}`, to: `{}`}, {from: `{\fade(10,200,300,400,500,600,700)}`, to: `{}`},
		{from: `{\fr30}`, to: `{}`}, {from: `{\q0}`, to: `{}`}, {from: `{\s3}`, to: `{}`},
	}
	var found []edit
	marked := source
	for _, candidate := range candidates {
		index := strings.Index(marked, candidate.from)
		if index < 0 {
			continue
		}
		candidate.at = index
		found = append(found, candidate)
		marked = marked[:index] + strings.Repeat("\x00", len(candidate.from)) + marked[index+len(candidate.from):]
	}
	if len(found) == 0 {
		return "", nil
	}
	sort.Slice(found, func(i, j int) bool { return found[i].at > found[j].at })
	edited := source
	removed := make([]ass.ConcreteSpan, 0, len(found))
	for _, f := range found {
		removed = append(removed, ass.ConcreteSpan{Start: f.at, End: f.at + len(f.from)})
		edited = edited[:f.at] + f.to + edited[f.at+len(f.from):]
	}
	slices.Reverse(removed)
	return edited, removed
}

func TestCompactCompareMatchesFullReference(t *testing.T) {
	profile := compactLibassProfile(t)
	options := EvaluationOptions{Styles: compactCorpusStyles(), DialogueStyle: "Default"}
	checked := 0
	for _, source := range compactCorpus {
		before := ass.ParseConcreteDialogue(source)
		edited, removed := compactEdits(source)
		afterSources := []string{source}
		if edited != "" {
			afterSources = append(afterSources, edited)
		}
		for _, afterSource := range afterSources {
			after := ass.ParseConcreteDialogue(afterSource)
			wantEquivalent, wantReason := referenceDecision(before, after, profile, options, removed)
			gotEquivalent, gotReason := decideCompact(
				observeCompactDialogue(before, profile, options),
				observeCompactDialogue(after, profile, options), removed)
			if gotEquivalent != wantEquivalent || gotReason != wantReason {
				t.Errorf("decision for %q -> %q: compact=(%t,%q), reference=(%t,%q)",
					source, afterSource, gotEquivalent, gotReason, wantEquivalent, wantReason)
				continue
			}
			checked++
			if !wantEquivalent {
				continue
			}
			// Accepted comparisons must expose the same audit wire as the
			// map pipeline: full state snapshots for every retained step.
			beforeTrace := observeCompactDialogue(before, profile, options)
			afterTrace := observeCompactDialogue(after, profile, options)
			wantBefore := observeProofTrace(before, profile, options)
			wantAfter := observeProofTrace(after, profile, options)
			if !reflect.DeepEqual(beforeTrace.wire().Steps, wantBefore.Steps) {
				t.Errorf("materialized before wire differs for %q", source)
			}
			if !reflect.DeepEqual(afterTrace.wire().Steps, wantAfter.Steps) {
				t.Errorf("materialized after wire differs for %q", source)
			}
		}
	}
	if checked < len(compactCorpus) {
		t.Fatalf("corpus exercised only %d of %d comparisons", checked, len(compactCorpus))
	}
}

func TestCompactGateRejectsWithoutMaterializing(t *testing.T) {
	profile := compactLibassProfile(t)
	options := EvaluationOptions{Styles: compactCorpusStyles(), DialogueStyle: "Default"}
	for _, source := range []string{
		`{\pos(}unclosed`, `{bad\an0}malformed`, `{\blur101}ambiguous`,
		`{\lua}unknown extension`, `{\t(1,2,\lua)}barrier child`,
	} {
		dialogue := ass.ParseConcreteDialogue(source)
		comparison := CompareResolvedProof(dialogue, dialogue, profile, options, nil)
		wantEquivalent, wantReason := referenceDecision(dialogue, dialogue, profile, options, nil)
		if comparison.Equivalent != wantEquivalent || comparison.Reason != wantReason {
			t.Errorf("%q gate: got (%t,%q), want (%t,%q)", source, comparison.Equivalent, comparison.Reason, wantEquivalent, wantReason)
		}
		if !comparison.Equivalent && len(comparison.Before.Steps) != 0 {
			t.Errorf("%q rejected comparison materialized a before trace", source)
		}
	}
}

func TestCompactStateEqualityCoversSnapshotRules(t *testing.T) {
	profile := compactLibassProfile(t)
	options := EvaluationOptions{Styles: compactCorpusStyles(), DialogueStyle: "Default"}
	trace := observeCompactDialogue(ass.ParseConcreteDialogue(`{\b1}x{\i1}y`), profile, options)
	if len(trace.steps) != 4 {
		t.Fatalf("step count = %d, want tag/text/tag/text", len(trace.steps))
	}
	first, second := trace.steps[1].state, trace.steps[3].state
	if stateEqualStrict(first, second) {
		t.Error("distinct assignments must not merge as text")
	}
	wireWire := materializeCompactState(first)
	want := observeProofTrace(ass.ParseConcreteDialogue(`{\b1}x{\i1}y`), profile, options).Steps[1].State
	if !reflect.DeepEqual(wireWire, want) {
		t.Fatalf("materialized snapshot = %+v, want %+v", wireWire, want)
	}
}

func TestCompactMissingOwnerAsymmetry(t *testing.T) {
	a := compactState{style: "Default", values: []compactEntry{{slot: "bold", value: KnownValue("1")}}, owners: []compactOwner{{slot: "bold", index: -1}}}
	b := compactState{style: "Default", values: []compactEntry{{slot: "bold", value: KnownValue("1")}}}
	if !stateEqualProof(a, b) {
		t.Error("an explicit -1 witness on the left with no key on the right must be equivalent")
	}
	if stateEqualProof(b, a) {
		t.Error("a provenance key present only on the right must not be equivalent")
	}
	c := compactState{style: "Default", values: []compactEntry{{slot: "bold", value: KnownValue("1")}}, owners: []compactOwner{{slot: "bold", index: 2}}}
	d := compactState{style: "Default", values: []compactEntry{{slot: "bold", value: KnownValue("1")}}, owners: []compactOwner{{slot: "bold", index: 3}}}
	if stateEqualProof(c, d) {
		t.Error("different owner ordinals must not be equivalent")
	}
	if stateEqualStrict(c, d) || !stateEqualStrict(c, c) {
		t.Error("strict equality must separate different ordinals and keep identical ones")
	}
	if stateEqualStrict(a, b) {
		t.Error("strict equality keeps the -1 witness distinct from absence")
	}
	e := compactState{style: "Default", values: []compactEntry{{slot: "bold", value: KnownValue("1")}, {slot: "italic", value: UnknownValue()}}}
	f := compactState{style: "Default", values: []compactEntry{{slot: "italic", value: UnknownValue()}, {slot: "bold", value: KnownValue("1")}}}
	if stateEqualProof(e, f) {
		t.Error("unordered values must not compare equal; capture order is canonical")
	}
}

func TestCompactRetainAndNormalizeMatchReference(t *testing.T) {
	profile := compactLibassProfile(t)
	options := EvaluationOptions{Styles: compactCorpusStyles(), DialogueStyle: "Default"}
	source := `{\pos(1,2)}{\pos(3,4)}{\b1}text`
	removed := []ass.ConcreteSpan{{Start: 0, End: 9}}
	before := ass.ParseConcreteDialogue(source)
	reference := normalizeProofSources(retainProofSteps(observeProofTrace(before, profile, options).Steps, removed))
	trace := observeCompactDialogue(before, profile, options)
	compact := mergeCompact(normalizeCompact(retainCompact(trace.steps, removed)))
	if len(reference) != len(compact) {
		t.Fatalf("reference pipeline has %d steps, compact has %d", len(reference), len(compact))
	}
	for i := range reference {
		materialized := materializeCompactStep(compact[i])
		if !reflect.DeepEqual(reference[i].State, materialized.State) {
			t.Errorf("step %d normalized state reference=%+v compact=%+v", i, reference[i].State, materialized.State)
		}
		if reference[i].Kind == "tag" && reference[i].Index != 0 && materialized.Kind == "tag" {
			if reference[i].Tag != materialized.Tag {
				t.Errorf("step %d tag name reference=%q compact=%q", i, reference[i].Tag, materialized.Tag)
			}
		}
	}
}

func TestCompactSyntaxProofMatchesReference(t *testing.T) {
	profile := compactLibassProfile(t)
	options := EvaluationOptions{Styles: compactCorpusStyles(), DialogueStyle: "Default"}
	pairs := [][2]string{
		{`{\b1}x{}`, `{\b1}x`},
		{`{}{\b1}x`, `{\b1}x`},
		{`x{}y`, `xy`},
		{`{\b1}{}x`, `{\b1}x`},
		{`{}` + "\n" + `{}`, ""},
	}
	for _, pair := range pairs {
		before := ass.ParseConcreteDialogue(pair[0])
		after := ass.ParseConcreteDialogue(pair[1])
		want := referenceCompareSyntaxProof(before, after, profile, options)
		got := CompareSyntaxProof(before, after, profile, options)
		if got.Equivalent != want.Equivalent || got.Reason != want.Reason {
			t.Errorf("syntax proof %q -> %q: got (%t,%q), want (%t,%q)",
				pair[0], pair[1], got.Equivalent, got.Reason, want.Equivalent, want.Reason)
		}
	}
}
