package lint

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"assx/internal/ass"
	"assx/internal/perftest"
)

func discoverTestSession(t *testing.T, doc ass.Document, policy RulePolicy) *Session {
	t.Helper()
	session, err := Discover(context.Background(), doc, DefaultAnalysisTargets(), policy)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	return session
}

func allCandidateHandles(session *Session) []CandidateID {
	var handles []CandidateID
	for _, finding := range session.Findings() {
		if finding.HasFixCandidate {
			handles = append(handles, finding.Candidate)
		}
	}
	return handles
}

func findingsWithID(findings []PendingFinding, id string) []PendingFinding {
	var out []PendingFinding
	for _, finding := range findings {
		if finding.Diagnostic.ID == id {
			out = append(out, finding)
		}
	}
	return out
}

// candidateDiagnostics returns the diagnostics that carry an unproved fix
// candidate: the same set Discover stores as candidates.
func candidateDiagnostics(diagnostics []Diagnostic) []Diagnostic {
	var out []Diagnostic
	for _, diagnostic := range diagnostics {
		if diagnostic.FixSafety == SafeFix && len(diagnostic.Edits) != 0 {
			out = append(out, diagnostic)
		}
	}
	return out
}

// redundantStyleDoc has Default Bold 0. Each dialogue carries one override
// block that is wholly Style-redundant (ASS013). Both tags inside the block
// are also first-wins no-effect findings (ASS006) whose edits overlap the
// ASS013 block edit.
func redundantStyleDoc() ass.Document {
	text := "[Script Info]\nPlayResX: 1920\nPlayResY: 1080\nYCbCr Matrix: None\n" +
		"[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n" +
		"Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,0,0,0,1\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\b0\\b0}plain\n" +
		"Dialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,{\\b0\\b0}again\n"
	return ass.Parse(text)
}

func TestSessionDiscoveryStripsSafeFixAndNeverBuildsProof(t *testing.T) {
	calls := 0
	old := proofBuilder
	proofBuilder = func(batch *proofBatch, edits []FixEditProof, validEdits map[FixEditProof]struct{}) (*FixProof, error) {
		calls++
		return old(batch, edits, validEdits)
	}
	t.Cleanup(func() { proofBuilder = old })

	session := discoverTestSession(t, ass.Parse(perftest.StyleEdit(t)), nil)
	if calls != 0 {
		t.Fatalf("Discover invoked the proof builder %d times, want 0", calls)
	}
	findings := session.Findings()
	if got := len(findingsWithID(findings, IssueNoEffect)); got != perftest.StyleEditDialogueCount*3 {
		t.Fatalf("pending ASS006 = %d, want %d", got, perftest.StyleEditDialogueCount*3)
	}
	if got := len(findingsWithID(findings, IssueStyleInteger)); got != 1 {
		t.Fatalf("pending ASS011 = %d, want 1", got)
	}
	for _, finding := range findings {
		if !finding.HasFixCandidate {
			continue
		}
		if finding.Diagnostic.FixSafety != "" || finding.Diagnostic.Edits != nil ||
			finding.Diagnostic.FixProof != nil || finding.Diagnostic.FixProofRef != "" {
			t.Fatalf("pending candidate keeps unproved fix state: %#v", finding.Diagnostic)
		}
	}

	authorized, err := session.ProveFixes(context.Background(), allCandidateHandles(session))
	if err != nil {
		t.Fatal(err)
	}
	if calls == 0 {
		t.Fatal("proof path never invoked the builder seam")
	}
	callsAfter := calls
	findings = session.Findings()
	if calls != callsAfter {
		t.Fatalf("Findings built %d proofs", calls-callsAfter)
	}
	for _, finding := range findings {
		if finding.HasFixCandidate && (finding.Diagnostic.FixSafety != "" || finding.Diagnostic.FixProof != nil) {
			t.Fatalf("Findings after proof still leaks fix state: %#v", finding.Diagnostic)
		}
	}
	candidates := 0
	for _, diagnostic := range authorized {
		if diagnostic.FixSafety != SafeFix {
			continue
		}
		candidates++
		if diagnostic.FixProof == nil && diagnostic.FixProofRef == "" {
			t.Fatalf("authorized candidate lacks proof: %#v", diagnostic)
		}
	}
	if want := perftest.StyleEditDialogueCount*3 + 1; candidates != want {
		t.Fatalf("authorized candidates = %d, want %d", candidates, want)
	}
}

