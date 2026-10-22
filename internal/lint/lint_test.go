package lint

import (
	"strings"
	"testing"

	"assx/internal/ass"
)

func TestAnalyzeReportsUnknownAndRepeatedSlashAsSuggestion(t *testing.T) {
	diagnostics := Analyze(ass.Dialogue{Text: `{\\blur2\mystery}`})
	ids := make(map[string]Severity)
	for _, diagnostic := range diagnostics {
		ids[diagnostic.ID] = diagnostic.Severity
	}
	if ids[IssueRepeatedSlash] != Suggestion || ids[IssueUnknownTag] != Suggestion {
		t.Fatalf("diagnostic severities = %#v", ids)
	}
}

func TestAnalyzeReportsSameValueAssignments(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{name: "same font size", text: `{\fs20}A{\fs20}B`, want: 1},
		{name: "numeric forms", text: `{\fs20}A{\fs20.0}B`, want: 1},
		{name: "same blur", text: `{\blur1}A{\blur1}B`, want: 1},
		{name: "color alias", text: `{\c&HFFFFFF&}A{\1c&HFFFFFF&}B`, want: 1},
		{name: "rotation alias", text: `{\fr1}A{\frz1.0}B`, want: 1},
		{name: "all border slots", text: `{\xbord2\ybord2}A{\bord2}B`, want: 1},
		{name: "partial border slots", text: `{\xbord2\ybord3}A{\bord2}B`},
		{name: "alpha multi-slot alias", text: `{\alpha&HFF&}A{\1a&HFF&}B`, want: 1},
		{name: "reset invalidates prior value", text: `{\fs20\r\fs20}A`, want: 1},
		{name: "relative size is not absolute", text: `{\fs20}A{\fs+10}B`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var findings []Diagnostic
			for _, diagnostic := range Analyze(ass.Dialogue{Text: test.text}) {
				if diagnostic.ID == IssueNoEffect {
					findings = append(findings, diagnostic)
				}
			}
			if len(findings) != test.want {
				t.Fatalf("ASS006 findings = %#v, want %d", findings, test.want)
			}
			if test.name == "reset invalidates prior value" && !strings.Contains(findings[0].Detail, "Reset before") {
				t.Fatalf("reset finding = %#v, want only the pre-reset assignment", findings[0])
			}
		})
	}
}

func TestAnalyzeAssignsMultiSlotTagsOnlyWhenEverySlotMatches(t *testing.T) {
	text := `{\xbord2\ybord2}A{\bord2}B`
	var findings []Diagnostic
	for _, diagnostic := range Analyze(ass.Dialogue{Text: text}) {
		if diagnostic.ID == IssueNoEffect {
			findings = append(findings, diagnostic)
		}
	}
	if len(findings) != 1 || findings[0].Tag != "bord" {
		t.Fatalf("multi-slot findings = %#v", findings)
	}
	fixed, _, err := ApplyFixes(text, findings, false)
	if err != nil {
		t.Fatal(err)
	}
	if fixed != `{\xbord2\ybord2}AB` {
		t.Fatalf("fixed text = %q", fixed)
	}
}

func TestAnalyzeAssignFirstWinsAndTransitionBehavior(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{name: "assigned override", text: `{\fs10\fs20}x`, want: 1},
		{name: "first wins", text: `{\an5\an1}x`, want: 1},
		{name: "transition retains start value", text: `{\blur6\t(0,650,0.1,\blur0.6)}x`, want: 0},
		{name: "reset before text", text: `{\fs10\r}x`, want: 1},
		{name: "overridden after text remains effective", text: `{\fs10}x{\fs20}y`, want: 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			count := 0
			for _, diagnostic := range Analyze(ass.Dialogue{Text: test.text}) {
				if diagnostic.ID == IssueNoEffect {
					count++
				}
			}
			if count != test.want {
				t.Fatalf("got %d no-effect findings, want %d", count, test.want)
			}
		})
	}
}

func TestUnverifiedRepeatedSlashEditRemainsUnavailable(t *testing.T) {
	for _, text := range []string{
		`{\\blur2}x`,
		`{\\\blur2}x`,
	} {
		diagnostics := Analyze(ass.Dialogue{Text: text})
		for _, diagnostic := range diagnostics {
			if diagnostic.ID == IssueRepeatedSlash && (diagnostic.FixSafety != "" || len(diagnostic.Edits) != 0) {
				t.Fatalf("unverified parser-prefix edit remains available: %#v", diagnostic)
			}
		}
		fixed, count, err := ApplyFixes(text, diagnostics, false)
		if err != nil || count != 0 || fixed != text {
			t.Errorf("unverified parser-prefix edit changed the source: (%q, %d, %v)", fixed, count, err)
		}
	}
}

