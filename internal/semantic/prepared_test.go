package semantic

import (
	"context"
	"reflect"
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

func allStandardProfiles(t *testing.T) []renderer.Profile {
	t.Helper()
	var profiles []renderer.Profile
	for _, kind := range []renderer.Kind{renderer.Libass, renderer.XYVSFilter, renderer.VSFilterMod} {
		standard, err := renderer.Standard(kind)
		if err != nil {
			t.Fatal(err)
		}
		profiles = append(profiles, standard)
	}
	mod, err := renderer.New(renderer.VSFilterMod, renderer.Build{Mod: renderer.FeatureEnabled, Lua: renderer.FeatureDisabled})
	if err != nil {
		t.Fatal(err)
	}
	return append(profiles, mod)
}

func stylesFixture() map[string]StyleState {
	doc := ass.Parse("[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n" +
		"Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1\n")
	return StyleStatesByName(doc.StyleFields)
}

func TestPreparedEvaluationMatchesCurrentPath(t *testing.T) {
	texts := []string{
		`{\t(0,500,\fscx100\fy45)}nested`,
		`{\t(0,500,\t(0,250,\b1))}deep`,
		`{\t(0,500,\b}unclosed`,
		`{\fn}emptyarg{\bad\x}unknown`,
		`{\r}{\pos(1,2}{\clip(0,0,1,1)}reset`,
		`{\1c&HFF&}{\an9}{\be1}mixed`,
		`{\b1\b1\fs20\fs20\t(0,500,\fscx100)}scaling shape`,
	}
	profiles := allStandardProfiles(t)
	styles := stylesFixture()
	for _, text := range texts {
		tree := ass.ParseConcreteDialogue(text)
		for _, profile := range profiles {
			prepared, err := PrepareDialogue(context.Background(), tree, profile)
			if err != nil {
				t.Fatalf("%q profile %v: %v", text, profile.Kind(), err)
			}
			for _, options := range []EvaluationOptions{
				{},
				{Styles: styles, DialogueStyle: "Default"},
				{SkipNoEffectProofs: true},
			} {
				current := EvaluateResolved(tree, profile, options)
				built, err := EvaluatePrepared(context.Background(), prepared, options)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(current, built) {
					t.Fatalf("prepared evaluation differs for %q profile %v options %+v", text, profile.Kind(), options)
				}
			}
		}
	}
}

func TestPreparedOperationsFeedWalkConsumers(t *testing.T) {
	text := `{\t(0,500,\b1\fscx100)}{\pos(1,2}{\an8}body`
	profiles := allStandardProfiles(t)
	for _, profile := range profiles {
		tree := ass.ParseConcreteDialogue(text)
		prepared, err := PrepareDialogue(context.Background(), tree, profile)
		if err != nil {
			t.Fatal(err)
		}
		var walked []renderer.Result
		var walkNested []bool
		profile.WalkDialogue(tree, func(result renderer.Result, nested bool) bool {
			walked = append(walked, result)
			walkNested = append(walkNested, nested)
			return true
		})
		var replayed []renderer.Result
		var replayNested []bool
		prepared.EachOperation(func(result renderer.Result, nested bool) bool {
			replayed = append(replayed, result)
			replayNested = append(replayNested, nested)
			return true
		})
		if len(walked) == 0 || !reflect.DeepEqual(walked, replayed) || !reflect.DeepEqual(walkNested, replayNested) {
			t.Fatalf("prepared walk replay differs for profile %v: %d vs %d results, %v vs %v nested",
				profile.Kind(), len(walked), len(replayed), walkNested, replayNested)
		}
	}
}

func TestPreparedDialogueCancellation(t *testing.T) {
	tree := ass.ParseConcreteDialogue(`{\b1}x`)
	profile, _ := renderer.Standard(renderer.Libass)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if prepared, err := PrepareDialogue(canceled, tree, profile); err == nil || prepared != nil {
		t.Fatal("canceled prepare succeeded")
	}
	prepared, err := PrepareDialogue(context.Background(), tree, profile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EvaluatePrepared(canceled, prepared, EvaluationOptions{}); err == nil {
		t.Fatal("canceled evaluation succeeded")
	}
	if _, err := EvaluatePrepared(context.Background(), prepared, EvaluationOptions{}); err != nil {
		t.Fatalf("evaluation after cancellation failed: %v", err)
	}
}

func TestDocumentViewFromConcreteMatchesDocumentView(t *testing.T) {
	documents := []string{
		// Reordered Style and Event field layouts.
		"[Script Info]\nPlayResX: 640\nPlayResY: 480\n[V4+ Styles]\nFormat: Fontsize, Name, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding, Fontname\nStyle: 20,Default,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1,Arial\n[Events]\nFormat: End, Layer, Start, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0:00:02.00,0,0:00:01.00,Default,,0,0,0,,{\\b1\\b1}swapped\n",
		// Unknown record dialect: extra fields beyond the VSFilter groups.
		"[Script Info]\nPlayResX: 640\nPlayResY: 480\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\nStyle: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1,extra\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text, Extra\nDialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,{\\pos(1)}dia,more\n",
		// Missing cells.
		"[Script Info]\nPlayResX: 640\nPlayResY: 480\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0\n",
	}
	profiles := allStandardProfiles(t)
	for _, text := range documents {
		doc := ass.Parse(text)
		tree := ass.ParseConcreteDocument(doc.Text)
		for _, profile := range profiles {
			layouts := profile.ResolveDocumentLayouts(tree)
			built, err := profile.DocumentViewFromConcrete(context.Background(), doc, tree, layouts)
			if err != nil {
				t.Fatalf("profile %v: %v", profile.Kind(), err)
			}
			if want := profile.DocumentView(doc); !reflect.DeepEqual(want, built) {
				t.Fatalf("DocumentViewFromConcrete differs for profile %v on document %q", profile.Kind(), text)
			}
		}
	}
}

func TestDocumentViewFromConcreteCancellation(t *testing.T) {
	doc := ass.Parse("[Script Info]\nPlayResX: 640\nPlayResY: 480\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,x\n")
	tree := ass.ParseConcreteDocument(doc.Text)
	xy, _ := renderer.Standard(renderer.XYVSFilter)
	layouts := xy.ResolveDocumentLayouts(tree)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := xy.DocumentViewFromConcrete(canceled, doc, tree, layouts); err == nil {
		t.Fatal("canceled projection succeeded")
	}
	if _, err := xy.DocumentViewFromConcrete(context.Background(), doc, tree, layouts); err != nil {
		t.Fatalf("later projection failed: %v", err)
	}
}

func TestCompareDialogueInModeLintOnlyDropsEquivalents(t *testing.T) {
	tree := ass.ParseConcreteDialogue(`{\pos(1,2)\fs20\t(0,500,\fscx100)}A`)
	source := CompareDialogue(tree, nil, EvaluationOptions{})
	lintOnly := CompareDialogueInMode(tree, nil, EvaluationOptions{}, ComparisonMode{LintOnly: true})
	for _, finding := range source {
		if finding.Status != CompatibilityEquivalent {
			continue
		}
		for _, kept := range lintOnly {
			if kept.Source == finding.Source && kept.Dimension == finding.Dimension {
				t.Fatalf("lint-only mode kept equivalent observation %s at %v", kept.Dimension, kept.Source)
			}
		}
	}
	var kept []CompatibilityFinding
	for _, finding := range source {
		if finding.Status != CompatibilityEquivalent {
			kept = append(kept, finding)
		}
	}
	if len(lintOnly) != len(kept) {
		t.Fatalf("lint-only emitted %d findings, want %d", len(lintOnly), len(kept))
	}
	for i := range lintOnly {
		// Detail strings, citations, and order must match the full pass.
		if lintOnly[i].Detail != kept[i].Detail || lintOnly[i].Dimension != kept[i].Dimension ||
			lintOnly[i].Status != kept[i].Status || lintOnly[i].Source != kept[i].Source ||
			!reflect.DeepEqual(lintOnly[i].Citations, kept[i].Citations) {
			t.Fatalf("lint-only finding %d differs: %#v vs %#v", i, lintOnly[i], kept[i])
		}
	}
}

func TestDialoguePreparationsReuseOneResolution(t *testing.T) {
	text := `{\t(0,500,\b1\fscx100)}{\pos(1,2}A`
	profile, _ := renderer.Standard(renderer.XYVSFilter)
	preparations := NewDialoguePreparations()
	first := preparations.PrepareText(profile, text)
	second := preparations.PrepareText(profile, text)
	if first != second {
		t.Fatal("equal text under one profile resolved twice")
	}
	other, _ := renderer.Standard(renderer.Libass)
	if preparations.PrepareText(other, text) == first {
		t.Fatal("prepared value crossed profiles")
	}
	// Reuse must reproduce the same evaluation as a fresh path.
	options := EvaluationOptions{SkipNoEffectProofs: false}
	current := EvaluateResolved(ass.ParseConcreteDialogue(text), profile, options)
	built, err := EvaluatePrepared(context.Background(), second, options)
	if err != nil || !reflect.DeepEqual(current, built) {
		t.Fatalf("cached evaluation differs: err %v", err)
	}
}

func TestCompareDocumentInModeMatchesFullMinusEquivalents(t *testing.T) {
	source := "[Script Info]\rScriptType: v4.00+\r[V4+ Styles]\rFormat: Name, Fontname, Fontsize\rStyle: Default,Arial,20\r[Events]\rFormat: Layer, Start, End, Text\rDialogue: 0,0:00:01.00,0:00:02.00,{\\fs20\\an5\\pos(1,2)}A\r"
	tree := ass.ParseConcreteDocument(source)
	xy, _ := renderer.Standard(renderer.XYVSFilter)
	vsm, _ := renderer.New(renderer.VSFilterMod, renderer.Build{Mod: renderer.FeatureEnabled, Lua: renderer.FeatureDisabled})
	profiles := []renderer.Profile{xy, vsm}
	full := CompareDocument(tree, profiles)
	mode := ComparisonMode{LintOnly: true}
	lintOnly := CompareDocumentInMode(tree, profiles, mode)
	var wanted []CompatibilityFinding
	for _, finding := range full {
		if finding.Status != CompatibilityEquivalent {
			wanted = append(wanted, finding)
		}
	}
	// Stable sort keeps non-equivalent findings in the same relative order.
	if len(lintOnly) != len(wanted) {
		t.Fatalf("lint-only document pass emitted %d findings, want %d", len(lintOnly), len(wanted))
	}
	for i := range lintOnly {
		if lintOnly[i].Source != wanted[i].Source || lintOnly[i].Dimension != wanted[i].Dimension ||
			lintOnly[i].Status != wanted[i].Status || lintOnly[i].Detail != wanted[i].Detail {
			t.Fatalf("lint-only finding %d differs", i)
		}
	}
	// Supplied layouts must not change any finding.
	prepared := CompareDocumentInMode(tree, profiles, ComparisonMode{
		Layouts: map[renderer.Profile][]renderer.RecordLayout{
			xy:  xy.ResolveDocumentLayouts(tree),
			vsm: vsm.ResolveDocumentLayouts(tree),
		},
	})
	if len(prepared) != len(full) {
		t.Fatalf("layout-supplied pass emitted %d findings, want %d", len(prepared), len(full))
	}
	for i := range prepared {
		if prepared[i].Source != full[i].Source || prepared[i].Detail != full[i].Detail ||
			prepared[i].Status != full[i].Status || prepared[i].Dimension != full[i].Dimension {
			t.Fatalf("layout-supplied finding %d differs", i)
		}
	}
}
