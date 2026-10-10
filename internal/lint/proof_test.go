package lint

import (
	"encoding/json"
	"strings"
	"testing"

	"assx/internal/ass/renderer"
)

func TestSafeFixProofCarriesEveryDefaultRenderer(t *testing.T) {
	doc := parseFontOverrideText(`{\fs21\fs21}A`)
	diagnostics := AnalyzeDocument(doc)
	var finding *Diagnostic
	for i := range diagnostics {
		if diagnostics[i].ID == IssueNoEffect && diagnostics[i].FixSafety == SafeFix {
			finding = &diagnostics[i]
			break
		}
	}
	if finding == nil || finding.FixProof == nil {
		t.Fatalf("expected a proved ASS006 fix: %#v", diagnostics)
	}
	if finding.FixProof.SourceSHA256 != hashSource(doc.Text) || len(finding.FixProof.SourceEdits) == 0 {
		t.Fatalf("proof omits source identity or edit ranges: %#v", finding.FixProof)
	}
	if len(finding.FixProof.Targets) != 2 || finding.FixProof.Targets[0].Renderer != "libass" ||
		finding.FixProof.Targets[1].Renderer != "xy-VSFilter" {
		t.Fatalf("default proof scope = %#v", finding.FixProof.Targets)
	}
	for _, target := range finding.FixProof.Targets {
		if target.Version == "" {
			t.Fatalf("target has no pinned version: %#v", target)
		}
	}
	if len(finding.FixProof.Interpretations) != 2 ||
		len(finding.FixProof.Interpretations[0].Dialogues) == 0 ||
		len(finding.FixProof.Interpretations[0].Dialogues[0].Before) == 0 ||
		len(finding.FixProof.Interpretations[0].Dialogues[0].After) == 0 {
		t.Fatalf("proof omits renderer interpretations, state, or provenance: %#v", finding.FixProof.Interpretations)
	}
	fixed, count, err := ApplyFixes(doc.Text, diagnostics, false)
	if err != nil || count != 1 || !strings.Contains(fixed, `\fs21}A`) {
		t.Fatalf("proved fix = (%q, %d, %v)", fixed, count, err)
	}
}

func TestEmptyOverrideSafeFixRequiresResolvedOperations(t *testing.T) {
	for _, test := range []struct {
		name     string
		text     string
		wantSafe bool
	}{
		{name: "resolved", text: `{}A`, wantSafe: true},
		{name: "unknown command", text: `{}{\mystery}A`},
	} {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := AnalyzeDocument(parseFontOverrideText(test.text))
			for _, diagnostic := range diagnostics {
				if diagnostic.ID != IssueEmptyOverrideBlock {
					continue
				}
				if test.wantSafe {
					if diagnostic.FixSafety != SafeFix || diagnostic.FixProof == nil {
						t.Fatalf("resolved empty block has no proof: %#v", diagnostic)
					}
				} else if diagnostic.FixSafety == SafeFix || diagnostic.FixProof != nil || len(diagnostic.Edits) != 0 {
					t.Fatalf("unresolved command did not revoke the empty-block fix: %#v", diagnostic)
				}
				return
			}
			t.Fatalf("no ASS018 diagnostic for %q", test.text)
		})
	}
}