func TestSessionDisabledRuleKeepsOverlappedCandidates(t *testing.T) {
	doc := redundantStyleDoc()
	enabled := discoverTestSession(t, doc, nil)
	if got := len(findingsWithID(enabled.Findings(), IssueRedundantStyleOverrides)); got != 2 {
		t.Fatalf("enabled ASS013 findings = %d, want 2", got)
	}
	if got := len(findingsWithID(enabled.Findings(), IssueNoEffect)); got != 0 {
		t.Fatalf("enabled ASS013 must suppress overlapping ASS006 findings, got %d", got)
	}

	disabled := discoverTestSession(t, doc, RulePolicy{{ID: IssueRedundantStyleOverrides, Disabled: true}})
	if got := len(findingsWithID(disabled.Findings(), IssueRedundantStyleOverrides)); got != 0 {
		t.Fatalf("disabled ASS013 findings = %d, want 0", got)
	}
	overlapped := findingsWithID(disabled.Findings(), IssueNoEffect)
	if len(overlapped) != 4 {
		t.Fatalf("disabled ASS013 must keep ASS006 findings, got %d", len(overlapped))
	}
	var handles []CandidateID
	for _, finding := range overlapped {
		if !finding.HasFixCandidate {
			t.Fatalf("overlapped ASS006 lost its candidate: %#v", finding.Diagnostic)
		}
		handles = append(handles, finding.Candidate)
	}
	authorized, err := disabled.ProveFixes(context.Background(), handles)
	if err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range authorized {
		if diagnostic.FixSafety != SafeFix || len(diagnostic.Edits) != 1 {
			t.Fatalf("subset proof: %#v", diagnostic)
		}
	}
	fixed, count, err := disabled.ApplyFixes(context.Background(), authorized, false)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.ReplaceAll(doc.Text, `{`+`\b0\b0`+`}`, `{}`)
	if count != 4 || fixed != want {
		t.Fatalf("application: count=%d match=%v fixed=%q", count, fixed == want, fixed)
	}
}

func TestSessionSeverityOverrideAndResultCopies(t *testing.T) {
	session := discoverTestSession(t, ass.Parse(perftest.StyleEdit(t)), RulePolicy{
		{ID: IssueNoEffect, Severity: Error},
		{ID: IssueStyleInteger, Disabled: true},
	})
	if got := len(findingsWithID(session.Findings(), IssueStyleInteger)); got != 0 {
		t.Fatalf("disabled ASS011 findings = %d, want 0", got)
	}
	for _, finding := range findingsWithID(session.Findings(), IssueNoEffect) {
		if finding.Diagnostic.Severity != Error {
			t.Fatalf("severity override not applied: %#v", finding.Diagnostic)
		}
	}
	first := session.Findings()[0]
	cloneIndex := -1
	for i, finding := range session.Findings() {
		if len(finding.Diagnostic.Sources) != 0 && finding.Diagnostic.Detail != "" {
			cloneIndex = i
			break
		}
	}
	if cloneIndex < 0 {
		t.Fatal("no finding carries mutable Sources and Detail")
	}
	first = session.Findings()[cloneIndex]
	first.Diagnostic.Sources = append(first.Diagnostic.Sources, "mutated")
	first.Diagnostic.Detail += "!"
	again := session.Findings()[cloneIndex]
	if again.Diagnostic.Detail != strings.TrimSuffix(first.Diagnostic.Detail, "!") || len(again.Diagnostic.Sources) != len(first.Diagnostic.Sources)-1 {
		t.Fatal("Findings aliases session state")
	}

	handles := allCandidateHandles(session)
	authorized, err := session.ProveFixes(context.Background(), handles)
	if err != nil {
		t.Fatal(err)
	}
	if len(authorized) != perftest.StyleEditDialogueCount*3 {
		t.Fatalf("authorized = %d, want %d (ASS011 disabled)", len(authorized), perftest.StyleEditDialogueCount*3)
	}
	authorized[0].Edits = nil
	authorized[0].Sources = append(authorized[0].Sources, "mutated")
	repeated, err := session.ProveFixes(context.Background(), handles)
	if err != nil {
		t.Fatal(err)
	}
	safe := 0
	for _, diagnostic := range repeated {
		if diagnostic.FixSafety == SafeFix && len(diagnostic.Edits) != 0 {
			safe++
		}
	}
	if safe != len(authorized) {
		t.Fatalf("repeated proof authorized %d, want %d", safe, len(authorized))
	}
}

