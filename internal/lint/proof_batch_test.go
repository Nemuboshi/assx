package lint

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/perftest"
)

// instrumentProofBuilder counts proof builds through the seam for one test.
func instrumentProofBuilder(t *testing.T) *int {
	t.Helper()
	calls := 0
	old := proofBuilder
	proofBuilder = func(batch *proofBatch, edits []FixEditProof, validEdits map[FixEditProof]struct{}) (*FixProof, error) {
		calls++
		return old(batch, edits, validEdits)
	}
	t.Cleanup(func() { proofBuilder = old })
	return &calls
}

// scalingCase returns one perftest scaling workload by name.
func scalingCase(t *testing.T, name string) string {
	t.Helper()
	for _, entry := range perftest.Scaling(t) {
		if entry.Name == name {
			return entry.Text
		}
	}
	t.Fatalf("perftest scaling case %s not found", name)
	return ""
}

func countSafeProved(diagnostics []Diagnostic) int {
	var safe int
	for _, diagnostic := range diagnostics {
		if diagnostic.FixSafety == SafeFix && diagnostic.FixProof != nil {
			safe++
		}
	}
	return safe
}

// proofGroups returns each proof ID's inline proof and every edit set that
// references it, in finding order. Findings share one aggregate proof through
// the reference, so a group collects both forms.
func proofGroups(findings []Diagnostic) (map[string]*FixProof, map[string][]FixEditProof) {
	proofs := map[string]*FixProof{}
	indexes := map[string][]int{}
	for i, diagnostic := range findings {
		if diagnostic.FixSafety != SafeFix || len(diagnostic.Edits) == 0 {
			continue
		}
		id := diagnostic.FixProofRef
		if diagnostic.FixProof != nil {
			id = diagnostic.FixProof.ID
			proofs[id] = diagnostic.FixProof
		}
		if id == "" {
			continue
		}
		indexes[id] = append(indexes[id], i)
	}
	edits := make(map[string][]FixEditProof, len(indexes))
	for id, group := range indexes {
		edits[id] = fixEditsForDiagnostics(findings, group)
	}
	return proofs, edits
}

func marshalJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// threeProfileTargets adds a VSFilterMod build beside the default targets.
func threeProfileTargets(t *testing.T) AnalysisTargets {
	t.Helper()
	mod, err := renderer.New(renderer.VSFilterMod, renderer.Build{Mod: renderer.FeatureEnabled, Lua: renderer.FeatureDisabled})
	if err != nil {
		t.Fatal(err)
	}
	defaults := DefaultAnalysisTargets()
	return AnalysisTargets{Primary: defaults.Primary, Compatibility: append(append([]renderer.Profile{}, defaults.Compatibility...), mod)}
}

// rebuildOneOff reproduces one proof from a fresh single-use batch.
func rebuildOneOff(t *testing.T, doc ass.Document, profiles []renderer.Profile, edits []FixEditProof, validEdits map[FixEditProof]struct{}) *FixProof {
	t.Helper()
	proof, err := newProofBatch(doc.Text, doc, profiles).build(edits, validEdits)
	if err != nil {
		t.Fatalf("one-off rebuild: %v", err)
	}
	return proof
}