func TestSafeFixRemovesOnlyTheRedundantTag(t *testing.T) {
	text := `{\fs10\fs20}word`
	diagnostics := Analyze(ass.Dialogue{Text: text})
	fixed, count, err := ApplyFixes(text, diagnostics, false)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || fixed != `{\fs20}word` {
		t.Fatalf("safe fix = (%q, %d), want (%q, 1)", fixed, count, `{\fs20}word`)
	}
}

func TestHeaderChecksAndUnsafeFixes(t *testing.T) {
	text := "[Script Info]\r\nPlayResX: 1920\r\nPlayResY: 1080\r\nYCbCr Matrix: TV.601\r\n[Events]\r\nFormat: Layer, Start, End, Text\r\nDialogue: 0,0,1,text\r\n"
	doc := ass.Parse(text)
	diagnostics := AnalyzeHeaders(doc)
	if len(diagnostics) != 2 || diagnostics[0].ID != IssueMatrixHeader || diagnostics[1].ID != IssueLayoutRes {
		t.Fatalf("header diagnostics = %#v", diagnostics)
	}
	if diagnostics[0].FixSafety != UnsafeFix || diagnostics[1].FixSafety != UnsafeFix {
		t.Fatalf("header fix safety = %#v", diagnostics)
	}
	unchanged, count, err := ApplyFixes(text, diagnostics, false)
	if err != nil || unchanged != text || count != 0 {
		t.Fatalf("safe-only apply = (%q, %d, %v)", unchanged, count, err)
	}
	fixed, count, err := ApplyFixes(text, diagnostics, true)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || !strings.Contains(fixed, "YCbCr Matrix: TV.709") || !strings.Contains(fixed, "LayoutResX: 1920\r\nLayoutResY: 1080\r\n") {
		t.Fatalf("unsafe fixed text (%d fixes):\n%s", count, fixed)
	}
	if got := len(AnalyzeHeaders(ass.Parse(fixed))); got != 0 {
		t.Fatalf("fixed headers still produce %d diagnostics", got)
	}
}

func TestMissingMatrixUnsafeFixAddsExplicitNone(t *testing.T) {
	text := "[Script Info]\nPlayResX: 640\nPlayResY: 480\n[Events]\n"
	diagnostics := AnalyzeHeaders(ass.Parse(text))
	fixed, count, err := ApplyFixes(text, diagnostics, true)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || !strings.Contains(fixed, "[Script Info]\nYCbCr Matrix: None\nLayoutResX: 640\nLayoutResY: 480\nPlayResX: 640") {
		t.Fatalf("missing header fixes = (%d, %q)", count, fixed)
	}
}

func TestApplyFixesRejectsOverlappingEdits(t *testing.T) {
	diagnostics := []Diagnostic{{
		FixSafety: SafeFix, FixProof: &FixProof{SourceSHA256: hashSource("abcdef"), Targets: fixTargets(defaultFixTargets())},
		Edits: []TextEdit{{Start: 1, End: 3}, {Start: 2, End: 4}},
	}}
	if _, _, err := ApplyFixes("abcdef", diagnostics, false); err == nil {
		t.Fatal("expected overlapping edit error")
	}
}

// provedNoEffectFix returns one source and its one proved no-effect finding.
func provedNoEffectFix(t *testing.T) (string, Diagnostic) {
	t.Helper()
	doc := parseFontOverrideText(`{\fs21\fs21}A`)
	for _, diagnostic := range AnalyzeDocument(doc) {
		if diagnostic.ID == IssueNoEffect && diagnostic.FixProof != nil && len(diagnostic.Edits) == 1 {
			return doc.Text, diagnostic
		}
	}
	t.Fatal("test document has no proved no-effect finding")
	return "", Diagnostic{}
}