func TestSessionRejectsInvalidPolicy(t *testing.T) {
	for _, test := range []struct {
		name   string
		policy RulePolicy
	}{
		{"unknown id", RulePolicy{{ID: "ASS999"}}},
		{"duplicate id", RulePolicy{{ID: IssueNoEffect}, {ID: IssueNoEffect, Disabled: true}}},
		{"invalid severity", RulePolicy{{ID: IssueNoEffect, Severity: Severity("hint")}}},
		{"empty id", RulePolicy{{Disabled: true}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, err := Discover(context.Background(), redundantStyleDoc(), DefaultAnalysisTargets(), test.policy)
			if err == nil || session != nil {
				t.Fatalf("policy accepted: %v %#v", err, session)
			}
		})
	}
}

func TestSessionRejectsForeignAndUnknownHandles(t *testing.T) {
	doc := redundantStyleDoc()
	left := discoverTestSession(t, doc, nil)
	right := discoverTestSession(t, doc, nil)
	handles := allCandidateHandles(left)
	if len(handles) == 0 {
		t.Fatal("left session found no candidates")
	}
	if _, err := right.ProveFixes(context.Background(), []CandidateID{handles[0]}); err == nil {
		t.Fatal("foreign handle accepted")
	}
	if _, err := left.ProveFixes(context.Background(), []CandidateID{{}}); err == nil {
		t.Fatal("zero handle accepted")
	}
	if _, err := left.ProveFixes(context.Background(), []CandidateID{{session: left.id, ordinal: 1 << 20}}); err == nil {
		t.Fatal("unknown ordinal accepted")
	}
	if _, err := left.ProveFixes(context.Background(), []CandidateID{handles[0], handles[0]}); err == nil {
		t.Fatal("duplicate handle accepted")
	}
	if _, err := left.ProveFixes(context.Background(), handles); err != nil {
		t.Fatalf("session unusable after rejected handles: %v", err)
	}
}

func TestSessionLateBarrierRevokesLastDialogue(t *testing.T) {
	doc := ass.Parse(benchmarkLateProofFailure(250))
	session := discoverTestSession(t, doc, nil)
	lastLine := doc.Dialogues[len(doc.Dialogues)-1].Line
	var handles []CandidateID
	var revoked int
	for _, finding := range session.Findings() {
		if finding.Diagnostic.ID != IssueNoEffect {
			continue
		}
		if finding.HasFixCandidate {
			handles = append(handles, finding.Candidate)
			continue
		}
		if finding.Diagnostic.Line == lastLine {
			revoked++
			if finding.Diagnostic.FixSafety != "" || finding.Diagnostic.Edits != nil || finding.Diagnostic.FixProof != nil || finding.Diagnostic.FixProofRef != "" {
				t.Fatalf("barriered candidate offers a fix: %#v", finding.Diagnostic)
			}
		}
	}
	if revoked != 3 {
		t.Fatalf("barriered findings=%d, want 3", revoked)
	}
	authorized, err := session.ProveFixes(context.Background(), handles)
	if err != nil {
		t.Fatal(err)
	}
	var proved int
	for _, diagnostic := range authorized {
		if diagnostic.ID == IssueNoEffect && diagnostic.FixSafety == SafeFix {
			proved++
		}
	}
	if proved != 747 {
		t.Fatalf("proved=%d, want 747", proved)
	}
	fixed, count, err := ApplyFixes(doc.Text, authorized, false)
	if err != nil || count != 747 || !strings.Contains(fixed, `\mystery`) {
		t.Fatalf("session application: count=%d err=%v", count, err)
	}
}