// TestProofBatchStyleEditSharesOneAggregateProof covers one batch with many
// dialogue candidates and one aggregate proof reference group: the Style edit
// selects every dialogue, so each profile compares the whole document once
// and all findings share the single proof.
func TestProofBatchStyleEditSharesOneAggregateProof(t *testing.T) {
	doc := ass.Parse(perftest.StyleEdit(t))
	targets := DefaultAnalysisTargets()
	calls := instrumentProofBuilder(t)
	resetProofBatchCounters()
	findings := AnalyzeDocument(doc)
	attempts, views, comparisons, reuse := *calls, proofBatchViews.Load(), proofBatchComparisons.Load(), proofBatchReuse.Load()

	if attempts != 1 {
		t.Fatalf("proof attempts = %d, want one aggregate build", attempts)
	}
	proved := countSafeProved(findings)
	want := perftest.StyleEditDialogueCount*3 + 1
	if proved != want {
		t.Fatalf("proved = %d, want %d", proved, want)
	}
	proofs, edits := proofGroups(findings)
	if len(proofs) != 1 {
		t.Fatalf("aggregate groups = %d, want 1", len(proofs))
	}
	if comparisons != int64(perftest.StyleEditDialogueCount*len(targets.Profiles())) {
		t.Errorf("comparisons = %d, want %d (Style work stays document-wide)",
			comparisons, perftest.StyleEditDialogueCount*len(targets.Profiles()))
	}
	if views != int64(len(targets.Profiles())) {
		t.Errorf("before-view preparations = %d, want %d", views, len(targets.Profiles()))
	}
	if reuse == 0 {
		t.Error("no before work was reused inside the aggregate build")
	}
	for id, proof := range proofs {
		if len(proof.Interpretations) != len(targets.Profiles()) {
			t.Fatalf("proof %s interpretations = %d, want %d", id, len(proof.Interpretations), len(targets.Profiles()))
		}
		for _, interpretation := range proof.Interpretations {
			if len(interpretation.Dialogues) != perftest.StyleEditDialogueCount {
				t.Fatalf("proof %s interpretation %s covers %d dialogues, want %d",
					id, interpretation.Target.Renderer, len(interpretation.Dialogues), perftest.StyleEditDialogueCount)
			}
		}
		rebuilt := rebuildOneOff(t, doc, targets.Profiles(), edits[id], fixEditIndex(findings))
		if got, want := marshalJSON(t, rebuilt), marshalJSON(t, proof); got != want {
			t.Fatalf("one-off rebuild of %s differs from the batch proof:\n%s\n%s", id, got, want)
		}
	}

	fixed, count, err := ApplyFixes(doc.Text, findings, false)
	if err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("applied %d fixes, want %d", count, want)
	}
	if strings.Contains(fixed, ",2,0.0,0,0,1") {
		t.Error("the Style integer edit was not applied")
	}
}

// TestProofBatchFallbackReusesBeforeWork drives the fallback schedule over a
// partially provable document: the aggregate build fails, the halving queue
// runs to its limit, and every attempt shares one batch's before work.
func TestProofBatchFallbackReusesBeforeWork(t *testing.T) {
	const dialogues = 250
	doc := ass.Parse(scalingCase(t, "scaling_250"))
	targets := DefaultAnalysisTargets()
	calls := instrumentProofBuilder(t)
	resetProofBatchCounters()
	first := AnalyzeDocument(doc)
	attempts, views, trees, hashes, reuse := *calls, proofBatchViews.Load(), proofBatchTrees.Load(), proofBatchHashes.Load(), proofBatchReuse.Load()

	if attempts < 2 {
		t.Fatalf("proof attempts = %d, want the fallback schedule to halve a failing batch", attempts)
	}
	proved := countSafeProved(first)
	if proved == 0 || proved >= 3*dialogues {
		t.Fatalf("proved = %d of a partially provable document", proved)
	}
	proofs, edits := proofGroups(first)
	if len(proofs) < 2 {
		t.Fatalf("fallback groups = %d, want independent groups from halving", len(proofs))
	}
	for _, diagnostic := range first {
		if diagnostic.FixSafety == SafeFix && (len(diagnostic.Edits) == 0 || (diagnostic.FixProof == nil && diagnostic.FixProofRef == "")) {
			t.Fatalf("proved finding without evidence: %#v", diagnostic)
		}
	}
	if views != int64(len(targets.Profiles())) {
		t.Errorf("before-view preparations = %d for %d attempts, want %d", views, attempts, len(targets.Profiles()))
	}
	if limit := int64(2 * dialogues * len(targets.Profiles())); trees > limit {
		t.Errorf("dialogue parses = %d, want at most %d", trees, limit)
	}
	if limit := int64(4 * dialogues * len(targets.Profiles())); hashes > limit {
		t.Errorf("trace digests = %d, want at most %d", hashes, limit)
	}
	if reuse == 0 {
		t.Error("no before work was reused across fallback attempts")
	}

	// Every group is exactly reproducible by a single-use build, so the
	// batch changed no proof byte.
	validEdits := fixEditIndex(first)
	for id, proof := range proofs {
		rebuilt := rebuildOneOff(t, doc, targets.Profiles(), edits[id], validEdits)
		if got, want := marshalJSON(t, rebuilt), marshalJSON(t, proof); got != want {
			t.Fatalf("one-off rebuild of %s differs from the batch proof:\n%s\n%s", id, got, want)
		}
	}

	second := AnalyzeDocument(doc)
	if got, want := marshalJSON(t, second), marshalJSON(t, first); got != want {
		t.Fatal("fallback run changed bytes between identical runs")
	}
}