// A candidate edit is authorized by the complete proof entry: its rule
// identity, source span, and replacement text must all match a proof edit.
func TestApplyFixesAuthorizesOnlyTheProvedEdit(t *testing.T) {
	source, proved := provedNoEffectFix(t)
	provedEdit := proved.FixProof.SourceEdits[0].Edit
	cases := []struct {
		name    string
		id      string
		edit    TextEdit
		outside bool
	}{
		{"proved edit", proved.ID, provedEdit, false},
		{"different replacement", proved.ID, TextEdit{Start: provedEdit.Start, End: provedEdit.End, Replacement: `\fs22`}, true},
		{"wider span", proved.ID, TextEdit{Start: provedEdit.Start, End: provedEdit.End + 1, Replacement: provedEdit.Replacement}, true},
		{"narrower span", proved.ID, TextEdit{Start: provedEdit.Start, End: provedEdit.End - 1, Replacement: provedEdit.Replacement}, true},
		{"different rule", IssueRepeatedSlash, provedEdit, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			diagnostic := proved
			diagnostic.ID = test.id
			diagnostic.Edits = []TextEdit{test.edit}
			fixed, count, err := ApplyFixes(source, []Diagnostic{diagnostic}, false)
			outside := err != nil && strings.Contains(err.Error(), "outside its proof scope")
			if outside != test.outside {
				t.Fatalf("ApplyFixes error = %v, want outside-proof-scope %v", err, test.outside)
			}
			if test.outside {
				return
			}
			if err != nil || count != 1 || fixed == source {
				t.Fatalf("proved edit application = (%q, %d, %v)", fixed, count, err)
			}
		})
	}
}

// Repeated proof entries authorize the same edit: membership is a set of
// complete entries, not a count. Verification still rejects the duplicate
// later, which is where a proof carrying a repeated edit cannot survive.
func TestApplyFixesAuthorizesRepeatedProofEntry(t *testing.T) {
	source, proved := provedNoEffectFix(t)
	entry := proved.FixProof.SourceEdits[0]
	proof := *proved.FixProof
	proof.SourceEdits = []FixEditProof{entry, entry}
	diagnostic := proved
	diagnostic.Edits = []TextEdit{entry.Edit, entry.Edit}
	diagnostic.FixProof = &proof
	if _, _, err := ApplyFixes(source, []Diagnostic{diagnostic}, false); err == nil || strings.Contains(err.Error(), "outside its proof scope") {
		t.Fatalf("repeated proof entries lost authorization: %v", err)
	}
}

// The membership index must answer exactly what the linear scan answers. An
// index broader than the scan authorizes a foreign edit; one narrower rejects a
// proved edit. Rule identity, span, and replacement text are all part of the
// key, and one span can carry two rule identities.
func TestProofEditMembershipMatchesLinearScan(t *testing.T) {
	proved := []FixEditProof{
		{ID: IssueNoEffect, Edit: TextEdit{Start: 1, End: 4, Replacement: `\b1`}},
		{ID: IssueNoEffect, Edit: TextEdit{Start: 9, End: 14}},
		{ID: IssueRepeatedSlash, Edit: TextEdit{Start: 9, End: 14}},
	}
	proof := &FixProof{SourceEdits: proved}
	membership := proofEditsOf(proof)
	probes := append([]FixEditProof{}, proved...)
	probes = append(probes,
		FixEditProof{ID: IssueNoEffect, Edit: TextEdit{Start: 1, End: 4}},
		FixEditProof{ID: IssueNoEffect, Edit: TextEdit{Start: 1, End: 5, Replacement: `\b1`}},
		FixEditProof{ID: IssueNoEffect, Edit: TextEdit{Start: 0, End: 4, Replacement: `\b1`}},
		FixEditProof{ID: IssueUnknownTag, Edit: TextEdit{Start: 9, End: 14}},
		FixEditProof{ID: IssueNoEffect, Edit: TextEdit{Start: 100, End: 101}},
	)
	for _, probe := range probes {
		got, want := membership.contains(probe), proofContainsEdit(proof, probe)
		if got != want {
			t.Fatalf("membership index and linear scan disagree on %#v: index=%v scan=%v", probe, got, want)
		}
	}
	if !membership.contains(proved[0]) {
		t.Fatal("membership index rejected a proved edit")
	}
	if membership.contains(probes[len(proved)]) {
		t.Fatal("membership index authorized an edit the proof does not contain")
	}
	if proofEditsOf(&FixProof{}).contains(proved[0]) {
		t.Fatal("empty proof authorized an edit")
	}
}