func TestSessionProveSubsetIsSelfContained(t *testing.T) {
	doc := ass.Parse(benchmarkLateProofFailure(20))
	session := discoverTestSession(t, doc, nil)
	handles := allCandidateHandles(session)
	// The final dialogue's unknown tag revokes its three candidates at
	// discovery, so nineteen dialogues contribute three candidates each.
	if len(handles) != 57 {
		t.Fatalf("uniform doc candidates=%d, want 57", len(handles))
	}
	authorized, err := session.ProveFixes(context.Background(), handles[:2])
	if err != nil {
		t.Fatal(err)
	}
	if len(authorized) != 2 {
		t.Fatalf("subset length=%d, want 2", len(authorized))
	}
	if authorized[0].FixSafety != SafeFix || authorized[1].FixSafety != SafeFix {
		t.Fatalf("subset not authorized: %#v %#v", authorized[0], authorized[1])
	}
	if authorized[0].FixProof == nil && authorized[0].FixProofRef == "" {
		t.Fatalf("subset first finding lacks proof: %#v", authorized[0])
	}
	if authorized[1].FixProof == nil && authorized[1].FixProofRef == "" {
		t.Fatalf("subset second finding lacks proof: %#v", authorized[1])
	}
	fixed, count, err := session.ApplyFixes(context.Background(), authorized, false)
	if err != nil || count != 2 {
		t.Fatalf("subset apply: count=%d err=%v", count, err)
	}
	changed := 0
	for i, dialogue := range ass.Parse(fixed).Dialogues {
		if dialogue.Text != doc.Dialogues[i].Text {
			changed++
		}
	}
	if changed != 1 {
		t.Fatalf("subset edit changed %d dialogues, want 1", changed)
	}
}

func TestSessionEagerAndDeferredMatch(t *testing.T) {
	for _, test := range []struct {
		name string
		text func(t *testing.T) string
	}{
		{"style_edit", func(t *testing.T) string { return perftest.StyleEdit(t) }},
		{"late_100", func(*testing.T) string { return benchmarkLateProofFailure(100) }},
		{"uniform_30", func(*testing.T) string { return benchmarkLateProofFailure(30) }},
		{"redundant", func(*testing.T) string { return redundantStyleDoc().Text }},
	} {
		t.Run(test.name, func(t *testing.T) {
			text := test.text(t)
			doc := ass.Parse(text)
			targets := DefaultAnalysisTargets()
			eager := AnalyzeDocumentForTargets(doc, targets)
			unproved := candidateDiagnostics(analyzeDocumentForTargetsUnproved(doc, targets, nil))
			session := discoverTestSession(t, doc, nil)
			lazy, err := session.ProveFixes(context.Background(), allCandidateHandles(session))
			if err != nil {
				t.Fatal(err)
			}
			if len(lazy) != len(unproved) {
				t.Fatalf("deferred length=%d, unproved candidates=%d", len(lazy), len(unproved))
			}
			candidates := candidateDiagnostics(eager)
			for i := range lazy {
				if lazy[i].FixSafety != candidates[i].FixSafety {
					t.Fatalf("%s candidate %d safety mismatch: %#v vs %#v", test.name, i, lazy[i], candidates[i])
				}
				if !reflect.DeepEqual(lazy[i], candidates[i]) {
					t.Fatalf("%s candidate %d differs from eager", test.name, i)
				}
				lazyJSON, err := json.Marshal(lazy[i])
				if err != nil {
					t.Fatal(err)
				}
				eagerJSON, err := json.Marshal(candidates[i])
				if err != nil {
					t.Fatal(err)
				}
				if string(lazyJSON) != string(eagerJSON) {
					t.Fatalf("%s candidate %d JSON differs from eager bytes", test.name, i)
				}
			}
			eagerFixed, eagerCount, err := ApplyFixes(text, eager, false)
			if err != nil {
				t.Fatal(err)
			}
			lazyFixed, lazyCount, err := session.ApplyFixes(context.Background(), lazy, false)
			if err != nil {
				t.Fatal(err)
			}
			if eagerFixed != lazyFixed || eagerCount != lazyCount {
				t.Fatalf("fixed output mismatch: counts %d vs %d", eagerCount, lazyCount)
			}
		})
	}
}