// TestProofBatchHashesSourceOnce drives the fallback schedule and checks that
// the batch hashes its source once. Every attempt's proof carries the same
// source digest, so a later attempt must not repeat that hash over the source.
func TestProofBatchHashesSourceOnce(t *testing.T) {
	doc := ass.Parse(scalingCase(t, "scaling_250"))
	calls := instrumentProofBuilder(t)
	resetProofBatchCounters()
	AnalyzeDocument(doc)
	if *calls < 2 {
		t.Fatalf("proof attempts = %d, want the fallback schedule to halve a failing batch", *calls)
	}
	if hashes := proofBatchSourceHashes.Load(); hashes != 1 {
		t.Fatalf("source digests = %d for %d proof attempts, want 1", hashes, *calls)
	}
}

// TestProofBatchFallbackGroupsApplyAndReject applies every independent group
// in one write and rejects stale sources, changed evidence, and changed
// targets.
func TestProofBatchFallbackGroupsApplyAndReject(t *testing.T) {
	doc := ass.Parse(scalingCase(t, "scaling_250"))
	findings := AnalyzeDocument(doc)
	proved := countSafeProved(findings)
	if proved == 0 {
		t.Fatal("fixture proved nothing")
	}
	fixed, count, err := ApplyFixes(doc.Text, findings, false)
	if err != nil {
		t.Fatalf("apply across independent proof groups: %v", err)
	}
	if count != proved {
		t.Fatalf("applied %d fixes, want %d", count, proved)
	}
	if fixed == doc.Text {
		t.Fatal("application changed no bytes")
	}
	if _, _, err := ApplyFixes(doc.Text+"\n", findings, false); err == nil {
		t.Error("stale source accepted SafeFix proofs")
	}
	if _, _, err := ApplyFixesForTargets(doc.Text, findings, false, threeProfileTargets(t)); err == nil {
		t.Error("changed renderer targets accepted SafeFix proofs")
	}

	tampered := cloneDiagnosticsForTest(findings)
	changed := false
	for i := range tampered {
		if tampered[i].FixProof != nil {
			tampered[i].FixProof.SourceSHA256 = strings.Repeat("0", 64)
			changed = true
		}
	}
	if !changed {
		t.Fatal("no inline proof to tamper")
	}
	if _, _, err := ApplyFixes(doc.Text, tampered, false); err == nil {
		t.Error("changed proof evidence accepted")
	}
}

// TestSessionProofBatchUsesDiscoveryViews requires the session's proof batch
// to reuse the views discovery prepared, so proving prepares no projection.
func TestSessionProofBatchUsesDiscoveryViews(t *testing.T) {
	doc := ass.Parse(benchmarkLateProofFailure(20))
	session := discoverTestSession(t, doc, nil)
	handles := allCandidateHandles(session)
	calls := instrumentProofBuilder(t)
	resetProofBatchCounters()
	first, err := session.ProveFixes(context.Background(), handles)
	if err != nil {
		t.Fatal(err)
	}
	if *calls == 0 {
		t.Fatal("proof path never invoked the builder seam")
	}
	if got := proofBatchViews.Load(); got != 0 {
		t.Errorf("before-view preparations = %d; discovery already prepared this source's views", got)
	}
	if proofBatchReuse.Load() == 0 {
		t.Error("seeded views were not reused")
	}
	if proved := countSafeProved(first); proved != 57 {
		t.Fatalf("proved = %d, want 57", proved)
	}
	second, err := session.ProveFixes(context.Background(), handles)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := marshalJSON(t, second), marshalJSON(t, first); got != want {
		t.Fatal("repeat ProveFixes changed bytes")
	}
}

