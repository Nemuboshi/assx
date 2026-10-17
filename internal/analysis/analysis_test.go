package analysis

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/lint"
	"assx/internal/perftest"
)

func analyzeTestResult(t *testing.T, text string, options Options) *Result {
	t.Helper()
	snapshot, err := NewSnapshot("file:///test.ass", text, nil)
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	result, err := Analyze(context.Background(), snapshot, options)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	return result
}

func mustSnapshot(t *testing.T, text string) Snapshot {
	t.Helper()
	snapshot, err := NewSnapshot("file:///test.ass", text, nil)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func candidateIDs(result *Result) []FindingID {
	var ids []FindingID
	for _, finding := range result.Findings() {
		if finding.HasFixCandidate {
			ids = append(ids, finding.ID)
		}
	}
	return ids
}

func mustProve(t *testing.T, result *Result) []lint.Diagnostic {
	t.Helper()
	out, err := result.ProveFixes(context.Background(), candidateIDs(result))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// pendingFromEager strips proof labels from eager diagnostics, so the result
// compares against discovery-visible state only.
func pendingFromEager(diagnostics []lint.Diagnostic) []lint.Diagnostic {
	out := make([]lint.Diagnostic, len(diagnostics))
	for i, diagnostic := range diagnostics {
		out[i] = diagnostic
		if diagnostic.FixSafety == lint.SafeFix {
			out[i].FixSafety = ""
			out[i].FixProof = nil
			out[i].FixProofRef = ""
			out[i].Edits = nil
		}
	}
	return out
}

// provedFromEager keeps only candidates that leave eager analysis with a
// SafeFix label, which is what the deferred API returns.
func provedFromEager(diagnostics []lint.Diagnostic) []lint.Diagnostic {
	var out []lint.Diagnostic
	for _, diagnostic := range diagnostics {
		if diagnostic.FixSafety == lint.SafeFix && len(diagnostic.Edits) != 0 {
			out = append(out, diagnostic)
		}
	}
	return out
}

func TestAnalyzeEmptyOptionsMatchDefaultTargets(t *testing.T) {
	text := perftest.StyleEdit(t)
	doc := ass.Parse(text)
	eager := lint.AnalyzeDocumentForTargets(doc, lint.DefaultAnalysisTargets())

	result := analyzeTestResult(t, text, Options{})
	var pending []lint.Diagnostic
	for _, finding := range result.Findings() {
		pending = append(pending, finding.Diagnostic)
	}
	if !reflect.DeepEqual(pending, pendingFromEager(eager)) {
		t.Fatal("default-options findings differ from eager diagnostics")
	}
	authorized := mustProve(t, result)
	if !reflect.DeepEqual(authorized, provedFromEager(eager)) {
		t.Fatal("deferred proof output differs from eager diagnostics")
	}
	full, err := result.Diagnostics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(full, eager) {
		t.Fatal("merged diagnostics differ from eager diagnostics")
	}
	eagerJSON, err := json.Marshal(eager)
	if err != nil {
		t.Fatal(err)
	}
	fullJSON, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	if string(eagerJSON) != string(fullJSON) {
		t.Fatal("merged JSON bytes differ from eager bytes")
	}

	eagerFixed, eagerCount, err := lint.ApplyFixes(text, eager, false)
	if err != nil {
		t.Fatal(err)
	}
	lazyFixed, lazyCount, err := result.ApplyFixes(context.Background(), authorized, false)
	if err != nil {
		t.Fatal(err)
	}
	if eagerFixed != lazyFixed || eagerCount != lazyCount {
		t.Fatalf("fixed output mismatch: %d vs %d", eagerCount, lazyCount)
	}
}

func TestNewSnapshotRejectsInvalidInput(t *testing.T) {
	if _, err := NewSnapshot("", "Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,x", nil); err == nil {
		t.Fatal("empty URI accepted")
	}
	if _, err := NewSnapshot("file:///t.ass", "\xff\xfe not utf8", nil); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
	version := int64(1)
	if _, err := NewSnapshot("file:///t.ass", "[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,ok", &version); err != nil {
		t.Fatalf("valid snapshot rejected: %v", err)
	}
}

func TestNewSnapshotCopiesVersion(t *testing.T) {
	version := int64(7)
	snapshot, err := NewSnapshot("file:///t.ass", "text", &version)
	if err != nil {
		t.Fatal(err)
	}
	version = 9
	if got := snapshot.Version(); got == nil || *got != 7 {
		t.Fatalf("version = %v, want 7", got)
	}
	unversioned, err := NewSnapshot("file:///t.ass", "text", nil)
	if err != nil || unversioned.Version() != nil {
		t.Fatalf("unversioned snapshot: %v %#v", err, unversioned.Version())
	}
	if unversioned.Text() != "text" || unversioned.URI() != "file:///t.ass" {
		t.Fatal("snapshot lost source")
	}
}

func TestAnalyzeOptionIsolation(t *testing.T) {
	text := perftest.StyleEdit(t)
	options := Options{
		Targets: lint.DefaultAnalysisTargets(),
		Rules:   lint.RulePolicy{{ID: lint.IssueNoEffect, Disabled: true}},
	}
	result := analyzeTestResult(t, text, options)
	if len(result.Findings()) == 0 {
		t.Fatal("no findings")
	}
	for _, finding := range result.Findings() {
		if finding.Diagnostic.ID == lint.IssueNoEffect {
			t.Fatalf("disabled rule reported: %#v", finding.Diagnostic)
		}
	}
	options.Rules[0] = lint.RuleOverride{ID: lint.IssueStyleInteger, Disabled: true}
	second := analyzeTestResult(t, text, options)
	for _, finding := range second.Findings() {
		if finding.Diagnostic.ID == lint.IssueStyleInteger {
			t.Fatalf("second result reused the first policy: %#v", finding.Diagnostic)
		}
	}
	firstNoEffect, firstStyleInteger := false, false
	for _, finding := range result.Findings() {
		switch finding.Diagnostic.ID {
		case lint.IssueNoEffect:
			firstNoEffect = true
		case lint.IssueStyleInteger:
			firstStyleInteger = true
		}
	}
	if firstNoEffect || !firstStyleInteger {
		t.Fatal("Options mutation changed an existing result")
	}
}

func TestAnalyzeRendererSelections(t *testing.T) {
	libass, _ := renderer.Standard(renderer.Libass)
	xy, _ := renderer.Standard(renderer.XYVSFilter)
	text := "[Script Info]\nPlayResX: 1920\nPlayResY: 1080\nYCbCr Matrix: TV.601\n" +
		"[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n" +
		"Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\b1\\b1}hello\n"
	first := analyzeTestResult(t, text, Options{Targets: lint.AnalysisTargets{Primary: libass, Compatibility: []renderer.Profile{xy}}})
	second := analyzeTestResult(t, text, Options{Targets: lint.AnalysisTargets{Primary: xy, Compatibility: []renderer.Profile{libass}}})
	scoped := func(result *Result, id string) string {
		for _, finding := range result.Findings() {
			if finding.Diagnostic.ID == id {
				return finding.Diagnostic.Renderer
			}
		}
		t.Fatalf("no %s finding", id)
		return ""
	}
	firstScope, secondScope := scoped(first, lint.IssueNoEffect), scoped(second, lint.IssueNoEffect)
	if !strings.HasPrefix(firstScope, "libass") || !strings.HasPrefix(secondScope, "xy") {
		t.Fatalf("primary scope not selected: %q vs %q", firstScope, secondScope)
	}

	mod := func(build renderer.Build) *Result {
		profile, err := renderer.New(renderer.VSFilterMod, build)
		if err != nil {
			t.Fatal(err)
		}
		return analyzeTestResult(t, text, Options{Targets: lint.AnalysisTargets{Primary: profile}})
	}
	unknown := mod(renderer.Build{Mod: renderer.FeatureUnknown, Lua: renderer.FeatureUnknown})
	if len(unknown.Findings()) == 0 {
		t.Fatal("unknown VSFilterMod build lost findings")
	}
	for _, diagnostic := range mustProve(t, unknown) {
		if diagnostic.FixSafety == lint.SafeFix {
			t.Fatalf("unknown VSFilterMod build authorized a fix: %#v", diagnostic)
		}
	}
	disabled := mod(renderer.Build{Mod: renderer.FeatureDisabled, Lua: renderer.FeatureDisabled})
	proved := 0
	for _, diagnostic := range mustProve(t, disabled) {
		if diagnostic.ID == lint.IssueNoEffect && diagnostic.FixSafety == lint.SafeFix {
			proved++
		}
	}
	if proved == 0 {
		t.Fatal("known disabled VSFilterMod build proved no fix")
	}
}

func TestResultCustomPolicyProofAndApply(t *testing.T) {
	text := perftest.StyleEdit(t)
	result := analyzeTestResult(t, text, Options{Rules: lint.RulePolicy{{ID: lint.IssueStyleInteger, Disabled: true}}})
	ids := candidateIDs(result)
	if want := perftest.StyleEditDialogueCount * 3; len(ids) != want {
		t.Fatalf("candidates=%d, want %d", len(ids), want)
	}
	authorized := mustProve(t, result)
	fixed, count, err := result.ApplyFixes(context.Background(), authorized, false)
	if err != nil {
		t.Fatal(err)
	}
	if count != len(authorized) {
		t.Fatalf("applied=%d, want %d", count, len(authorized))
	}
	if !strings.Contains(fixed, ",0.0,0,0,1") {
		t.Fatal("disabled ASS011 edit was applied")
	}
}

func TestResultRejectsForeignAndModifiedFindings(t *testing.T) {
	text := "[Script Info]\nPlayResX: 1920\nPlayResY: 1080\nYCbCr Matrix: None\n" +
		"[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n" +
		"Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,0,0,0,1\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\b1\\b1}one\n" +
		"Dialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,{\\fs20\\fs20}two\n"
	left := analyzeTestResult(t, text, Options{})
	right := analyzeTestResult(t, text+"\n", Options{})
	if _, err := left.ProveFixes(context.Background(), candidateIDs(right)); err == nil {
		t.Fatal("foreign finding ID accepted")
	}
	if _, err := left.ProveFixes(context.Background(), []FindingID{{}}); err == nil {
		t.Fatal("zero finding ID accepted")
	}
	authorized := mustProve(t, left)
	modified := cloneDiagnostics(authorized)
	modified[0].Edits[0].End++
	if _, _, err := left.ApplyFixes(context.Background(), modified, false); err == nil {
		t.Fatal("modified edit accepted")
	}
	if _, _, err := left.ApplyFixes(context.Background(), mustProve(t, right), false); err == nil {
		t.Fatal("proof group from another result accepted")
	}
}

func cloneDiagnostics(diagnostics []lint.Diagnostic) []lint.Diagnostic {
	out := make([]lint.Diagnostic, len(diagnostics))
	for i, diagnostic := range diagnostics {
		out[i] = diagnostic
		out[i].Edits = append([]lint.TextEdit(nil), diagnostic.Edits...)
	}
	return out
}

func TestResultUnicodeOffsetsAndMalformedRecords(t *testing.T) {
	text := "[Script Info]\nPlayResX: 1920\nPlayResY: 1080\nYCbCr Matrix: None\n" +
		"[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n" +
		"Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\b1\\b1}日本語\n" +
		"Dialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,{\\fs20}short fields\n" +
		"Dialogue: 0,0:00:02.00,0:00:03.00,Default,,0,0,0\n"
	result := analyzeTestResult(t, text, Options{})
	if len(result.Findings()) == 0 {
		t.Fatal("no findings")
	}
	authorized := mustProve(t, result)
	fixed, count, err := result.ApplyFixes(context.Background(), authorized, false)
	if err != nil || count == 0 {
		t.Fatalf("apply: count=%d err=%v", count, err)
	}
	if !strings.Contains(fixed, "日本語") {
		t.Fatal("Unicode content lost")
	}
	edited := ass.Parse(fixed).Dialogues[0].Text
	if !strings.HasPrefix(edited[strings.Index(edited, "日本語"):], "日本語") {
		t.Fatal("Unicode byte offsets wrong after the edit")
	}
	malformedLine := ass.Parse(text).Dialogues[2].Line
	malformed := false
	for _, finding := range result.Findings() {
		if finding.Diagnostic.Line == malformedLine {
			malformed = true
		}
	}
	if !malformed {
		t.Fatal("malformed record not reported")
	}
}

func TestAnalyzeEmptyTargetsVersusExplicitLibassOnly(t *testing.T) {
	// The libass profile is the zero renderer.Profile. An explicit libass-only
	// selection passes a non-nil empty compatibility slice and must prove
	// against libass alone, not the default libass + xy-VSFilter set.
	libass, _ := renderer.Standard(renderer.Libass)
	text := "[Script Info]\nPlayResX: 1920\nPlayResY: 1080\nYCbCr Matrix: None\n" +
		"[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n" +
		"Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\b1\\b1}hello\n"
	explicit := analyzeTestResult(t, text, Options{Targets: lint.AnalysisTargets{Primary: libass, Compatibility: []renderer.Profile{}}})
	authorized := mustProve(t, explicit)
	var proof *lint.FixProof
	for _, diagnostic := range authorized {
		if diagnostic.FixProof != nil {
			proof = diagnostic.FixProof
		}
	}
	if proof == nil {
		t.Fatal("explicit libass-only proved nothing")
	}
	if len(proof.Targets) != 1 || proof.Targets[0].Renderer != "libass" {
		t.Fatalf("explicit libass-only targets = %v, want libass alone", proof.Targets)
	}
	defaults := analyzeTestResult(t, text, Options{})
	defaultAuthorized := mustProve(t, defaults)
	var defaultProof *lint.FixProof
	for _, diagnostic := range defaultAuthorized {
		if diagnostic.FixProof != nil {
			defaultProof = diagnostic.FixProof
		}
	}
	if defaultProof == nil || len(defaultProof.Targets) != 2 {
		t.Fatalf("empty options did not select default targets: %v", defaultProof)
	}
}

func TestAnalyzeRejectsInvalidPolicy(t *testing.T) {
	snapshot := mustSnapshot(t, "[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\b1}x\n")
	for _, policy := range []lint.RulePolicy{
		{{ID: "ASS999"}},
		{{ID: lint.IssueNoEffect}, {ID: lint.IssueNoEffect, Disabled: true}},
		{{ID: lint.IssueNoEffect, Severity: lint.Severity("hint")}},
	} {
		if result, err := Analyze(context.Background(), snapshot, Options{Rules: policy}); err == nil || result != nil {
			t.Fatalf("invalid policy accepted: %v %#v", err, result)
		}
	}
}

func TestProveCancellationAfterBuildReturnsNoPartialOutput(t *testing.T) {
	// Budget: session entry check and proof-core entry check. The aggregate
	// proof build then runs, and the next check stops before any proof is
	// attached, so the caller sees no partial authorized output.
	text := perftest.StyleEdit(t)
	result := analyzeTestResult(t, text, Options{})
	budget := &budgetContext{Context: context.Background(), remaining: 2}
	authorized, err := result.ProveFixes(budget, candidateIDs(result))
	if err == nil || authorized != nil {
		t.Fatalf("canceled proof returned output: %v %#v", err, authorized)
	}
	full, err := result.ProveFixes(context.Background(), candidateIDs(result))
	if err != nil || len(full) == 0 {
		t.Fatalf("result unusable after canceled proof: %v", err)
	}
	if _, _, err := result.ApplyFixes(context.Background(), full, false); err != nil {
		t.Fatalf("later application failed: %v", err)
	}
}

func TestProveFixesNonCandidateAndDuplicates(t *testing.T) {
	text := perftest.StyleEdit(t)
	result := analyzeTestResult(t, text, Options{})
	ids := candidateIDs(result)
	var nonCandidate FindingID
	for _, finding := range result.Findings() {
		if !finding.HasFixCandidate {
			nonCandidate = finding.ID
			break
		}
	}
	if _, err := result.ProveFixes(context.Background(), []FindingID{nonCandidate}); err == nil {
		t.Fatal("non-candidate finding accepted")
	}
	if _, err := result.ProveFixes(context.Background(), []FindingID{ids[0], ids[0]}); err == nil {
		t.Fatal("duplicate finding ID accepted")
	}
}

func TestResultCancellationLeavesUsableState(t *testing.T) {
	text := perftest.StyleEdit(t)
	snapshot := mustSnapshot(t, text)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Analyze(canceled, snapshot, Options{}); err == nil {
		t.Fatal("canceled Analyze succeeded")
	}
	result := analyzeTestResult(t, text, Options{})
	if _, err := result.ProveFixes(canceled, candidateIDs(result)); err == nil {
		t.Fatal("canceled ProveFixes succeeded")
	}
	if _, _, err := result.ApplyFixes(canceled, nil, false); err == nil {
		t.Fatal("canceled ApplyFixes succeeded")
	}
	if len(result.Findings()) == 0 {
		t.Fatal("Findings lost after cancellation")
	}
	authorized, err := result.ProveFixes(context.Background(), candidateIDs(result))
	if err != nil || len(authorized) == 0 {
		t.Fatalf("result unusable after cancellation: %v", err)
	}
}

// budgetContext reports cancellation after a fixed number of checks, so a
// test can cancel exactly between dialogue units. Methods use pointer
// receivers so each check consumes budget.
type budgetContext struct {
	context.Context
	remaining int
}

func (c *budgetContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *budgetContext) Done() <-chan struct{}       { return nil }
func (c *budgetContext) Err() error {
	if c.remaining <= 0 {
		return context.Canceled
	}
	c.remaining--
	return nil
}
func (c *budgetContext) Value(any) any { return nil }

func TestAnalyzeCancellationMidRequestReturnsNoResult(t *testing.T) {
	snapshot := mustSnapshot(t, perftest.StyleEdit(t))
	// Budget: Analyze entry and request-preparation entry; the first
	// per-profile layout check then stops the request.
	budget := &budgetContext{Context: context.Background(), remaining: 2}
	result, err := Analyze(budget, snapshot, Options{})
	if err == nil || result != nil {
		t.Fatalf("mid-document cancellation returned a result: %v %#v", err, result)
	}
	whole, err := Analyze(context.Background(), snapshot, Options{})
	if err != nil || len(whole.Findings()) == 0 {
		t.Fatalf("later request unusable: %v", err)
	}
}

func TestAnalyzeOneLargeDialogueIsAtomic(t *testing.T) {
	block := "{\\b1\\b1" + strings.Repeat("\\fs20", 2000) + "}"
	text := "[Script Info]\nPlayResX: 1920\nPlayResY: 1080\nYCbCr Matrix: None\n" +
		"[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n" +
		"Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,0,0,0,1\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,," + block + "large\n" +
		"Dialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,{\\fs20\\fs20}second\n"
	snapshot := mustSnapshot(t, text)
	// Budget: Analyze entry, request-preparation entry, one check per target
	// profile layout plus one per projection, discovery entry, profile-pass
	// entry, and the dialogue-0 boundary check. The large dialogue then runs
	// whole, and the dialogue-1 boundary stops the request. That proves one
	// dialogue is the atomic unit: no partial dialogue state is returned.
	budget := &budgetContext{Context: context.Background(), remaining: 9}
	result, err := Analyze(budget, snapshot, Options{})
	if err == nil || result != nil {
		t.Fatalf("canceled discovery returned a result: %v %#v", err, result)
	}
	whole, err := Analyze(context.Background(), snapshot, Options{})
	if err != nil || len(whole.Findings()) == 0 {
		t.Fatalf("later request unusable: %v", err)
	}
	authorized, err := whole.ProveFixes(context.Background(), candidateIDs(whole))
	if err != nil {
		t.Fatal(err)
	}
	partial := 0
	for _, diagnostic := range authorized {
		if diagnostic.FixSafety == lint.SafeFix && len(diagnostic.Edits) != 0 {
			partial++
		}
	}
	if partial != len(authorized) {
		t.Fatalf("authorized=%d proved=%d, want all proved", len(authorized), partial)
	}
}