func TestGroupedProofSerializesOnceWithReferences(t *testing.T) {
	doc := parseFontOverrideText(`{\fs21\fs21}A{\fs22\fs22}B`)
	diagnostics := AnalyzeDocument(doc)
	type proofSummary struct {
		FixProof    json.RawMessage `json:"fix_proof"`
		FixProofRef string          `json:"fix_proof_ref"`
	}
	data, err := json.Marshal(diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	var summaries []proofSummary
	if err := json.Unmarshal(data, &summaries); err != nil {
		t.Fatal(err)
	}
	var inline, references int
	var proofID string
	for _, summary := range summaries {
		if len(summary.FixProof) != 0 {
			inline++
			var proof struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(summary.FixProof, &proof); err != nil {
				t.Fatal(err)
			}
			proofID = proof.ID
		}
		if summary.FixProofRef != "" {
			references++
			if proofID != "" && summary.FixProofRef != proofID {
				t.Fatalf("proof reference %q does not match inline ID %q", summary.FixProofRef, proofID)
			}
		}
	}
	if inline != 1 || references != 1 || proofID == "" {
		t.Fatalf("group proof JSON has %d inline proofs and %d references: %s", inline, references, data)
	}
}

func TestSafeFixCannotUseAModOnlyPrefixProofInDefaultMode(t *testing.T) {
	source := `{\fsvp6}A`
	mod, err := renderer.New(renderer.VSFilterMod, renderer.Build{
		Mod: renderer.FeatureEnabled, Lua: renderer.FeatureDisabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = buildFixProof(source, []FixEditProof{{
		ID: IssueNoEffect, Edit: TextEdit{Start: 1, End: 7},
	}}, defaultFixTargets())
	if err == nil {
		t.Fatal("default libass/xy proof accepted removal of the Mod prefix collision")
	}
	if mod.Version() == "" {
		t.Fatal("VSFilterMod test profile has no pinned version")
	}
	defaultTargets := fixTargets(defaultFixTargets())
	if len(defaultTargets) != 2 || defaultTargets[0].Renderer == mod.Kind().String() || defaultTargets[1].Renderer == mod.Kind().String() {
		t.Fatalf("default proof scope includes VSFilterMod: %#v", defaultTargets)
	}
}

func TestDefaultFixApplicationRejectsModOnlyProof(t *testing.T) {
	source := `{\fs20\fs20}A`
	mod, err := renderer.New(renderer.VSFilterMod, renderer.Build{
		Mod: renderer.FeatureEnabled, Lua: renderer.FeatureDisabled,
	})
	if err != nil {
		t.Fatal(err)
	}
	edit := TextEdit{Start: 6, End: 11}
	diagnostic := Diagnostic{
		ID: IssueNoEffect, FixSafety: SafeFix, Edits: []TextEdit{edit},
		FixProof: &FixProof{
			SourceSHA256: hashSource(source),
			SourceEdits:  []FixEditProof{{ID: IssueNoEffect, Edit: edit}},
			Targets:      fixTargets([]renderer.Profile{mod}),
		},
	}
	if _, _, err := ApplyFixes(source, []Diagnostic{diagnostic}, false); err == nil {
		t.Fatal("default ApplyFixes accepted a VSFilterMod-only proof")
	}
}

func TestSafeFixRejectsStaleSourceAndMissingProof(t *testing.T) {
	doc := parseFontOverrideText(`{\fs21\fs21}A`)
	diagnostics := AnalyzeDocument(doc)
	if _, _, err := ApplyFixes(doc.Text+"B", diagnostics, false); err == nil {
		t.Fatal("SafeFix applied after its source changed")
	}
	unproved := []Diagnostic{{
		ID: IssueNoEffect, FixSafety: SafeFix, Edits: []TextEdit{{Start: 1, End: 2}},
	}}
	fixed, count, err := ApplyFixes("abc", unproved, true)
	if err != nil || count != 0 || fixed != "abc" {
		t.Fatalf("unproved fix changed the source: (%q, %d, %v)", fixed, count, err)
	}
	if safe, _, unfixable := CountFixes(unproved); safe != 0 || unfixable != 1 {
		t.Fatalf("unproved fix counts = safe %d, unfixable %d", safe, unfixable)
	}
}

func TestSafeFixProofPreservesFirstWinsOwnership(t *testing.T) {
	doc := parseFontOverrideText(`{\an7\an8}A`)
	diagnostics := AnalyzeDocument(doc)
	var finding *Diagnostic
	for i := range diagnostics {
		if diagnostics[i].ID == IssueNoEffect && diagnostics[i].Tag == "an" {
			finding = &diagnostics[i]
			break
		}
	}
	if finding == nil || finding.FixSafety != SafeFix || finding.FixProof == nil {
		t.Fatalf("second first-wins tag has no all-target proof: %#v", diagnostics)
	}
	fixed, count, err := ApplyFixes(doc.Text, diagnostics, false)
	if err != nil || count != 1 || !strings.Contains(fixed, `\an7}A`) {
		t.Fatalf("first-wins fixed result = (%q, %d, %v)", fixed, count, err)
	}
}