// TestProofBatchKeepsPerProfileWorkSeparate proves one edit set under three
// renderer builds and requires one view and one interpretation per profile.
func TestProofBatchKeepsPerProfileWorkSeparate(t *testing.T) {
	doc := redundantStyleDoc()
	targets := threeProfileTargets(t)
	profiles := targets.Profiles()
	if len(profiles) != 3 {
		t.Fatalf("profiles = %d, want 3", len(profiles))
	}
	calls := instrumentProofBuilder(t)
	resetProofBatchCounters()
	findings := AnalyzeDocumentForTargets(doc, targets)
	if *calls != 1 {
		t.Fatalf("proof attempts = %d, want one aggregate build", *calls)
	}
	if got := proofBatchViews.Load(); got != 3 {
		t.Errorf("before-view preparations = %d, want one per profile", got)
	}
	proofs, edits := proofGroups(findings)
	if len(proofs) != 1 {
		t.Fatalf("groups = %d, want 1", len(proofs))
	}
	proved := countSafeProved(findings)
	if proved != 2 {
		t.Fatalf("proved = %d, want 2", proved)
	}
	for id, proof := range proofs {
		seen := map[string]bool{}
		if len(proof.Interpretations) != 3 {
			t.Fatalf("proof %s interpretations = %d, want 3", id, len(proof.Interpretations))
		}
		for _, interpretation := range proof.Interpretations {
			if seen[interpretation.Target.Renderer] {
				t.Fatalf("proof %s repeats renderer %s", id, interpretation.Target.Renderer)
			}
			seen[interpretation.Target.Renderer] = true
			if len(interpretation.Dialogues) != 2 {
				t.Fatalf("proof %s renderer %s covers %d dialogues, want 2",
					id, interpretation.Target.Renderer, len(interpretation.Dialogues))
			}
		}
		rebuilt := rebuildOneOff(t, doc, profiles, edits[id], fixEditIndex(findings))
		if got, want := marshalJSON(t, rebuilt), marshalJSON(t, proof); got != want {
			t.Fatalf("one-off rebuild of %s differs from the batch proof", id)
		}
	}
	fixed, count, err := ApplyFixesForTargets(doc.Text, findings, false, targets)
	if err != nil || count != proved {
		t.Fatalf("apply: count=%d err=%v", count, err)
	}
	if strings.Contains(fixed, `\b0\b0`) {
		t.Error("proved edits were not applied")
	}
}

// missingFieldDoc carries one fractional Style integer, one provable dialogue,
// and one dialogue whose event row is missing fields.
func missingFieldDoc() ass.Document {
	text := "[Script Info]\nPlayResX: 1920\nPlayResY: 1080\nYCbCr Matrix: None\n" +
		"[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n" +
		"Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,0.0,0,0,1\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\b1\\b1\\fs20\\fs20\\t(0,500,\\fscx100)}Benchmark subtitle\n" +
		"Dialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0\n"
	return ass.Parse(text)
}

// TestProofBatchRejectsUnverifiableDialogueLayout requires the batch to keep
// provable groups and refuse the group whose document-wide sweep meets a
// dialogue row it cannot map, instead of proving a partial document.
func TestProofBatchRejectsUnverifiableDialogueLayout(t *testing.T) {
	doc := missingFieldDoc()
	findings := AnalyzeDocument(doc)
	proved := countSafeProved(findings)
	if proved != 3 {
		t.Fatalf("proved = %d, want the three dialogue edits of the mapped row", proved)
	}
	for _, diagnostic := range findings {
		if diagnostic.ID != IssueStyleInteger {
			continue
		}
		if diagnostic.FixSafety == SafeFix || diagnostic.FixProof != nil || len(diagnostic.Edits) != 0 {
			t.Fatalf("Style edit survived an unverifiable dialogue layout: %#v", diagnostic)
		}
	}
	fixed, count, err := ApplyFixes(doc.Text, findings, false)
	if err != nil || count != proved {
		t.Fatalf("apply: count=%d err=%v", count, err)
	}
	if !strings.Contains(fixed, ",2,0.0,0,0,1") {
		t.Error("the rejected Style edit was applied")
	}
}

// TestProofBatchReplacesForeignSourceBatch proves a second source with a
// session's batch and requires a fresh batch, then reuses the session's own.
func TestProofBatchReplacesForeignSourceBatch(t *testing.T) {
	session := discoverTestSession(t, ass.Parse(benchmarkLateProofFailure(6)), nil)
	foreign := ass.Parse(benchmarkLateProofFailure(7))
	targets := DefaultAnalysisTargets()
	diagnostics := candidateDiagnostics(analyzeDocumentForTargetsUnproved(foreign, targets, nil))
	resetProofBatchCounters()
	authorized, err := proveSafeFixesBatchContext(context.Background(), foreign, diagnostics, targets.Profiles(), session.proof)
	if err != nil {
		t.Fatal(err)
	}
	if got := proofBatchViews.Load(); got != int64(len(targets.Profiles())) {
		t.Errorf("before-view preparations = %d for a foreign source, want a fresh batch's %d", got, len(targets.Profiles()))
	}
	foreignViews := proofBatchViews.Load()
	if foreignViews != int64(len(targets.Profiles())) {
		t.Fatalf("foreign batch prepared %d views, want %d", foreignViews, len(targets.Profiles()))
	}
	proved := countSafeProved(authorized)
	if proved == 0 {
		t.Fatal("foreign source proved nothing")
	}
	eager := AnalyzeDocumentForTargets(foreign, targets)
	if want := countSafeProved(eager); proved != want {
		t.Fatalf("foreign proof proved %d, want %d", proved, want)
	}

	handles := allCandidateHandles(session)
	before := proofBatchViews.Load()
	if _, err := session.ProveFixes(context.Background(), handles); err != nil {
		t.Fatalf("session batch unusable after a foreign source: %v", err)
	}
	if got := proofBatchViews.Load(); got != before {
		t.Errorf("session prove prepared %d extra views; it must reuse its own discovery views", got-before)
	}
}