func TestSessionRejectsStaleAndForeignApplication(t *testing.T) {
	doc := ass.Parse(benchmarkLateProofFailure(10))
	session := discoverTestSession(t, doc, nil)
	authorized, err := session.ProveFixes(context.Background(), allCandidateHandles(session))
	if err != nil {
		t.Fatal(err)
	}
	if len(authorized) == 0 {
		t.Fatal("no authorized findings")
	}

	stale := cloneDiagnosticsForTest(authorized)
	stale[0].Edits[0].Start++
	if _, _, err := session.ApplyFixes(context.Background(), stale, false); err == nil {
		t.Fatal("changed edit accepted")
	}

	tampered := cloneDiagnosticsForTest(authorized)
	proofChanged := false
	for i := range tampered {
		if tampered[i].FixProof != nil {
			tampered[i].FixProof.SourceSHA256 = strings.Repeat("0", 64)
			proofChanged = true
		}
	}
	if !proofChanged {
		t.Fatal("no inline proof to tamper")
	}
	if _, _, err := session.ApplyFixes(context.Background(), tampered, false); err == nil {
		t.Fatal("tampered proof evidence accepted")
	}

	foreign := AnalyzeDocumentForTargets(ass.Parse(benchmarkLateProofFailure(11)), DefaultAnalysisTargets())
	if _, _, err := session.ApplyFixes(context.Background(), foreign, false); err == nil {
		t.Fatal("foreign session findings accepted")
	}
}

func TestSessionCancellationKeepsResultUsable(t *testing.T) {
	doc := ass.Parse(benchmarkLateProofFailure(250))
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if session, err := Discover(canceled, doc, DefaultAnalysisTargets(), nil); err == nil || session != nil {
		t.Fatalf("canceled Discover succeeded: %v %#v", err, session)
	}
	session := discoverTestSession(t, doc, nil)
	if _, err := session.ProveFixes(canceled, allCandidateHandles(session)); err == nil {
		t.Fatal("canceled proof succeeded")
	}
	if len(session.Findings()) == 0 {
		t.Fatal("canceled proof damaged session state")
	}
	authorized, err := session.ProveFixes(context.Background(), allCandidateHandles(session))
	if err != nil {
		t.Fatalf("session unusable after cancellation: %v", err)
	}
	if len(authorized) == 0 {
		t.Fatal("later request returned nothing")
	}
}

func cloneDiagnosticsForTest(diagnostics []Diagnostic) []Diagnostic {
	encoded, err := json.Marshal(diagnostics)
	if err != nil {
		panic(err)
	}
	out := make([]Diagnostic, len(diagnostics))
	if err := json.Unmarshal(encoded, &out); err != nil {
		panic(err)
	}
	for i := range out {
		out[i].FixProof = diagnostics[i].FixProof
	}
	return out
}