// Authorization work must grow with the edit count, not with its square.
func TestApplyFixesDoesNotScanEveryProofEdit(t *testing.T) {
	measure := func(dialogues int) (int, int) {
		doc := ass.Parse(benchmarkASSDocument(dialogues))
		findings := AnalyzeDocument(doc)
		before := proofMembershipComparisons.Load()
		fixed, count, err := ApplyFixes(doc.Text, findings, false)
		work := int(proofMembershipComparisons.Load() - before)
		if err != nil || count != 3*dialogues || fixed == doc.Text {
			t.Fatalf("applying %d proved edits: count=%d err=%v", 3*dialogues, count, err)
		}
		return work, count
	}
	small, smallEdits := measure(100)
	large, largeEdits := measure(400)
	if large > small*8 {
		t.Fatalf("membership comparisons grew with the square of the edit count: %d for %d edits, %d for %d edits",
			small, smallEdits, large, largeEdits)
	}
}

func TestRuleRegistryHasStableMetadata(t *testing.T) {
	rules := []struct {
		id, want string
	}{
		{IssueArgumentCount, "ASS001"}, {IssueInvalidValue, "ASS002"},
		{IssueRendererDiff, "ASS003"}, {IssueFontComma, "ASS004"},
		{IssueVSFilterModTag, "ASS005"}, {IssueNoEffect, "ASS006"},
		{IssueUnknownTag, "ASS007"}, {IssueMatrixHeader, "ASS008"},
		{IssueLayoutRes, "ASS009"}, {IssueRepeatedSlash, "ASS010"},
		{IssueStyleInteger, "ASS011"}, {IssueStyleFloat, "ASS012"},
		{IssueRedundantStyleOverrides, "ASS013"}, {IssueRepeatedOpenBrace, "ASS014"},
		{IssueUndefinedStyle, "ASS017"},
		{IssueEmptyOverrideBlock, "ASS018"}, {IssueOverrideJunk, "ASS019"}, {IssueMalformedDrawing, "ASS020"},
		{IssueStyleBoxGeometry, "ASS033"}, {IssueStyleBorderMode, "ASS034"},
	}
	for _, test := range rules {
		id := test.id
		if id != test.want {
			t.Errorf("rule ID = %q, want %q", id, test.want)
		}
		rule, ok := Rules[id]
		if !ok || rule.ID != id || rule.Title == "" || rule.Description == "" {
			t.Errorf("rule %q is missing required metadata: %#v", id, rule)
		}
	}
}

func TestNoEffectTransformFixRequiresVerifiedSignature(t *testing.T) {
	cases := []struct {
		text        string
		want        string
		wantSafeFix bool
	}{
		{text: `{\pos(100,100)\t()}A`, want: `{\pos(100,100)\t()}A`},
		{text: `{\pos(100,100)\bord2\t(\bord2)}A`, want: `{\pos(100,100)\bord2}A`, wantSafeFix: true},
	}
	for _, test := range cases {
		dialogue := ass.Dialogue{Text: test.text, Line: 1}
		diagnostics := Analyze(dialogue)
		var transform []Diagnostic
		for _, diagnostic := range diagnostics {
			if diagnostic.ID == IssueNoEffect && diagnostic.Tag == "t" {
				transform = append(transform, diagnostic)
			}
		}
		if len(transform) != 1 {
			t.Fatalf("%s: diagnostics = %#v", test.text, diagnostics)
		}
		if !test.wantSafeFix {
			if transform[0].FixSafety != "" || len(transform[0].Edits) != 0 {
				t.Fatalf("unverified transform signature remained fixable: %#v", transform[0])
			}
			for _, includeUnsafe := range []bool{false, true} {
				fixed, count, err := ApplyFixes(test.text, transform, includeUnsafe)
				if err != nil || count != 0 || fixed != test.text {
					t.Fatalf("unverified transform changed with unsafe=%t: (%q, %d, %v)", includeUnsafe, fixed, count, err)
				}
			}
			continue
		}
		if transform[0].FixSafety != SafeFix {
			t.Fatalf("verified transform fix lacks proof: %#v", transform[0])
		}
		fixed, count, err := ApplyFixes(test.text, transform, false)
		if err != nil || count != 1 || fixed != test.want {
			t.Fatalf("%s: fixed=%q count=%d, want %q (%v)", test.text, fixed, count, test.want, err)
		}
	}
}