// styleVariantDoc carries two dialogues with identical text under two Styles,
// plus one fractional Style integer. The Style edit selects every dialogue, so
// one proof covers two dialogues whose before and after texts are equal while
// the Styles they are interpreted under are not.
func styleVariantDoc() ass.Document {
	text := "[Script Info]\nPlayResX: 1920\nPlayResY: 1080\nYCbCr Matrix: None\n" +
		"[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n" +
		"Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,0.0,0,0,1\n" +
		"Style: Alt,Arial,30,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,0,0,0,1\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,Benchmark subtitle\n" +
		"Dialogue: 0,0:00:01.00,0:00:02.00,Alt,,0,0,0,,Benchmark subtitle\n"
	return ass.Parse(text)
}

// TestProofBatchKeysAfterTraceDigestByDialogue proves one Style edit that
// selects two dialogues whose texts are identical under two different Styles.
// Both dialogues share one after text, so a digest keyed by text alone reports
// one dialogue's trace under the other dialogue's line. Every published digest
// must describe its own dialogue: a Style field edit changes no dialogue text
// and no interpreted Style state, so each dialogue's before and after traces
// are identical bytes and must hash equal, while the two dialogues, whose
// active Styles differ, must not share a digest.
func TestProofBatchKeysAfterTraceDigestByDialogue(t *testing.T) {
	doc := styleVariantDoc()
	findings := AnalyzeDocument(doc)
	proofs, _ := proofGroups(findings)
	if len(proofs) != 1 {
		t.Fatalf("aggregate groups = %d, want the single Style-edit proof", len(proofs))
	}
	for id, proof := range proofs {
		if len(proof.Interpretations) == 0 {
			t.Fatalf("proof %s carries no interpretation", id)
		}
		for _, interpretation := range proof.Interpretations {
			target := interpretation.Target.Renderer
			dialogues := interpretation.Dialogues
			if len(dialogues) != 2 {
				t.Fatalf("proof %s %s covers %d dialogues, want the two Style-edit dialogues", id, target, len(dialogues))
			}
			if dialogues[0].BeforeTraceSHA256 == dialogues[1].BeforeTraceSHA256 {
				t.Fatalf("proof %s %s: both dialogues share one before digest; the fixture must interpret them under different Styles", id, target)
			}
			if dialogues[0].AfterTraceSHA256 == dialogues[1].AfterTraceSHA256 {
				t.Errorf("proof %s %s: the two dialogues share after digest %s although their Styles differ",
					id, target, dialogues[0].AfterTraceSHA256)
			}
			for _, dialogue := range dialogues {
				if dialogue.AfterTraceSHA256 != dialogue.BeforeTraceSHA256 {
					t.Errorf("proof %s %s line %d: after digest %s, want this dialogue's own before digest %s; the Style edit changed no dialogue text",
						id, target, dialogue.Line, dialogue.AfterTraceSHA256, dialogue.BeforeTraceSHA256)
				}
			}
		}
	}
}

// TestProofBatchRequiresTheSameDocumentShape requires the batch gate to
// compare the document the batch was built for with the document it is
// offered. Source equality cannot replace that comparison, because the same
// text parses to the same source: the line and dialogue counts are what tell a
// batch that its prepared views, trees, and digests do not describe this
// document.
func TestProofBatchRequiresTheSameDocumentShape(t *testing.T) {
	doc := ass.Parse(perftest.StyleEdit(t))
	profiles := DefaultAnalysisTargets().Profiles()
	batch := newProofBatch(doc.Text, doc, profiles)
	truncated := doc
	truncated.Dialogues = doc.Dialogues[:1]
	shortened := doc
	shortened.Lines = doc.Lines[:1]
	if !batch.usable(doc.Text, doc, profiles) {
		t.Fatal("a batch rejected its own document")
	}
	if batch.usable(doc.Text, truncated, profiles) {
		t.Error("a batch accepted a document with fewer dialogues")
	}
	if batch.usable(doc.Text, shortened, profiles) {
		t.Error("a batch accepted a document with fewer lines")
	}
}
